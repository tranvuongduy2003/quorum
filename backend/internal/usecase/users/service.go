package users

import (
	"context"

	domainuser "quorum/internal/domain/user"
)

type Service interface {
	Create(ctx context.Context, input CreateInput) (domainuser.User, error)
	GetByID(ctx context.Context, id string) (domainuser.User, error)
}

type userUseCase struct {
	users Repository
}

func NewService(users Repository) Service {
	return userUseCase{users: users}
}

func (u userUseCase) Create(ctx context.Context, input CreateInput) (domainuser.User, error) {
	user, err := domainuser.NewUser(input.ID, input.Name, input.Email)
	if err != nil {
		return domainuser.User{}, err
	}
	if err := u.users.Create(ctx, user); err != nil {
		return domainuser.User{}, err
	}
	return user, nil
}

func (u userUseCase) GetByID(ctx context.Context, id string) (domainuser.User, error) {
	if id == "" {
		return domainuser.User{}, domainuser.ErrInvalidID
	}
	return u.users.FindByID(ctx, id)
}
