package verification

import (
	"context"
)

type Check struct {
	Name       string
	Violations int64
}

type Repository interface {
	Run(context.Context) ([]Check, error)
}

type Service struct {
	repository Repository
}
