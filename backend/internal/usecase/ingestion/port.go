package ingestion

import (
	"context"
	domainingestion "quorum/internal/domain/ingestion"
)

type ArchiveFactory interface {
	Open(path string) (Archive, error)
}

type Archive interface {
	ValidateTables(tables []domainingestion.Table) error
	OpenTable(ctx context.Context, table domainingestion.Table, maxRecordBytes int) (RecordStream, error)
	Close() error
}

type RecordStream interface {
	Next(ctx context.Context) (domainingestion.SourceRecord, error)
	Close() error
}
