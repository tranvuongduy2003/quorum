package ingestion

import (
	"context"
	"errors"
	"fmt"

	domainingestion "quorum/internal/domain/ingestion"
	"quorum/internal/usecase/availability"
	usecaseingestion "quorum/internal/usecase/ingestion"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ usecaseingestion.Writer = Store{}

type transactionStarter interface {
	Begin(context.Context) (copyTransaction, error)
}

type copyTransaction interface {
	Commit(context.Context) error
	Rollback(context.Context) error
	CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error)
}

type Store struct {
	pool transactionStarter
}

type poolTransactionStarter struct {
	pool *pgxpool.Pool
}

func (s poolTransactionStarter) Begin(ctx context.Context) (copyTransaction, error) {
	return s.pool.Begin(ctx)
}

func NewStore(pool *pgxpool.Pool) Store {
	return Store{pool: poolTransactionStarter{pool: pool}}
}

func (s Store) WriteAccepted(
	ctx context.Context,
	site domainingestion.Site,
	table domainingestion.Table,
	records []domainingestion.SourceRecord,
) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	plans, ok := acceptedPlans[table]
	if !ok {
		return 0, fmt.Errorf("%w: %q", domainingestion.ErrUnsupportedTable, table)
	}
	if len(records) == 0 {
		return 0, nil
	}

	for idx, record := range records {
		if record.Table != table {
			return 0, fmt.Errorf("%w: record at index %d has table %q, expected %q",
				domainingestion.ErrUnsupportedTable,
				idx,
				record.Table,
				table,
			)
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, classifyWriteError(err)
	}
	defer tx.Rollback(ctx)

	for _, plan := range plans {
		count, err := copyAcceptedDestination(ctx, tx, plan, site, records)
		if err != nil {
			return 0, classifyWriteError(err)
		}

		if count != int64(len(records)) {
			return 0, fmt.Errorf("%w: destination=%s requested=%d confirmed=%d", usecaseingestion.ErrWriteCountMismatch, plan.Table.Sanitize(), len(records), count)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, classifyWriteError(err)
	}

	return int64(len(records)), nil
}

func (s Store) WriteQuarantine(ctx context.Context, records []domainingestion.QuarantineRecord) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if len(records) == 0 {
		return 0, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, classifyWriteError(err)
	}
	defer tx.Rollback(ctx)

	copied, err := copyQuarantineDestination(ctx, tx, quarantinePlan, records)
	if err != nil {
		return 0, classifyWriteError(err)
	}

	if copied != int64(len(records)) {
		return 0, fmt.Errorf("%w: destination=%s requested=%d confirmed=%d", usecaseingestion.ErrWriteCountMismatch, quarantinePlan.Table.Sanitize(), len(records), copied)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, classifyWriteError(err)
	}

	return copied, nil
}

func copyAcceptedDestination(
	ctx context.Context,
	tx copyTransaction,
	plan destinationPlan,
	site domainingestion.Site,
	records []domainingestion.SourceRecord,
) (int64, error) {
	source := pgx.CopyFromSlice(len(records), func(index int) ([]any, error) {
		return plan.Map(site, records[index])
	})

	return tx.CopyFrom(ctx, plan.Table, plan.Columns, source)
}

func copyQuarantineDestination(ctx context.Context, tx copyTransaction, plan destinationPlan, records []domainingestion.QuarantineRecord) (int64, error) {
	source := pgx.CopyFromSlice(len(records), func(index int) ([]any, error) {
		return mapQuarantineRow(records[index]), nil
	})

	return tx.CopyFrom(ctx, plan.Table, plan.Columns, source)
}

func classifyWriteError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		return fmt.Errorf("%w: %w", usecaseingestion.ErrWriteRejected, err)
	}

	return fmt.Errorf("%w: %w", availability.ErrUnavailable, err)
}
