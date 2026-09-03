package diagnostics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"quorum/internal/usecase/availability"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TimeRepository struct {
	pool *pgxpool.Pool
}

func NewTimeRepository(pool *pgxpool.Pool) TimeRepository {
	return TimeRepository{pool: pool}
}

func (r TimeRepository) Now(ctx context.Context) (time.Time, error) {
	var now time.Time

	if err := r.pool.QueryRow(ctx, `SELECT CURRENT_TIMESTAMP`).Scan(&now); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return time.Time{}, err
		}

		return time.Time{}, fmt.Errorf("reading the database clock: %w: %w", availability.ErrUnavailable, err)
	}

	return now, nil
}
