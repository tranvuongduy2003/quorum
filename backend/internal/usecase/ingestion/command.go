package ingestion

import (
	"math"
	"path/filepath"
	domainingestion "quorum/internal/domain/ingestion"
)

const (
	DefaultRejectThresholdPercent = 0.5
	DefaultMaxRecordBytes         = 8 * 1024 * 1024
	MinRecordBytes                = 1024
	MaxRecordBytes                = 64 * 1024 * 1024
)

type Command struct {
	Site                   domainingestion.Site
	ArchivePath            string
	Tables                 []domainingestion.Table
	DryRun                 bool
	RejectThresholdPercent float64
	MaxRecordBytes         int
	WatermarkPatternsPath  string
}

func NewCommand(
	site domainingestion.Site,
	archivePath string,
	tables []domainingestion.Table,
	dryRun bool,
	rejectThresholdPercent float64,
	maxRecordBytes int,
	watermarkPatternsPath string,
) (Command, error) {
	if math.IsNaN(rejectThresholdPercent) ||
		math.IsInf(rejectThresholdPercent, 0) ||
		rejectThresholdPercent < 0 ||
		rejectThresholdPercent > 100 {
		return Command{}, ErrInvalidThreshold
	}

	if maxRecordBytes < MinRecordBytes || maxRecordBytes > MaxRecordBytes {
		return Command{}, ErrInvalidRecordLimit
	}

	selected := append([]domainingestion.Table(nil), tables...)

	if archivePath != "" {
		filepath.Clean(archivePath)
	}

	return Command{
		Site:                   site,
		ArchivePath:            archivePath,
		Tables:                 selected,
		DryRun:                 dryRun,
		RejectThresholdPercent: rejectThresholdPercent,
		MaxRecordBytes:         maxRecordBytes,
		WatermarkPatternsPath:  watermarkPatternsPath,
	}, nil
}
