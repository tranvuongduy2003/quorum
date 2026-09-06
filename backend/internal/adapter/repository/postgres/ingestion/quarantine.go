package ingestion

import (
	"context"
	"errors"
	"fmt"

	domainingestion "quorum/internal/domain/ingestion"
	"quorum/internal/usecase/availability"
	usecaseingestion "quorum/internal/usecase/ingestion"

	"github.com/jackc/pgx/v5/pgxpool"
)

var _ usecaseingestion.QuarantineStore = Store{}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) Store {
	return Store{pool: pool}
}

func (s Store) Save(ctx context.Context, record domainingestion.QuarantineRecord) error {
	query := `
	INSERT INTO ingest_quarantine (
		site,
		source_table,
		source_offset,
		raw_row,
		reason_code,
		found_at
	)
	VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := s.pool.Exec(
		ctx,
		query,
		record.Site.String(),
		record.Source.Table.String(),
		record.Source.Offset,
		record.Source.Raw,
		string(record.Reason),
		record.FoundAt)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return err
		}

		return fmt.Errorf("saving an ingest quarantine record: %w: %w", availability.ErrUnavailable, err)
	}

	return nil
}
