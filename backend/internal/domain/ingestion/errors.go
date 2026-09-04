package ingestion

import (
	"errors"
)

var ErrInvalidSite = errors.New("invalid Stack Exchange site")
var ErrUnsupportedTable = errors.New("unsupported corpus table")
var ErrEmptyTables = errors.New("at least one corpus table is required")

type UnsupportedTableError struct {
	Value string
}

func (e UnsupportedTableError) Error() string {
	return e.Value
}
func (e UnsupportedTableError) Unwrap() error {
	return ErrUnsupportedTable
}
