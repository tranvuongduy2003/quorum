package ingestion

import (
	"context"
	"errors"
	"io"
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
	streams map[domainingestion.Table]*fakeRecordStream
	calls   []string
	closed  bool
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

type fakeArchiveFactory struct {
	archive *fakeArchive
	opened  bool
}

type fakeQuarantineStore struct {
	records []domainingestion.QuarantineRecord
	err     error
}

func (s *fakeQuarantineStore) Save(_ context.Context, record domainingestion.QuarantineRecord) error {
	s.records = append(s.records, record)
	return s.err
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
	service := NewService(factory, nil, nil)
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
	service := NewService(&fakeArchiveFactory{archive: archive}, nil, nil)

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
	service := NewService(&fakeArchiveFactory{archive: archive}, nil, nil, watermark)

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
	service := NewService(&fakeArchiveFactory{archive: archive}, nil, nil, watermark)
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
	service := NewService(&fakeArchiveFactory{archive: archive}, nil, nil, watermark)

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
	service := NewService(&fakeArchiveFactory{archive: archive}, nil, nil, domainingestion.NewTimestampPolicy(), watermark)

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
	service := NewService(factory, nil, nil)
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

func TestServiceRunRequiresQuarantineStoreBeforeOpeningArchive(t *testing.T) {
	factory := &fakeArchiveFactory{archive: &fakeArchive{}}
	service := NewService(factory, nil, nil)
	command := testCommandWithModeAndThreshold(t, []domainingestion.Table{domainingestion.TablePosts}, false, 100)

	summary, err := service.Run(context.Background(), command)

	if factory.opened {
		t.Fatal("Run() opened an archive without a quarantine store")
	}
	if summary.Status != RunStatusFailed {
		t.Fatalf("summary status = %q, want %q", summary.Status, RunStatusFailed)
	}
	var runErr RunError
	if !errors.As(err, &runErr) || runErr.Table != "request" || runErr.Offset != 0 || !errors.Is(err, ErrQuarantineStoreRequired) {
		t.Fatalf("Run() error = %#v", err)
	}
}

func TestServiceRunPersistsOnlyRejectedRecordsWithCurrentTime(t *testing.T) {
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
	store := &fakeQuarantineStore{}
	foundAt := time.Date(2026, time.September, 6, 10, 30, 0, 0, time.FixedZone("ICT", 7*60*60))
	service := NewService(&fakeArchiveFactory{archive: archive}, store, func() time.Time { return foundAt }, watermark)
	command := testCommandWithModeAndThreshold(t, []domainingestion.Table{domainingestion.TablePosts}, false, 100)

	summary, err := service.Run(context.Background(), command)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Tables[0].Rejected != 1 || summary.Tables[0].Valid != 1 {
		t.Fatalf("table summary = %#v", summary.Tables[0])
	}
	if got, want := len(store.records), 1; got != want {
		t.Fatalf("saved records = %d, want %d", got, want)
	}
	record := store.records[0]
	if record.Site != command.Site || record.Source.Offset != rejected.Offset || record.Source.Raw != rejected.Raw || record.Reason != domainingestion.ReasonWatermarkPattern {
		t.Fatalf("saved record = %#v", record)
	}
	if got, want := record.FoundAt, foundAt.UTC(); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("found at = %v, want %v in UTC", got, want)
	}
}

func TestServiceRunStopsOnQuarantineFailureWithPartialSummary(t *testing.T) {
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
	store := &fakeQuarantineStore{err: saveErr}
	service := NewService(&fakeArchiveFactory{archive: archive}, store, nil, watermark)
	command := testCommandWithModeAndThreshold(t, []domainingestion.Table{domainingestion.TablePosts}, false, 100)

	summary, err := service.Run(context.Background(), command)

	if summary.Status != RunStatusFailed || len(summary.Tables) != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	table := summary.Tables[0]
	if table.Processed != 1 || table.Rejected != 1 || table.Valid != 0 || table.LastOffset != 19 {
		t.Fatalf("partial table summary = %#v", table)
	}
	if posts.index != 1 || len(store.records) != 1 {
		t.Fatalf("processing continued after failure: stream index=%d saved=%d", posts.index, len(store.records))
	}
	var runErr RunError
	if !errors.As(err, &runErr) || runErr.Table != "posts" || runErr.Offset != 19 || !errors.Is(err, saveErr) {
		t.Fatalf("Run() error = %#v", err)
	}
}

func TestServiceRunDryRunMakesNoQuarantineCalls(t *testing.T) {
	watermark, err := domainingestion.NewWatermarkPolicy([]string{"synthetic-marker"})
	if err != nil {
		t.Fatalf("NewWatermarkPolicy() error = %v", err)
	}
	posts := &fakeRecordStream{results: []streamResult{
		{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 8, "synthetic-marker", nil)},
		{err: io.EOF},
	}}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TablePosts: posts}}
	store := &fakeQuarantineStore{}
	service := NewService(&fakeArchiveFactory{archive: archive}, store, nil, watermark)

	summary, err := service.Run(context.Background(), testCommandWithThreshold(t, []domainingestion.Table{domainingestion.TablePosts}, 100))

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Tables[0].Rejected != 1 || len(store.records) != 0 {
		t.Fatalf("summary = %#v, saved records = %d", summary, len(store.records))
	}
}

func testCommand(t *testing.T, tables []domainingestion.Table) Command {
	return testCommandWithThreshold(t, tables, 0.5)
}

func testCommandWithThreshold(t *testing.T, tables []domainingestion.Table, threshold float64) Command {
	return testCommandWithModeAndThreshold(t, tables, true, threshold)
}

func testCommandWithModeAndThreshold(t *testing.T, tables []domainingestion.Table, dryRun bool, threshold float64) Command {
	t.Helper()
	command, err := NewCommand(domainingestion.Site("stackoverflow.com"), "source.7z", tables, dryRun, threshold, 1024, "")
	if err != nil {
		t.Fatalf("NewCommand() error = %v", err)
	}
	return command
}
