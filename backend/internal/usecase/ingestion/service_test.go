package ingestion

import (
	"context"
	"errors"
	"io"
	"testing"

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

func (f *fakeArchiveFactory) Open(string) (Archive, error) {
	f.opened = true
	return f.archive, nil
}

func TestServiceRunProcessesTablesSequentiallyAndClosesResources(t *testing.T) {
	posts := &fakeRecordStream{results: []streamResult{{record: domainingestion.NewSourceRecord(domainingestion.TablePosts, 8, "<row />", nil)}, {err: io.EOF}}}
	votes := &fakeRecordStream{results: []streamResult{{record: domainingestion.NewSourceRecord(domainingestion.TableVotes, 3, "<row />", nil)}, {err: io.EOF}}}
	archive := &fakeArchive{streams: map[domainingestion.Table]*fakeRecordStream{domainingestion.TablePosts: posts, domainingestion.TableVotes: votes}}
	factory := &fakeArchiveFactory{archive: archive}
	service := NewService(factory)
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
	service := NewService(&fakeArchiveFactory{archive: archive})

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
	service := NewService(&fakeArchiveFactory{archive: archive}, watermark)
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
	service := NewService(&fakeArchiveFactory{archive: archive}, watermark)

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
	service := NewService(&fakeArchiveFactory{archive: archive}, domainingestion.NewTimestampPolicy(), watermark)

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
	service := NewService(factory)
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

func testCommand(t *testing.T, tables []domainingestion.Table) Command {
	return testCommandWithThreshold(t, tables, 0.5)
}

func testCommandWithThreshold(t *testing.T, tables []domainingestion.Table, threshold float64) Command {
	t.Helper()
	command, err := NewCommand(domainingestion.Site("stackoverflow.com"), "source.7z", tables, true, threshold, 1024, "")
	if err != nil {
		t.Fatalf("NewCommand() error = %v", err)
	}
	return command
}
