package ratelimit

import (
	"context"
	"fmt"
	"time"

	domainratelimit "quorum/internal/domain/ratelimit"
	"quorum/internal/usecase/ratelimit"

	goredis "github.com/redis/go-redis/v9"
)

const rateLimitKeyPrefix = "quorum:ratelimit:v1:"

const rateLimitScript = `
local count = redis.call("INCR", KEYS[1])

if count == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end

local remaining = redis.call("PTTL", KEYS[1])

return {count, remaining}
`

type RateLimiter struct {
	client *goredis.Client
	script *goredis.Script
}

var _ ratelimit.Store = (*RateLimiter)(nil)

func NewRateLimiter(client *goredis.Client) *RateLimiter {
	return &RateLimiter{
		client: client,
		script: goredis.NewScript(rateLimitScript),
	}
}

func (l *RateLimiter) Take(ctx context.Context, key string, limit int, window time.Duration) (domainratelimit.Budget, error) {
	windowMs := window.Milliseconds()

	values, err := l.script.Run(ctx, l.client, []string{rateLimitKeyPrefix + key}, windowMs).Int64Slice()
	if err != nil {
		return domainratelimit.Budget{}, fmt.Errorf("rate limiter: take %q: %w", key, err)
	}

	if len(values) != 2 {
		return domainratelimit.Budget{}, fmt.Errorf("rate limiter: take %q: got %d values, want 2", key, len(values))
	}

	remainingMs := values[1]
	if remainingMs <= 0 {
		remainingMs = windowMs
	}

	return domainratelimit.Budget{
		Limit:      limit,
		Used:       int(values[0]),
		ResetAfter: time.Duration(remainingMs) * time.Millisecond,
	}, nil
}
