package ingestion

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	domainingestion "quorum/internal/domain/ingestion"
	"quorum/internal/usecase/availability"
	usecaseingestion "quorum/internal/usecase/ingestion"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeTransactionStarter struct {
	tx    copyTransaction
	err   error
	calls int
}

func (s *fakeTransactionStarter) Begin(context.Context) (copyTransaction, error) {
	s.calls++
	return s.tx, s.err
}

type fakeCopyTransaction struct {
	copyCounts []int64
	copyErrors []error
	tables     []pgx.Identifier
	rows       [][][]any
	commits    int
	commitErr  error
	rollbacks  int
}

func (tx *fakeCopyTransaction) CopyFrom(_ context.Context, table pgx.Identifier, _ []string, source pgx.CopyFromSource) (int64, error) {
	call := len(tx.tables)
	tx.tables = append(tx.tables, table)
	var rows [][]any
	for source.Next() {
		values, err := source.Values()
		if err != nil {
			return 0, err
		}
		rows = append(rows, append([]any(nil), values...))
	}
	if err := source.Err(); err != nil {
		return 0, err
	}
	tx.rows = append(tx.rows, rows)
	if call < len(tx.copyErrors) && tx.copyErrors[call] != nil {
		return 0, tx.copyErrors[call]
	}
	if call < len(tx.copyCounts) {
		return tx.copyCounts[call], nil
	}
	return int64(len(rows)), nil
}

func (tx *fakeCopyTransaction) Commit(context.Context) error {
	tx.commits++
	return tx.commitErr
}

func (tx *fakeCopyTransaction) Rollback(context.Context) error {
	tx.rollbacks++
	return nil
}

func TestStoreEmptyBatchesDoNotBeginTransaction(t *testing.T) {
	starter := &fakeTransactionStarter{}
	store := Store{pool: starter}

	accepted, acceptedErr := store.WriteAccepted(context.Background(), domainingestion.Site("stackoverflow.com"), domainingestion.TablePosts, nil)
	quarantined, quarantineErr := store.WriteQuarantine(context.Background(), nil)

	if acceptedErr != nil || quarantineErr != nil || accepted != 0 || quarantined != 0 {
		t.Fatalf("empty writes = accepted:(%d, %v) quarantine:(%d, %v)", accepted, acceptedErr, quarantined, quarantineErr)
	}
	if starter.calls != 0 {
		t.Fatalf("Begin() calls = %d, want 0", starter.calls)
	}
}

func TestStoreWriteAcceptedCopiesPostDestinationsInOneTransaction(t *testing.T) {
	tx := &fakeCopyTransaction{}
	starter := &fakeTransactionStarter{tx: tx}
	store := Store{pool: starter}
	records := []domainingestion.SourceRecord{validPostRecord(1, 10), validPostRecord(2, 20)}

	count, err := store.WriteAccepted(context.Background(), domainingestion.Site("stackoverflow.com"), domainingestion.TablePosts, records)

	if err != nil {
		t.Fatalf("WriteAccepted() error = %v", err)
	}
	if count != 2 || starter.calls != 1 || tx.commits != 1 || tx.rollbacks != 1 {
		t.Fatalf("write result = count:%d begins:%d commits:%d rollbacks:%d", count, starter.calls, tx.commits, tx.rollbacks)
	}
	wantTables := []pgx.Identifier{{"posts"}, {"post_bodies"}}
	if !reflect.DeepEqual(tx.tables, wantTables) {
		t.Fatalf("copied tables = %#v, want %#v", tx.tables, wantTables)
	}
	if len(tx.rows) != 2 || len(tx.rows[0]) != 2 || len(tx.rows[1]) != 2 {
		t.Fatalf("copied row counts = %#v", tx.rows)
	}
}

func TestStoreWriteAcceptedRollsBackCountMismatch(t *testing.T) {
	tx := &fakeCopyTransaction{copyCounts: []int64{0}}
	store := Store{pool: &fakeTransactionStarter{tx: tx}}
	record := validVoteRecord(1, 10)

	count, err := store.WriteAccepted(context.Background(), domainingestion.Site("stackoverflow.com"), domainingestion.TableVotes, []domainingestion.SourceRecord{record})

	if count != 0 || !errors.Is(err, usecaseingestion.ErrWriteCountMismatch) {
		t.Fatalf("WriteAccepted() = %d, %v", count, err)
	}
	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("transaction calls = commits:%d rollbacks:%d", tx.commits, tx.rollbacks)
	}
}

func TestStoreWriteQuarantineOnlyConfirmsAfterCommit(t *testing.T) {
	commitErr := errors.New("connection lost during commit")
	tx := &fakeCopyTransaction{commitErr: commitErr}
	store := Store{pool: &fakeTransactionStarter{tx: tx}}
	record := domainingestion.NewQuarantineRecord(
		domainingestion.Site("stackoverflow.com"),
		domainingestion.NewSourceRecord(domainingestion.TablePosts, 15, "raw", nil),
		domainingestion.ReasonWatermarkPattern,
		time.Unix(1, 0),
	)

	count, err := store.WriteQuarantine(context.Background(), []domainingestion.QuarantineRecord{record})

	if count != 0 || !errors.Is(err, availability.ErrUnavailable) || !errors.Is(err, commitErr) {
		t.Fatalf("WriteQuarantine() = %d, %v", count, err)
	}
	if tx.commits != 1 || tx.rollbacks != 1 {
		t.Fatalf("transaction calls = commits:%d rollbacks:%d", tx.commits, tx.rollbacks)
	}
	assertWriteFailure(t, err, 15, commitErr)
}

func TestStoreWriteAcceptedLocatesMapperFailure(t *testing.T) {
	tx := &fakeCopyTransaction{}
	store := Store{pool: &fakeTransactionStarter{tx: tx}}
	records := []domainingestion.SourceRecord{validVoteRecord(1, 10), validVoteRecord(2, 20)}
	records[1].Attributes["Id"] = "invalid"

	count, err := store.WriteAccepted(context.Background(), domainingestion.Site("stackoverflow.com"), domainingestion.TableVotes, records)

	if count != 0 || !errors.Is(err, ErrInvalidWriteAttribute) || errors.Is(err, availability.ErrUnavailable) {
		t.Fatalf("WriteAccepted() = %d, %v", count, err)
	}
	assertWriteFailure(t, err, 20, ErrInvalidWriteAttribute)
	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("transaction calls = commits:%d rollbacks:%d", tx.commits, tx.rollbacks)
	}
}

func TestStoreWriteAcceptedLocatesPostgresCopyLine(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23514", Message: "check violation", Where: "COPY votes, line 2, column id"}
	tx := &fakeCopyTransaction{copyErrors: []error{pgErr}}
	store := Store{pool: &fakeTransactionStarter{tx: tx}}
	records := []domainingestion.SourceRecord{validVoteRecord(1, 47), validVoteRecord(2, 133)}

	count, err := store.WriteAccepted(context.Background(), domainingestion.Site("stackoverflow.com"), domainingestion.TableVotes, records)

	if count != 0 || !errors.Is(err, usecaseingestion.ErrWriteRejected) || !errors.Is(err, pgErr) {
		t.Fatalf("WriteAccepted() = %d, %v", count, err)
	}
	assertWriteFailure(t, err, 133, pgErr)
	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("transaction calls = commits:%d rollbacks:%d", tx.commits, tx.rollbacks)
	}
}

func TestStoreWriteQuarantineLocatesPostgresCopyLine(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23514", Message: "check violation", Where: "COPY ingest_quarantine, line 1, column source_offset"}
	tx := &fakeCopyTransaction{copyErrors: []error{pgErr}}
	store := Store{pool: &fakeTransactionStarter{tx: tx}}
	records := []domainingestion.QuarantineRecord{
		domainingestion.NewQuarantineRecord(domainingestion.Site("stackoverflow.com"), domainingestion.NewSourceRecord(domainingestion.TablePosts, 47, "first", nil), domainingestion.ReasonWatermarkPattern, time.Unix(1, 0)),
		domainingestion.NewQuarantineRecord(domainingestion.Site("stackoverflow.com"), domainingestion.NewSourceRecord(domainingestion.TablePosts, 133, "second", nil), domainingestion.ReasonWatermarkPattern, time.Unix(2, 0)),
	}

	count, err := store.WriteQuarantine(context.Background(), records)

	if count != 0 || !errors.Is(err, usecaseingestion.ErrWriteRejected) || !errors.Is(err, pgErr) {
		t.Fatalf("WriteQuarantine() = %d, %v", count, err)
	}
	assertWriteFailure(t, err, 47, pgErr)
}

func TestLocateCopyFailureUsesPreciseAndFallbackOffsets(t *testing.T) {
	offsets := []int64{47, 91, 133}
	tests := []struct {
		name  string
		where string
		want  int64
	}{
		{name: "first", where: "COPY votes, line 1, column id", want: 47},
		{name: "middle", where: "COPY votes, line 2, column id", want: 91},
		{name: "last", where: "COPY votes, line 3, column id", want: 133},
		{name: "missing", want: 133},
		{name: "zero", where: "COPY votes, line 0, column id", want: 133},
		{name: "negative", where: "COPY votes, line -1, column id", want: 133},
		{name: "malformed", where: "COPY votes, line many, column id", want: 133},
		{name: "out of range", where: "COPY votes, line 4, column id", want: 133},
		{name: "unrelated number", where: "COPY votes, code 2", want: 133},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pgErr := &pgconn.PgError{Code: "23514", Message: "check violation", Where: test.where}
			err := locateCopyFailure(len(offsets), func(index int) int64 { return offsets[index] }, pgErr)

			if !errors.Is(err, usecaseingestion.ErrWriteRejected) {
				t.Fatalf("locateCopyFailure() error = %v", err)
			}
			assertWriteFailure(t, err, test.want, pgErr)
		})
	}
}

func TestStoreWriteFailuresUseLastBatchOffsetWithoutCopyLine(t *testing.T) {
	beginErr := errors.New("begin failed")
	records := []domainingestion.SourceRecord{validVoteRecord(1, 47), validVoteRecord(2, 133)}
	store := Store{pool: &fakeTransactionStarter{err: beginErr}}

	_, err := store.WriteAccepted(context.Background(), domainingestion.Site("stackoverflow.com"), domainingestion.TableVotes, records)

	if !errors.Is(err, availability.ErrUnavailable) {
		t.Fatalf("WriteAccepted() error = %v", err)
	}
	assertWriteFailure(t, err, 133, beginErr)

	canceled := fmt.Errorf("copy canceled: %w", context.Canceled)
	tx := &fakeCopyTransaction{copyErrors: []error{canceled}}
	store = Store{pool: &fakeTransactionStarter{tx: tx}}

	_, err = store.WriteAccepted(context.Background(), domainingestion.Site("stackoverflow.com"), domainingestion.TableVotes, records)

	if !errors.Is(err, context.Canceled) || errors.Is(err, availability.ErrUnavailable) {
		t.Fatalf("WriteAccepted() cancellation = %v", err)
	}
	assertWriteFailure(t, err, 133, context.Canceled)
}

func TestStoreWritesPreserveContextCancellation(t *testing.T) {
	starter := &fakeTransactionStarter{}
	store := Store{pool: starter}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, acceptedErr := store.WriteAccepted(ctx, domainingestion.Site("stackoverflow.com"), domainingestion.TablePosts, []domainingestion.SourceRecord{validPostRecord(1, 1)})
	_, quarantineErr := store.WriteQuarantine(ctx, []domainingestion.QuarantineRecord{{}})

	for _, err := range []error{acceptedErr, quarantineErr} {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("write error = %v, want context cancellation", err)
		}
		if errors.Is(err, availability.ErrUnavailable) {
			t.Fatalf("write error = %v, unexpectedly classified unavailable", err)
		}
	}
	if starter.calls != 0 {
		t.Fatalf("Begin() calls = %d, want 0", starter.calls)
	}
}

func TestClassifyWriteErrorSeparatesRejectionAndAvailability(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23514", Message: "check violation"}
	rejected := classifyWriteError(pgErr)
	if !errors.Is(rejected, usecaseingestion.ErrWriteRejected) || !errors.Is(rejected, pgErr) || errors.Is(rejected, availability.ErrUnavailable) {
		t.Fatalf("PostgreSQL error classification = %v", rejected)
	}

	driverErr := errors.New("driver failed")
	unavailable := classifyWriteError(driverErr)
	if !errors.Is(unavailable, availability.ErrUnavailable) || !errors.Is(unavailable, driverErr) || errors.Is(unavailable, usecaseingestion.ErrWriteRejected) {
		t.Fatalf("driver error classification = %v", unavailable)
	}

	canceled := fmt.Errorf("wrapped: %w", context.Canceled)
	if got := classifyWriteError(canceled); got != canceled {
		t.Fatalf("cancellation classification = %v, want original error", got)
	}
}

func TestStoreRejectsMixedTableBatchBeforeTransaction(t *testing.T) {
	starter := &fakeTransactionStarter{}
	store := Store{pool: starter}
	record := validVoteRecord(1, 1)

	_, err := store.WriteAccepted(context.Background(), domainingestion.Site("stackoverflow.com"), domainingestion.TablePosts, []domainingestion.SourceRecord{record})

	if !errors.Is(err, domainingestion.ErrUnsupportedTable) {
		t.Fatalf("WriteAccepted() error = %v", err)
	}
	if starter.calls != 0 {
		t.Fatalf("Begin() calls = %d, want 0", starter.calls)
	}
}

func validPostRecord(id int64, offset int64) domainingestion.SourceRecord {
	return domainingestion.NewSourceRecord(domainingestion.TablePosts, offset, "raw", map[string]string{
		"Id":               strconv.FormatInt(id, 10),
		"PostTypeId":       "1",
		"CreationDate":     "2026-01-02T03:04:05.006",
		"Score":            "0",
		"LastActivityDate": "2026-01-02T03:04:05.006",
		"Body":             "body",
	})
}

func validVoteRecord(id int64, offset int64) domainingestion.SourceRecord {
	return domainingestion.NewSourceRecord(domainingestion.TableVotes, offset, "raw", map[string]string{
		"Id":           strconv.FormatInt(id, 10),
		"PostId":       "1",
		"VoteTypeId":   "2",
		"CreationDate": "2026-01-02T03:04:05.006",
	})
}

func assertWriteFailure(t *testing.T, err error, offset int64, cause error) {
	t.Helper()
	var failure usecaseingestion.WriteFailure
	if !errors.As(err, &failure) {
		t.Fatalf("error %v does not contain WriteFailure", err)
	}
	if failure.Offset != offset {
		t.Fatalf("WriteFailure offset = %d, want %d", failure.Offset, offset)
	}
	if !errors.Is(failure.Err, cause) {
		t.Fatalf("WriteFailure error = %v, want cause %v", failure.Err, cause)
	}
}
