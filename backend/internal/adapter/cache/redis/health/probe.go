package health

import (
	"context"

	"quorum/internal/usecase/health"

	goredis "github.com/redis/go-redis/v9"
)

type Probe struct {
	client *goredis.Client
}

var _ health.DependencyProbe = Probe{}

func NewProbe(client *goredis.Client) Probe {
	return Probe{client: client}
}

func (p Probe) Name() string {
	return "redis"
}

func (p Probe) Check(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}
