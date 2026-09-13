package verification

import (
	"context"
	"fmt"

	usecaseverification "quorum/internal/usecase/verification"

	"github.com/jackc/pgx/v5/pgxpool"
)

var _ usecaseverification.Repository = Repository{}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return Repository{
		pool: pool,
	}
}

func (r Repository) Run(ctx context.Context) ([]usecaseverification.Check, error) {
	var count int64

	query := `
		SELECT count(*)
		FROM posts AS p
		LEFT JOIN post_bodies AS b
		  ON b.site = p.site
		 AND b.post_id = p.id
		WHERE b.post_id IS NULL
	`

	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("run verification check posts_have_bodies: %w", err)
	}

	return []usecaseverification.Check{
		{
			Name:       "posts_have_bodies",
			Violations: count,
		},
	}, nil
}
