package user

import "strings"

type User struct {
	ID    string
	Name  string
	Email string
}

func NewUser(id, name, email string) (User, error) {
	if strings.TrimSpace(id) == "" {
		return User{}, ErrInvalidID
	}
	if strings.TrimSpace(name) == "" {
		return User{}, ErrInvalidName
	}
	if strings.TrimSpace(email) == "" {
		return User{}, ErrInvalidEmail
	}

	return User{ID: id, Name: name, Email: email}, nil
}
