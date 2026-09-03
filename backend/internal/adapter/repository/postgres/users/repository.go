package users

import (
	"context"
	"errors"

	"quorum/internal/domain/user"
)

var ErrNotConfigured = errors.New("PostgreSQL repository is not configured")

type UserRepository struct{}

func (UserRepository) Create(_ context.Context, _ user.User) error {
	return ErrNotConfigured
}

func (UserRepository) FindByID(_ context.Context, _ string) (user.User, error) {
	return user.User{}, ErrNotConfigured
}
