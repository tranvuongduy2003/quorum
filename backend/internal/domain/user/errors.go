package user

import "errors"

var (
	ErrInvalidID    = errors.New("user ID is required")
	ErrInvalidName  = errors.New("user name is required")
	ErrInvalidEmail = errors.New("user email is required")
	ErrNotFound     = errors.New("user was not found")
)
