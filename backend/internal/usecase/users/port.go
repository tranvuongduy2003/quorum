package users

import (
	"context"

	"quorum/internal/domain/user"
)

type Repository interface {
	Create(ctx context.Context, user user.User) error
	FindByID(ctx context.Context, id string) (user.User, error)
}
