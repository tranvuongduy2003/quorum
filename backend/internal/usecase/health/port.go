package health

import "context"

type DependencyProbe interface {
	Name() string
	Check(ctx context.Context) error
}
