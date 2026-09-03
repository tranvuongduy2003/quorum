package message

import "errors"

var (
	ErrEmpty   = errors.New("message is required")
	ErrTooLong = errors.New("message is too long")
)
