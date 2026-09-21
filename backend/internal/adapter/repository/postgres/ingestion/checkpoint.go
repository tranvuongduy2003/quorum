package ingestion

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	domainingestion "quorum/internal/domain/ingestion"
	usecaseingestion "quorum/internal/usecase/ingestion"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ usecaseingestion.CheckpointStore = CheckpointRepository{}

type checkpointRow interface {
	Scan(...any) error
}

type checkpointTransaction interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

type checkpointDatabase interface {
	QueryRow(context.Context, string, ...any) checkpointRow
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Begin(context.Context) (checkpointTransaction, error)
}

type poolCheckpointDatabase struct {
	pool *pgxpool.Pool
}

func (d poolCheckpointDatabase) QueryRow(ctx context.Context, query string, args ...any) checkpointRow {
	return d.pool.QueryRow(ctx, query, args...)
}

func (d poolCheckpointDatabase) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return d.pool.Exec(ctx, query, args...)
}

func (d poolCheckpointDatabase) Begin(ctx context.Context) (checkpointTransaction, error) {
	return d.pool.Begin(ctx)
}

type CheckpointRepository struct {
	db checkpointDatabase
}

func NewCheckpointRepository(pool *pgxpool.Pool) CheckpointRepository {
	return CheckpointRepository{db: poolCheckpointDatabase{pool: pool}}
}

func (r CheckpointRepository) Load(
	ctx context.Context,
	site domainingestion.Site,
	table domainingestion.Table,
) (domainingestion.Checkpoint, bool, error) {
	if err := ctx.Err(); err != nil {
		return domainingestion.Checkpoint{}, false, err
	}

	const query = `SELECT archive_id, source_offset, confirmed_count, updated_at
FROM ingest_checkpoints
WHERE site = $1 AND source_table = $2`

	var archiveID string
	var sourceOffset int64
	var confirmedCount int64
	var updatedAt time.Time
	err := r.db.QueryRow(ctx, query, string(site), string(table)).Scan(
		&archiveID,
		&sourceOffset,
		&confirmedCount,
		&updatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainingestion.Checkpoint{}, false, nil
	}
	if err != nil {
		return domainingestion.Checkpoint{}, false, fmt.Errorf("%w: %w", usecaseingestion.ErrCheckpointFailed, err)
	}

	checkpoint := domainingestion.NewCheckpoint(
		site,
		table,
		domainingestion.ArchiveIdentity(archiveID),
		sourceOffset,
		confirmedCount,
		updatedAt,
	)
	return checkpoint, true, nil
}

func (r CheckpointRepository) Save(ctx context.Context, checkpoint domainingestion.Checkpoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	const query = `INSERT INTO ingest_checkpoints (site, source_table, archive_id, source_offset, confirmed_count, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (site, source_table) DO UPDATE SET
    archive_id = EXCLUDED.archive_id,
    source_offset = EXCLUDED.source_offset,
    confirmed_count = EXCLUDED.confirmed_count,
    updated_at = EXCLUDED.updated_at`

	_, err := r.db.Exec(
		ctx,
		query,
		string(checkpoint.Site),
		string(checkpoint.Table),
		string(checkpoint.ArchiveID),
		checkpoint.SourceOffset,
		checkpoint.ConfirmedCount,
		checkpoint.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("%w: %w", usecaseingestion.ErrCheckpointFailed, err)
	}

	return nil
}

func (r CheckpointRepository) CleanAfterCheckpoint(
	ctx context.Context,
	site domainingestion.Site,
	table domainingestion.Table,
	afterOffset int64,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	plans, ok := acceptedPlans[table]
	if !ok {
		return fmt.Errorf("%w: unsupported table %q", usecaseingestion.ErrCheckpointFailed, table)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", usecaseingestion.ErrCheckpointFailed, err)
	}
	defer tx.Rollback(ctx)

	for _, plan := range plans {
		if !slices.Contains(plan.Columns, "source_offset") {
			continue
		}

		query := fmt.Sprintf(`DELETE FROM %s WHERE site = $1 AND source_offset > $2`, plan.Table.Sanitize())
		if _, err := tx.Exec(ctx, query, string(site), afterOffset); err != nil {
			return fmt.Errorf("%w: %w", usecaseingestion.ErrCheckpointFailed, err)
		}
	}

	const quarantineQuery = `DELETE FROM ingest_quarantine
WHERE site = $1 AND source_table = $2 AND source_offset > $3`
	if _, err := tx.Exec(ctx, quarantineQuery, string(site), string(table), afterOffset); err != nil {
		return fmt.Errorf("%w: %w", usecaseingestion.ErrCheckpointFailed, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%w: %w", usecaseingestion.ErrCheckpointFailed, err)
	}

	return nil
}
