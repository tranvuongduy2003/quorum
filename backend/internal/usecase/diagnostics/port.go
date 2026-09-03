package diagnostics

import (
	"context"
	"time"
)

type DatabaseTimeSource interface {
	Now(ctx context.Context) (time.Time, error)
}
