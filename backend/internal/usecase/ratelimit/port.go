package ratelimit

import (
	"context"
	"time"

	domainratelimit "quorum/internal/domain/ratelimit"
)

type Store interface {
	Take(ctx context.Context, key string, limit int, window time.Duration) (domainratelimit.Budget, error)
}
