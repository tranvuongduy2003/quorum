package health

import (
	"context"

	"quorum/internal/usecase/health"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Probe struct {
	pool *pgxpool.Pool
}

var _ health.DependencyProbe = Probe{}

func NewProbe(pool *pgxpool.Pool) Probe {
	return Probe{pool: pool}
}

func (p Probe) Name() string {
	return "postgres"
}

func (p Probe) Check(ctx context.Context) error {
	return p.pool.Ping(ctx)
}
