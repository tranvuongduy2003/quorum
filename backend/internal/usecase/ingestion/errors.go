package ingestion

import (
	"errors"
	"fmt"
)

var ErrInvalidThreshold = errors.New("reject threshold must be a finite percentage from 0 to 100")
var ErrInvalidRecordLimit = errors.New("max record bytes must be from 1024 to 67108864")

type RunError struct {
	Table  string
	Offset int64
	Err    error
}

func (e RunError) Error() string {
	return fmt.Sprintf("error table=%s offset=%d cause=%s", e.Table, e.Offset, e.Err)
}
func (e RunError) Unwrap() error {
	return e.Err
}
