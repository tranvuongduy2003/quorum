package ingestion

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domainingestion "quorum/internal/domain/ingestion"
	usecaseingestion "quorum/internal/usecase/ingestion"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeCheckpointRow struct {
	scan func(...any) error
}

func (r fakeCheckpointRow) Scan(destinations ...any) error {
	return r.scan(destinations...)
}

type checkpointCall struct {
	query string
	args  []any
}

type fakeCheckpointTransaction struct {
	calls       []checkpointCall
	execErrors  []error
	committed   bool
	rolledBack  bool
	commitError error
}

func (tx *fakeCheckpointTransaction) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	tx.calls = append(tx.calls, checkpointCall{query: query, args: append([]any(nil), args...)})
	index := len(tx.calls) - 1
	if index < len(tx.execErrors) && tx.execErrors[index] != nil {
		return pgconn.CommandTag{}, tx.execErrors[index]
	}
	return pgconn.NewCommandTag("DELETE 1"), nil
}

func (tx *fakeCheckpointTransaction) Commit(context.Context) error {
	tx.committed = true
	return tx.commitError
}

func (tx *fakeCheckpointTransaction) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

type fakeCheckpointDatabase struct {
	row        checkpointRow
	queryCall  checkpointCall
	execCall   checkpointCall
	execError  error
	tx         *fakeCheckpointTransaction
	beginError error
	beginCalls int
}

func (db *fakeCheckpointDatabase) QueryRow(_ context.Context, query string, args ...any) checkpointRow {
	db.queryCall = checkpointCall{query: query, args: append([]any(nil), args...)}
	return db.row
}

func (db *fakeCheckpointDatabase) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	db.execCall = checkpointCall{query: query, args: append([]any(nil), args...)}
	return pgconn.NewCommandTag("INSERT 0 1"), db.execError
}

func (db *fakeCheckpointDatabase) Begin(context.Context) (checkpointTransaction, error) {
	db.beginCalls++
	if db.beginError != nil {
		return nil, db.beginError
	}
	return db.tx, nil
}

func TestCheckpointRepositoryLoadReturnsStoredCheckpoint(t *testing.T) {
	updatedAt := time.Date(2026, time.September, 21, 3, 30, 0, 0, time.UTC)
	db := &fakeCheckpointDatabase{row: fakeCheckpointRow{scan: func(destinations ...any) error {
		*destinations[0].(*string) = "archive"
		*destinations[1].(*int64) = 44
		*destinations[2].(*int64) = 20
		*destinations[3].(*time.Time) = updatedAt
		return nil
	}}}
	repository := CheckpointRepository{db: db}

	checkpoint, found, err := repository.Load(context.Background(), "academia.stackexchange.com", domainingestion.TablePosts)

	if err != nil || !found {
		t.Fatalf("Load() = %#v, %t, %v", checkpoint, found, err)
	}
	if checkpoint.Site != "academia.stackexchange.com" || checkpoint.Table != domainingestion.TablePosts || checkpoint.ArchiveID != "archive" || checkpoint.SourceOffset != 44 || checkpoint.ConfirmedCount != 20 || !checkpoint.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("checkpoint = %#v", checkpoint)
	}
	if len(db.queryCall.args) != 2 || db.queryCall.args[0] != "academia.stackexchange.com" || db.queryCall.args[1] != "posts" {
		t.Fatalf("query args = %#v", db.queryCall.args)
	}
}

func TestCheckpointRepositoryLoadDistinguishesMissingAndFailedRows(t *testing.T) {
	for _, test := range []struct {
		name      string
		scanError error
		wantError error
	}{
		{name: "missing", scanError: pgx.ErrNoRows},
		{name: "failed", scanError: errors.New("query failed"), wantError: usecaseingestion.ErrCheckpointFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := &fakeCheckpointDatabase{row: fakeCheckpointRow{scan: func(...any) error { return test.scanError }}}
			checkpoint, found, err := (CheckpointRepository{db: db}).Load(context.Background(), "academia.stackexchange.com", domainingestion.TablePosts)
			if found || checkpoint != (domainingestion.Checkpoint{}) || !errors.Is(err, test.wantError) {
				t.Fatalf("Load() = %#v, %t, %v", checkpoint, found, err)
			}
		})
	}
}

func TestCheckpointRepositorySaveUsesUpsertAndWrapsFailure(t *testing.T) {
	updatedAt := time.Date(2026, time.September, 21, 4, 0, 0, 0, time.UTC)
	checkpoint := domainingestion.NewCheckpoint("academia.stackexchange.com", domainingestion.TableVotes, "archive", 80, 50, updatedAt)
	db := &fakeCheckpointDatabase{}
	repository := CheckpointRepository{db: db}

	if err := repository.Save(context.Background(), checkpoint); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !strings.Contains(db.execCall.query, "ON CONFLICT (site, source_table) DO UPDATE SET") {
		t.Fatalf("query = %q", db.execCall.query)
	}
	if len(db.execCall.args) != 6 || db.execCall.args[0] != "academia.stackexchange.com" || db.execCall.args[1] != "votes" || db.execCall.args[2] != "archive" || db.execCall.args[3] != int64(80) || db.execCall.args[4] != int64(50) || db.execCall.args[5] != updatedAt {
		t.Fatalf("args = %#v", db.execCall.args)
	}

	db.execError = errors.New("write failed")
	if err := repository.Save(context.Background(), checkpoint); !errors.Is(err, usecaseingestion.ErrCheckpointFailed) {
		t.Fatalf("Save() error = %v", err)
	}
}

func TestCheckpointRepositoryCleanAfterCheckpointDeletesOnlyOffsetOwnedRows(t *testing.T) {
	tx := &fakeCheckpointTransaction{}
	db := &fakeCheckpointDatabase{tx: tx}
	repository := CheckpointRepository{db: db}

	if err := repository.CleanAfterCheckpoint(context.Background(), "academia.stackexchange.com", domainingestion.TablePosts, 91); err != nil {
		t.Fatalf("CleanAfterCheckpoint() error = %v", err)
	}
	if !tx.committed || !tx.rolledBack || len(tx.calls) != 2 {
		t.Fatalf("transaction = %#v", tx)
	}
	if !strings.Contains(tx.calls[0].query, `DELETE FROM "posts"`) || strings.Contains(tx.calls[0].query, "post_bodies") {
		t.Fatalf("corpus delete = %q", tx.calls[0].query)
	}
	if tx.calls[0].args[0] != "academia.stackexchange.com" || tx.calls[0].args[1] != int64(91) {
		t.Fatalf("corpus args = %#v", tx.calls[0].args)
	}
	if !strings.Contains(tx.calls[1].query, "DELETE FROM ingest_quarantine") || tx.calls[1].args[1] != "posts" || tx.calls[1].args[2] != int64(91) {
		t.Fatalf("quarantine call = %#v", tx.calls[1])
	}
}

func TestCheckpointRepositoryCleanAfterCheckpointRollsBackFailures(t *testing.T) {
	cause := errors.New("delete failed")
	tx := &fakeCheckpointTransaction{execErrors: []error{cause}}
	db := &fakeCheckpointDatabase{tx: tx}

	err := (CheckpointRepository{db: db}).CleanAfterCheckpoint(context.Background(), "academia.stackexchange.com", domainingestion.TablePosts, 91)

	if !errors.Is(err, usecaseingestion.ErrCheckpointFailed) || !errors.Is(err, cause) || tx.committed || !tx.rolledBack {
		t.Fatalf("CleanAfterCheckpoint() error = %v, transaction = %#v", err, tx)
	}
}

func TestCheckpointRepositoryCleanAfterCheckpointRejectsUnsupportedTableBeforeTransaction(t *testing.T) {
	db := &fakeCheckpointDatabase{}

	err := (CheckpointRepository{db: db}).CleanAfterCheckpoint(context.Background(), "academia.stackexchange.com", domainingestion.Table("unknown"), 0)

	if !errors.Is(err, usecaseingestion.ErrCheckpointFailed) || db.beginCalls != 0 {
		t.Fatalf("CleanAfterCheckpoint() error = %v, begin calls = %d", err, db.beginCalls)
	}
}
