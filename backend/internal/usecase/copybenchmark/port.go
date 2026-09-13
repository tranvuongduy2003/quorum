package copybenchmark

import "context"

type Runner interface {
	Prepare(context.Context) error
	WarmUp(context.Context, int64) error
	Run(context.Context, Strategy, int64) (Trial, error)
	Environment(context.Context) (Environment, error)
}
