package transaction

import "context"

type TransactionManager interface {
	WithinTransaction(ctx context.Context, operation func(context.Context) error) error
}
