package ingestion

import (
	"errors"
	"fmt"
	domainingestion "quorum/internal/domain/ingestion"
)

var ErrInvalidThreshold = errors.New("reject threshold must be a finite percentage from 0 to 100")
var ErrInvalidRecordLimit = errors.New("max record bytes must be from 1024 to 67108864")
var ErrArchiveMemberMissing = errors.New("selected table is missing from archive")
var ErrRejectThresholdExceeded = errors.New("rejected record percentage exceeds configured threshold")

type RejectThresholdError struct {
	ObservedPercent  float64
	ThresholdPercent float64
}

func (e RejectThresholdError) Error() string {
	return fmt.Sprintf("rejected record percentage %.4f exceeds configured threshold %.4f", e.ObservedPercent, e.ThresholdPercent)
}

func (e RejectThresholdError) Unwrap() error {
	return ErrRejectThresholdExceeded
}

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

type SourceError struct {
	Table  domainingestion.Table
	Offset int64
	Err    error
}

func (e SourceError) Error() string {
	return fmt.Sprintf("table=%s offset=%d cause=%s", e.Table, e.Offset, e.Err)
}
func (e SourceError) Unwrap() error {
	return e.Err
}
