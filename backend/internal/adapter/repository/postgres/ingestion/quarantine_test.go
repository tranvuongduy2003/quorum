package ingestion

import (
	"context"
	"errors"
	"testing"

	"quorum/internal/domain/ingestion"
	"quorum/internal/usecase/availability"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStoreSavePreservesContextCancellation(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://user:password@127.0.0.1:1/database")
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = NewStore(pool).Save(ctx, ingestion.QuarantineRecord{})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Save() error = %v, want context cancellation", err)
	}
	if errors.Is(err, availability.ErrUnavailable) {
		t.Fatalf("Save() error = %v, unexpectedly classified unavailable", err)
	}
}

func TestStoreSaveClassifiesDriverFailureAsUnavailable(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://user:password@127.0.0.1:1/database")
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	pool.Close()

	err = NewStore(pool).Save(context.Background(), ingestion.QuarantineRecord{})

	if !errors.Is(err, availability.ErrUnavailable) {
		t.Fatalf("Save() error = %v, want unavailable classification", err)
	}
}
