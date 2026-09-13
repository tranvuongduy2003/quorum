package verification

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeRepository struct {
	checks []Check
	err    error
}

func (r fakeRepository) Run(context.Context) ([]Check, error) {
	return r.checks, r.err
}

func TestServiceRunReturnsZeroViolationChecks(t *testing.T) {
	want := []Check{{Name: "posts_have_bodies", Violations: 0}}

	got, err := NewService(fakeRepository{checks: want}).Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Run() checks = %#v, want %#v", got, want)
	}
}

func TestServiceRunReturnsAllChecksAndFirstViolation(t *testing.T) {
	want := []Check{
		{Name: "posts_have_bodies", Violations: 2},
		{Name: "another_check", Violations: 3},
	}

	got, err := NewService(fakeRepository{checks: want}).Run(context.Background())

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Run() checks = %#v, want %#v", got, want)
	}
	if !errors.Is(err, ErrInvariantViolation) {
		t.Fatalf("Run() error = %v, want invariant violation", err)
	}
	if got, want := err.Error(), "database invariant violated: check=posts_have_bodies violations=2"; got != want {
		t.Fatalf("Run() error = %q, want %q", got, want)
	}
}

func TestServiceRunPropagatesRepositoryError(t *testing.T) {
	repositoryErr := errors.New("query failed")

	checks, err := NewService(fakeRepository{err: repositoryErr}).Run(context.Background())

	if checks != nil || !errors.Is(err, repositoryErr) {
		t.Fatalf("Run() = %#v, %v", checks, err)
	}
}

func TestServiceRunPreservesCancellationBeforeRepositoryCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	checks, err := NewService(fakeRepository{err: errors.New("must not be returned")}).Run(ctx)

	if checks != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() = %#v, %v", checks, err)
	}
}
