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
	Identity() domainingestion.ArchiveIdentity
}

type RecordStream interface {
	Next(ctx context.Context) (domainingestion.SourceRecord, error)
	Close() error
}

type Writer interface {
	WriteAccepted(context.Context, domainingestion.Site, domainingestion.Table, []domainingestion.SourceRecord) (int64, error)
	WriteQuarantine(context.Context, []domainingestion.QuarantineRecord) (int64, error)
}

type CheckpointStore interface {
	Load(context.Context, domainingestion.Site, domainingestion.Table) (domainingestion.Checkpoint, bool, error)
	Save(context.Context, domainingestion.Checkpoint) error
	CleanAfterCheckpoint(context.Context, domainingestion.Site, domainingestion.Table, int64) error
}

type CheckpointReporter interface {
	CheckpointSaved(domainingestion.Checkpoint)
}
