package diagnostics

import (
	"context"
	"time"
)

type Service interface {
	DatabaseTime(ctx context.Context) (time.Time, error)
}

func NewService(db DatabaseTimeSource) Service {
	return &diagnosticsUseCase{db: db}
}

type diagnosticsUseCase struct {
	db DatabaseTimeSource
}

func (d diagnosticsUseCase) DatabaseTime(ctx context.Context) (time.Time, error) {
	return d.db.Now(ctx)
}
