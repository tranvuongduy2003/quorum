package ingestion

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	domainingestion "quorum/internal/domain/ingestion"
)

type streamResult struct {
	record domainingestion.SourceRecord
	err    error
}

type fakeRecordStream struct {
	results []streamResult
	index   int
	closed  bool
}

func (s *fakeRecordStream) Next(context.Context) (domainingestion.SourceRecord, error) {
	result := s.results[s.index]
	s.index++
	return result.record, result.err
}

func (s *fakeRecordStream) Close() error {
	s.closed = true
	return nil
}

type fakeArchive struct {
	streams  map[domainingestion.Table]*fakeRecordStream
	calls    []string
	closed   bool
	identity domainingestion.ArchiveIdentity
}

func (a *fakeArchive) ValidateTables([]domainingestion.Table) error {
	a.calls = append(a.calls, "validate")
	return nil
}

func (a *fakeArchive) OpenTable(_ context.Context, table domainingestion.Table, _ int) (RecordStream, error) {
	a.calls = append(a.calls, "open:"+table.String())
	return a.streams[table], nil
}

func (a *fakeArchive) Close() error {
	a.closed = true
	return nil
}

func (a *fakeArchive) Identity() domainingestion.ArchiveIdentity {
	return a.identity
}

type fakeArchiveFactory struct {
	archive *fakeArchive
	opened  bool
}

type writerResult struct {
	count int64
	err   error
}

type fakeWriter struct {
	accepted         [][]domainingestion.SourceRecord
	quarantine       [][]domainingestion.QuarantineRecord
	acceptedResults  []writerResult
	quarantineResult []writerResult
}

type checkpointCleanCall struct {
	site   domainingestion.Site
	table  domainingestion.Table
	offset int64
}

type fakeCheckpointStore struct {
	checkpoint domainingestion.Checkpoint
	found      bool
	loadErr    error
	saveErr    error
	cleanErr   error
	loads      int
	saves      []domainingestion.Checkpoint
	cleans     []checkpointCleanCall
}

type fakeCheckpointReporter struct {
	checkpoints []domainingestion.Checkpoint
}

func (r *fakeCheckpointReporter) CheckpointSaved(checkpoint domainingestion.Checkpoint) {
	r.checkpoints = append(r.checkpoints, checkpoint)
}

func (s *fakeCheckpointStore) Load(context.Context, domainingestion.Site, domainingestion.Table) (domainingestion.Checkpoint, bool, error) {
	s.loads++
	return s.checkpoint, s.found, s.loadErr
}

func (s *fakeCheckpointStore) Save(_ context.Context, checkpoint domainingestion.Checkpoint) error {
	s.saves = append(s.saves, checkpoint)
	return s.saveErr
}

func (s *fakeCheckpointStore) CleanAfterCheckpoint(_ context.Context, site domainingestion.Site, table domainingestion.Table, offset int64) error {
	s.cleans = append(s.cleans, checkpointCleanCall{site: site, table: table, offset: offset})
	return s.cleanErr
}

func (w *fakeWriter) WriteAccepted(_ context.Context, _ domainingestion.Site, _ domainingestion.Table, records []domainingestion.SourceRecord) (int64, error) {
	w.accepted = append(w.accepted, append([]domainingestion.SourceRecord(nil), records...))
	index := len(w.accepted) - 1
	if index < len(w.acceptedResults) {
		return w.acceptedResults[index].count, w.acceptedResults[index].err
	}
	return int64(len(records)), nil
}

func (w *fakeWriter) WriteQuarantine(_ context.Context, records []domainingestion.QuarantineRecord) (int64, error) {
	w.quarantine = append(w.quarantine, append([]domainingestion.QuarantineRecord(nil), records...))
	index := len(w.quarantine) - 1
	if index < len(w.quarantineResult) {
		return w.quarantineResult[index].count, w.quarantineResult[index].err
	}
	return int64(len(records)), nil
}

func (f *fakeArchiveFactory) Open(string) (Archive, error) {
	f.opened = true
	return f.archive, nil
}

func TestServiceRunProcessesTablesSequentiallyAndClosesResources(t *testing.T) {
	posts := &fakeRecordStream{results: []streamResult{{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 8, "<row />", nil)}, {err: io.EOF}}}
	votes := &fakeRecordStream{results: []streamResult{{record: domainingestion.NewSourceRecord(domainingestion.TableVotes, 3, "<row />", nil)}, {err: io.EOF}}}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TablePosts: posts, domainingestion.TableVotes: votes}}
	factory := &fakeArchiveFactory{archive: archive}
	service := NewService(factory, nil, nil, nil)
	command := testCommand(t, []domainingestion.Table{domainingestion.TablePosts, domainingestion.TableVotes})

	summary, err := service.Run(context.Background(), command)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !factory.opened || !archive.closed || !posts.closed || !votes.closed {
		t.Fatalf("resource ownership = factory:%t archive:%t posts:%t votes:%t", factory.opened, archive.closed, posts.closed, votes.closed)
	}
	if got, want := len(summary.Tables), 2; got != want {
		t.Fatalf("table summaries = %d, want %d", got, want)
	}
	for _, table := range summary.Tables {
		if table.Processed != 1 || table.Valid != 1 || table.Malformed != 0 {
			t.Fatalf("summary for %s = %#v", table.Table, table)
		}
	}
}

func TestServiceRunReturnsPartialMalformedSummary(t *testing.T) {
	malformed := SourceError{Table: domainingestion.TablePosts, Offset: 21, Err: domainingestion.ErrMalformedRecord}
	posts := &fakeRecordStream{results: []streamResult{{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 5, "<row />", nil)}, {err: malformed}}}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TablePosts: posts}}
	service := NewService(&fakeArchiveFactory{archive: archive}, nil, nil, nil)

	summary, err := service.Run(context.Background(), testCommand(t, []domainingestion.Table{domainingestion.TablePosts}))
	if summary.Status != RunStatusFailed || len(summary.Tables) != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	table := summary.Tables[0]
	if table.Processed != 2 || table.Valid != 1 || table.Malformed != 1 || table.LastOffset != 21 {
		t.Fatalf("partial table summary = %#v", table)
	}
	var runErr RunError
	if !errors.As(err, &runErr) || runErr.Table != "posts" || runErr.Offset != 21 || !errors.Is(runErr, domainingestion.ErrMalformedRecord) {
		t.Fatalf("Run() error = %#v", err)
	}
}

func TestServiceRunReportsObservedPercentageInPartialSummary(t *testing.T) {
	watermark, err := domainingestion.NewWatermarkPolicy([]string{"synthetic-marker"})
	if err != nil {
		t.Fatalf("NewWatermarkPolicy() error = %v", err)
	}
	malformed := SourceError{Table: domainingestion.TablePosts, Offset: 21, Err: domainingestion.ErrMalformedRecord}
	posts := &fakeRecordStream{results: []streamResult{
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 5, "synthetic-marker", nil)},
		{err: malformed},
	}}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TablePosts: posts}}
	service := NewService(&fakeArchiveFactory{archive: archive}, nil, nil, nil, watermark)

	summary, runErr := service.Run(context.Background(), testCommandWithThreshold(t, []domainingestion.Table{domainingestion.TablePosts}, 100))

	if runErr == nil {
		t.Fatal("Run() error = nil")
	}
	if summary.ObservedPercent != 50 {
		t.Fatalf("ObservedPercent = %v, want 50", summary.ObservedPercent)
	}
}

func TestServiceRunReturnsCompleteSummaryBeforeThresholdFailure(t *testing.T) {
	watermark, err := domainingestion.NewWatermarkPolicy([]string{"synthetic-marker"})
	if err != nil {
		t.Fatalf("NewWatermarkPolicy() error = %v", err)
	}
	posts := &fakeRecordStream{results: []streamResult{
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 8, "synthetic-marker", nil)},
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 16, "clean", nil)},
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 24, "clean", nil)},
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 32, "clean", nil)},
		{err: io.EOF},
	}}
	votes := &fakeRecordStream{results: []streamResult{
		{record: domainingestion.NewSourceRecord(domainingestion.TableVotes, 3, "clean", nil)},
		{record: domainingestion.NewSourceRecord(domainingestion.TableVotes, 6, "synthetic-marker", nil)},
		{err: io.EOF},
	}}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{
		domainingestion.TablePosts: posts,
		domainingestion.TableVotes: votes,
	}}
	service := NewService(&fakeArchiveFactory{archive: archive}, nil, nil, nil, watermark)
	command := testCommandWithThreshold(t, []domainingestion.Table{domainingestion.TablePosts, domainingestion.TableVotes}, 10)

	summary, err := service.Run(context.Background(), command)

	if summary.Status != RunStatusFailed || len(summary.Tables) != 2 || summary.ObservedPercent != 50 {
		t.Fatalf("summary = %#v", summary)
	}
	if posts.index != len(posts.results) || votes.index != len(votes.results) {
		t.Fatalf("stream positions = posts:%d/%d votes:%d/%d", posts.index, len(posts.results), votes.index, len(votes.results))
	}
	var runErr RunError
	if !errors.As(err, &runErr) || runErr.Table != "posts" || runErr.Offset != 32 {
		t.Fatalf("Run() error = %#v", err)
	}
	if !errors.Is(err, ErrRejectThresholdExceeded) {
		t.Fatalf("errors.Is(%v, ErrRejectThresholdExceeded) = false", err)
	}
	if got, want := err.Error(), "error table=posts offset=32 cause=rejected record percentage 25.0000 exceeds configured threshold 10.0000"; got != want {
		t.Fatalf("Run() error = %q, want %q", got, want)
	}
}

func TestServiceRunAllowsRejectionRateEqualToThreshold(t *testing.T) {
	watermark, err := domainingestion.NewWatermarkPolicy([]string{"synthetic-marker"})
	if err != nil {
		t.Fatalf("NewWatermarkPolicy() error = %v", err)
	}
	results := make([]streamResult, 0, 201)
	results = append(results, streamResult{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 1, "synthetic-marker", nil)})
	for offset := int64(2); offset <= 200; offset++ {
		results = append(results, streamResult{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, offset, "clean", nil)})
	}
	results = append(results, streamResult{err: io.EOF})
	posts := &fakeRecordStream{results: results}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TablePosts: posts}}
	service := NewService(&fakeArchiveFactory{archive: archive}, nil, nil, nil, watermark)

	summary, err := service.Run(context.Background(), testCommandWithThreshold(t, []domainingestion.Table{domainingestion.TablePosts}, 0.5))

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Status != RunStatusOK || summary.ObservedPercent != 0.5 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestServiceRunClassifiesRecordsWithFirstMatchingPolicy(t *testing.T) {
	watermark, err := domainingestion.NewWatermarkPolicy([]string{"synthetic-marker"})
	if err != nil {
		t.Fatalf("NewWatermarkPolicy() error = %v", err)
	}
	posts := &fakeRecordStream{results: []streamResult{
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 8, "<row Body=\"synthetic-marker\" />", map[string]string{"CreationDate": "not-a-date"})},
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 16, "<row Body=\"synthetic-marker\" />", nil)},
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 24, "<row />", nil)},
		{err: io.EOF},
	}}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TablePosts: posts}}
	service := NewService(&fakeArchiveFactory{archive: archive}, nil, nil, nil, domainingestion.NewTimestampPolicy(), watermark)

	summary, err := service.Run(context.Background(), testCommandWithThreshold(t, []domainingestion.Table{domainingestion.TablePosts}, 100))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	table := summary.Tables[0]
	if table.Processed != 3 || table.Valid != 1 || table.Rejected != 2 || table.Malformed != 0 || table.LastOffset != 24 {
		t.Fatalf("table summary = %#v", table)
	}
	if table.Processed != table.Valid+table.Rejected {
		t.Fatalf("processed = %d, valid + rejected = %d", table.Processed, table.Valid+table.Rejected)
	}
	if got := table.Rejections[domainingestion.ReasonInvalidTimestamp]; got != 1 {
		t.Fatalf("invalid timestamp rejections = %d, want 1", got)
	}
	if got := table.Rejections[domainingestion.ReasonWatermarkPattern]; got != 1 {
		t.Fatalf("watermark rejections = %d, want 1", got)
	}
}

func TestServiceRunDoesNotOpenArchiveAfterCancellation(t *testing.T) {
	factory := &fakeArchiveFactory{archive: &fakeArchive{}}
	service := NewService(factory, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.Run(ctx, testCommand(t, []domainingestion.Table{domainingestion.TablePosts}))
	if factory.opened {
		t.Fatal("Run() opened an archive after cancellation")
	}
	var runErr RunError
	if !errors.As(err, &runErr) || runErr.Table != "posts" || runErr.Offset != 0 || !errors.Is(runErr, context.Canceled) {
		t.Fatalf("Run() error = %#v", err)
	}
}

func TestServiceRunRequiresWriterBeforeOpeningArchive(t *testing.T) {
	factory := &fakeArchiveFactory{archive: &fakeArchive{}}
	service := NewService(factory, nil, nil, nil)
	command := testCommandWithModeAndThreshold(t, []domainingestion.Table{domainingestion.TablePosts}, false, 100)

	summary, err := service.Run(context.Background(), command)

	if factory.opened {
		t.Fatal("Run() opened an archive without a writer")
	}
	if summary.Status != RunStatusFailed {
		t.Fatalf("summary status = %q, want %q", summary.Status, RunStatusFailed)
	}
	var runErr RunError
	if !errors.As(err, &runErr) || runErr.Table != "request" || runErr.Offset != 0 || !errors.Is(err, ErrWriterRequired) {
		t.Fatalf("Run() error = %#v", err)
	}
}

func TestServiceRunRoutesAcceptedAndRejectedRecordsAndConfirmsSourceCount(t *testing.T) {
	watermark, err := domainingestion.NewWatermarkPolicy([]string{"synthetic-marker"})
	if err != nil {
		t.Fatalf("NewWatermarkPolicy() error = %v", err)
	}
	rejected := domainingestion.NewSourceRecord(domainingestion.TablePosts, 8, "synthetic-marker", map[string]string{"Id": "1"})
	posts := &fakeRecordStream{results: []streamResult{
		{record: rejected},
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 16, "clean", nil)},
		{err: io.EOF},
	}}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TablePosts: posts}}
	writer := &fakeWriter{}
	foundAt := time.Date(2026, time.September, 6, 10, 30, 0, 0, time.FixedZone("ICT", 7*60*60))
	service := NewService(&fakeArchiveFactory{archive: archive}, writer, nil, func() time.Time { return foundAt }, watermark)
	command := testCommandWithModeAndThreshold(t, []domainingestion.Table{domainingestion.TablePosts}, false, 100)

	summary, err := service.Run(context.Background(), command)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Tables[0].Rejected != 1 || summary.Tables[0].Valid != 1 || summary.Tables[0].Confirmed != 2 {
		t.Fatalf("table summary = %#v", summary.Tables[0])
	}
	if len(writer.accepted) != 1 || len(writer.accepted[0]) != 1 || writer.accepted[0][0].Offset != 16 {
		t.Fatalf("accepted batches = %#v", writer.accepted)
	}
	if len(writer.quarantine) != 1 || len(writer.quarantine[0]) != 1 {
		t.Fatalf("quarantine batches = %#v", writer.quarantine)
	}
	record := writer.quarantine[0][0]
	if record.Site != command.Site || record.Source.Offset != rejected.Offset || record.Source.Raw != rejected.Raw || record.Reason != domainingestion.ReasonWatermarkPattern {
		t.Fatalf("saved record = %#v", record)
	}
	if got, want := record.FoundAt, foundAt.UTC(); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("found at = %v, want %v in UTC", got, want)
	}
}

func TestServiceRunPreservesAcceptedConfirmationWhenQuarantineFails(t *testing.T) {
	watermark, err := domainingestion.NewWatermarkPolicy([]string{"synthetic-marker"})
	if err != nil {
		t.Fatalf("NewWatermarkPolicy() error = %v", err)
	}
	posts := &fakeRecordStream{results: []streamResult{
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 19, "synthetic-marker", nil)},
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 28, "clean", nil)},
		{err: io.EOF},
	}}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TablePosts: posts}}
	saveErr := errors.New("save failed")
	writer := &fakeWriter{quarantineResult: []writerResult{{err: saveErr}}}
	service := NewService(&fakeArchiveFactory{archive: archive}, writer, nil, nil, watermark)
	command := testCommandWithModeAndThreshold(t, []domainingestion.Table{domainingestion.TablePosts}, false, 100)

	summary, err := service.Run(context.Background(), command)

	if summary.Status != RunStatusFailed || len(summary.Tables) != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	table := summary.Tables[0]
	if table.Processed != 2 || table.Rejected != 1 || table.Valid != 1 || table.Confirmed != 1 || table.LastOffset != 28 {
		t.Fatalf("partial table summary = %#v", table)
	}
	if posts.index != len(posts.results) || len(writer.accepted) != 1 || len(writer.quarantine) != 1 {
		t.Fatalf("write boundary = stream:%d/%d accepted:%d quarantine:%d", posts.index, len(posts.results), len(writer.accepted), len(writer.quarantine))
	}
	var runErr RunError
	if !errors.As(err, &runErr) || runErr.Table != "posts" || runErr.Offset != 19 || !errors.Is(err, saveErr) {
		t.Fatalf("Run() error = %#v", err)
	}
}

func TestServiceRunUsesWriterFailureOffsetAndPreservesCause(t *testing.T) {
	stream := &fakeRecordStream{results: []streamResult{
		{record: domainingestion.NewSourceRecord(domainingestion.TableVotes, 47, "first", nil)},
		{record: domainingestion.NewSourceRecord(domainingestion.TableVotes, 133, "second", nil)},
		{err: io.EOF},
	}}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TableVotes: stream}}
	cause := errors.New("rejected row")
	writer := &fakeWriter{acceptedResults: []writerResult{{err: WriteFailure{Offset: 47, Err: cause}}}}
	service := NewService(&fakeArchiveFactory{archive: archive}, writer, nil, nil)

	summary, err := service.Run(context.Background(), testCommandWithModeAndThreshold(t, []domainingestion.Table{domainingestion.TableVotes}, false, 100))

	if summary.Status != RunStatusFailed || len(summary.Tables) != 1 || summary.Tables[0].Confirmed != 0 {
		t.Fatalf("summary = %#v", summary)
	}
	var runErr RunError
	if !errors.As(err, &runErr) || runErr.Table != "votes" || runErr.Offset != 47 || !errors.Is(err, cause) {
		t.Fatalf("Run() error = %#v", err)
	}
	var failure WriteFailure
	if errors.As(err, &failure) {
		t.Fatalf("Run() leaked WriteFailure = %#v", failure)
	}
}

func TestServiceRunRejectsWriterCountMismatchWithoutConfirmation(t *testing.T) {
	stream := &fakeRecordStream{results: []streamResult{
		{record: domainingestion.NewSourceRecord(domainingestion.TableVotes, 41, "clean", nil)},
		{err: io.EOF},
	}}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TableVotes: stream}}
	writer := &fakeWriter{acceptedResults: []writerResult{{count: 0}}}
	service := NewService(&fakeArchiveFactory{archive: archive}, writer, nil, nil)

	summary, err := service.Run(context.Background(), testCommandWithModeAndThreshold(t, []domainingestion.Table{domainingestion.TableVotes}, false, 100))

	if len(summary.Tables) != 1 || summary.Tables[0].Confirmed != 0 || summary.Status != RunStatusFailed {
		t.Fatalf("summary = %#v", summary)
	}
	var runErr RunError
	if !errors.As(err, &runErr) || runErr.Table != "votes" || runErr.Offset != 41 || !errors.Is(err, ErrWriteCountMismatch) {
		t.Fatalf("Run() error = %#v", err)
	}
}

func TestServiceRunDryRunMakesNoWriterCalls(t *testing.T) {
	watermark, err := domainingestion.NewWatermarkPolicy([]string{"synthetic-marker"})
	if err != nil {
		t.Fatalf("NewWatermarkPolicy() error = %v", err)
	}
	posts := &fakeRecordStream{results: []streamResult{
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 8, "synthetic-marker", nil)},
		{err: io.EOF},
	}}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TablePosts: posts}}
	writer := &fakeWriter{}
	service := NewService(&fakeArchiveFactory{archive: archive}, writer, nil, nil, watermark)

	summary, err := service.Run(context.Background(), testCommandWithThreshold(t, []domainingestion.Table{domainingestion.TablePosts}, 100))

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Tables[0].Rejected != 1 || summary.Tables[0].Confirmed != 0 || len(writer.accepted) != 0 || len(writer.quarantine) != 0 {
		t.Fatalf("summary = %#v, accepted calls = %d, quarantine calls = %d", summary, len(writer.accepted), len(writer.quarantine))
	}
}

func TestServiceRunFlushesAtRowBoundary(t *testing.T) {
	results := make([]streamResult, 0, copyBatchRows+2)
	for offset := int64(1); offset <= copyBatchRows+1; offset++ {
		results = append(results, streamResult{record: domainingestion.NewSourceRecord(domainingestion.TableVotes, offset, "x", nil)})
	}
	results = append(results, streamResult{err: io.EOF})
	stream := &fakeRecordStream{results: results}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TableVotes: stream}}
	writer := &fakeWriter{}
	service := NewService(&fakeArchiveFactory{archive: archive}, writer, nil, nil)

	summary, err := service.Run(context.Background(), testCommandWithModeAndThreshold(t, []domainingestion.Table{domainingestion.TableVotes}, false, 100))

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(writer.accepted) != 2 || len(writer.accepted[0]) != copyBatchRows || len(writer.accepted[1]) != 1 {
		t.Fatalf("accepted batch sizes = %v", acceptedBatchSizes(writer.accepted))
	}
	if summary.Tables[0].Confirmed != copyBatchRows+1 {
		t.Fatalf("confirmed = %d, want %d", summary.Tables[0].Confirmed, copyBatchRows+1)
	}
}

func TestServiceRunFlushesSingleOversizedRecordOnce(t *testing.T) {
	record := domainingestion.NewSourceRecord(domainingestion.TableVotes, 1, strings.Repeat("x", int(copyBatchRawBytes)+1), nil)
	stream := &fakeRecordStream{results: []streamResult{{record: record}, {err: io.EOF}}}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TableVotes: stream}}
	writer := &fakeWriter{}
	service := NewService(&fakeArchiveFactory{archive: archive}, writer, nil, nil)

	summary, err := service.Run(context.Background(), testCommandWithModeAndThreshold(t, []domainingestion.Table{domainingestion.TableVotes}, false, 100))

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(writer.accepted) != 1 || len(writer.accepted[0]) != 1 || summary.Tables[0].Confirmed != 1 {
		t.Fatalf("writes = batches:%v summary:%#v", acceptedBatchSizes(writer.accepted), summary.Tables[0])
	}
}

func TestPendingWritesFlushesBeforeByteLimitOverflow(t *testing.T) {
	pending := pendingWrites{accepted: []domainingestion.SourceRecord{{}}, rawBytes: copyBatchRawBytes - 1}
	if pending.shouldFlush(domainingestion.NewSourceRecord(domainingestion.TableVotes, 1, "xx", nil)) != true {
		t.Fatal("shouldFlush() = false at byte overflow")
	}
	if pending.shouldFlush(domainingestion.NewSourceRecord(domainingestion.TableVotes, 1, "x", nil)) {
		t.Fatal("shouldFlush() = true at exact byte limit")
	}
}

func TestServiceRunSavesAtExactCheckpointIntervals(t *testing.T) {
	results := make([]streamResult, 0, 6)
	for offset := int64(1); offset <= 5; offset++ {
		results = append(results, streamResult{record: domainingestion.NewSourceRecord(domainingestion.TableVotes, offset, "x", nil)})
	}
	results = append(results, streamResult{err: io.EOF})
	archive := &fakeArchive{
		streams:  map[domainingestion.Table]*fakeRecordStream{domainingestion.TableVotes: {results: results}},
		identity: "archive",
	}
	writer := &fakeWriter{}
	checkpoints := &fakeCheckpointStore{}
	reporter := &fakeCheckpointReporter{}
	now := time.Date(2026, time.September, 21, 7, 0, 0, 0, time.UTC)
	service := NewService(&fakeArchiveFactory{archive: archive}, writer, checkpoints, func() time.Time { return now })
	service = service.WithCheckpointReporter(reporter)
	command := testCommandWithModeAndThreshold(t, []domainingestion.Table{domainingestion.TableVotes}, false, 100)
	command.CheckpointInterval = 2

	summary, err := service.Run(context.Background(), command)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := acceptedBatchSizes(writer.accepted); len(got) != 3 || got[0] != 2 || got[1] != 2 || got[2] != 1 {
		t.Fatalf("accepted batch sizes = %v", got)
	}
	if len(checkpoints.saves) != 2 {
		t.Fatalf("checkpoint saves = %#v", checkpoints.saves)
	}
	if len(reporter.checkpoints) != 2 || reporter.checkpoints[0] != checkpoints.saves[0] || reporter.checkpoints[1] != checkpoints.saves[1] {
		t.Fatalf("reported checkpoints = %#v", reporter.checkpoints)
	}
	for index, want := range []int64{2, 4} {
		checkpoint := checkpoints.saves[index]
		if checkpoint.ConfirmedCount != want || checkpoint.SourceOffset != want || checkpoint.ArchiveID != "archive" || checkpoint.UpdatedAt != now {
			t.Fatalf("checkpoint %d = %#v", index, checkpoint)
		}
	}
	if summary.Tables[0].Confirmed != 5 || checkpoints.loads != 1 || len(checkpoints.cleans) != 0 {
		t.Fatalf("summary = %#v, store = %#v", summary.Tables[0], checkpoints)
	}
}

func TestServiceRunResumesMatchingCheckpointAndCleansOrphans(t *testing.T) {
	stream := &fakeRecordStream{results: []streamResult{
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 1, "one", nil)},
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 2, "two", nil)},
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 3, "three", nil)},
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 4, "four", nil)},
		{err: io.EOF},
	}}
	archive := &fakeArchive{
		streams:  map[domainingestion.Table]*fakeRecordStream{domainingestion.TablePosts: stream},
		identity: "archive",
	}
	checkpoint := domainingestion.NewCheckpoint("stackoverflow.com", domainingestion.TablePosts, "archive", 2, 2, time.Now())
	checkpoints := &fakeCheckpointStore{checkpoint: checkpoint, found: true}
	writer := &fakeWriter{}
	service := NewService(&fakeArchiveFactory{archive: archive}, writer, checkpoints, nil)
	command := testCommandWithModeAndThreshold(t, []domainingestion.Table{domainingestion.TablePosts}, false, 100)

	summary, err := service.Run(context.Background(), command)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	table := summary.Tables[0]
	if !table.Resumed || table.ResumedOffset != 2 || table.Processed != 2 || table.Valid != 2 || table.Confirmed != 4 || table.LastOffset != 4 {
		t.Fatalf("summary = %#v", table)
	}
	if len(checkpoints.cleans) != 1 || checkpoints.cleans[0].site != command.Site || checkpoints.cleans[0].table != domainingestion.TablePosts || checkpoints.cleans[0].offset != 2 {
		t.Fatalf("clean calls = %#v", checkpoints.cleans)
	}
	if len(writer.accepted) != 1 || len(writer.accepted[0]) != 2 || writer.accepted[0][0].Offset != 3 || writer.accepted[0][1].Offset != 4 {
		t.Fatalf("accepted = %#v", writer.accepted)
	}
}

func TestServiceRunRejectsIncompatibleCheckpointBeforeCleanupOrStreamOpen(t *testing.T) {
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{}, identity: "current"}
	checkpoints := &fakeCheckpointStore{
		checkpoint: domainingestion.NewCheckpoint("stackoverflow.com", domainingestion.TablePosts, "stored", 8, 2, time.Now()),
		found:      true,
	}
	service := NewService(&fakeArchiveFactory{archive: archive}, &fakeWriter{}, checkpoints, nil)
	command := testCommandWithModeAndThreshold(t, []domainingestion.Table{domainingestion.TablePosts}, false, 100)

	_, err := service.Run(context.Background(), command)

	if !errors.Is(err, domainingestion.ErrIncompatibleArchive) || !strings.Contains(err.Error(), "stored=stored current=current") {
		t.Fatalf("Run() error = %v", err)
	}
	if len(checkpoints.cleans) != 0 || len(archive.calls) != 1 || archive.calls[0] != "validate" {
		t.Fatalf("store = %#v, archive calls = %v", checkpoints, archive.calls)
	}
}

func TestServiceRunStopsOnCheckpointOperations(t *testing.T) {
	loadCause := errors.New("load failed")
	cleanCause := errors.New("clean failed")
	saveCause := errors.New("save failed")
	tests := []struct {
		name        string
		store       *fakeCheckpointStore
		want        error
		wantOffset  int64
		wantSaves   int
		wantProcess int64
	}{
		{name: "load", store: &fakeCheckpointStore{loadErr: loadCause}, want: loadCause},
		{name: "clean", store: &fakeCheckpointStore{checkpoint: domainingestion.NewCheckpoint("stackoverflow.com", domainingestion.TablePosts, "archive", 1, 1, time.Now()), found: true, cleanErr: cleanCause}, want: cleanCause, wantOffset: 1},
		{name: "save", store: &fakeCheckpointStore{saveErr: saveCause}, want: saveCause, wantOffset: 1, wantSaves: 1, wantProcess: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stream := &fakeRecordStream{results: []streamResult{{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 1, "one", nil)}, {err: io.EOF}}}
			archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TablePosts: stream}, identity: "archive"}
			service := NewService(&fakeArchiveFactory{archive: archive}, &fakeWriter{}, test.store, nil)
			command := testCommandWithModeAndThreshold(t, []domainingestion.Table{domainingestion.TablePosts}, false, 100)
			command.CheckpointInterval = 1

			summary, err := service.Run(context.Background(), command)

			if !errors.Is(err, test.want) {
				t.Fatalf("Run() error = %v", err)
			}
			var runErr RunError
			if !errors.As(err, &runErr) || runErr.Offset != test.wantOffset {
				t.Fatalf("RunError = %#v", err)
			}
			if len(test.store.saves) != test.wantSaves {
				t.Fatalf("saves = %#v", test.store.saves)
			}
			if test.wantProcess > 0 && (len(summary.Tables) != 1 || summary.Tables[0].Processed != test.wantProcess) {
				t.Fatalf("summary = %#v", summary)
			}
		})
	}
}

func acceptedBatchSizes(batches [][]domainingestion.SourceRecord) []int {
	sizes := make([]int, len(batches))
	for index, batch := range batches {
		sizes[index] = len(batch)
	}
	return sizes
}

func testCommand(t *testing.T, tables []domainingestion.Table) Command {
	return testCommandWithThreshold(t, tables, 0.5)
}

func testCommandWithThreshold(t *testing.T, tables []domainingestion.Table, threshold float64) Command {
	return testCommandWithModeAndThreshold(t, tables, true, threshold)
}

func testCommandWithModeAndThreshold(t *testing.T, tables []domainingestion.Table, dryRun bool, threshold float64) Command {
	t.Helper()
	command, err := NewCommand(domainingestion.Site("stackoverflow.com"), "source.7z", tables, dryRun, threshold, 1024, "", DefaultCheckpointInterval)
	if err != nil {
		t.Fatalf("NewCommand() error = %v", err)
	}
	return command
}
