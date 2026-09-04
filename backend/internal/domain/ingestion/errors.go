package ingestion

import (
	"errors"
	"fmt"
)

var ErrInvalidSite = errors.New("invalid Stack Exchange site")
var ErrUnsupportedTable = errors.New("unsupported corpus table")
var ErrEmptyTables = errors.New("at least one corpus table is required")

type UnsupportedTableError struct {
	Value string
}

func (e UnsupportedTableError) Error() string {
	return fmt.Sprintf("%s %q", ErrUnsupportedTable, e.Value)
}

func (e UnsupportedTableError) Unwrap() error {
	return ErrUnsupportedTable
}
