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
	t.Helper()
	command, err := NewCommand(domainingestion.Site("stackoverflow.com"), "source.7z", tables, true, 0.5, 1024, "")
	if err != nil {
		t.Fatalf("NewCommand() error = %v", err)
	}
	return command
}
