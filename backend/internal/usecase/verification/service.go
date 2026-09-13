package verification

import (
	"context"
	"errors"
	"fmt"
)

var ErrInvariantViolation = errors.New("database invariant violated")

func NewService(repository Repository) Service {
	return Service{
		repository: repository,
	}
}
func (s Service) Run(ctx context.Context) ([]Check, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	checks, err := s.repository.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("run: %w", err)
	}

	for _, check := range checks {
		if check.Violations > 0 {
			return checks, fmt.Errorf("%w: check=%s violations=%d", ErrInvariantViolation, check.Name, check.Violations)
		}
	}

	return checks, nil
}
