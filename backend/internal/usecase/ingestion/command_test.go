package ingestion

import (
	"errors"
	"math"
	"path/filepath"
	"testing"

	domainingestion "quorum/internal/domain/ingestion"
)

func TestNewCommandCopiesTablesAndCleansPaths(t *testing.T) {
	tables := []domainingestion.Table{domainingestion.TablePosts}
	command, err := NewCommand(
		domainingestion.Site("stackoverflow.com"),
		"archive/../source.7z",
		tables,
		true,
		DefaultRejectThresholdPercent,
		DefaultMaxRecordBytes,
		"patterns/../watermarks.txt",
	)
	if err != nil {
		t.Fatalf("NewCommand() error = %v", err)
	}

	tables[0] = domainingestion.TableVotes
	if command.Tables[0] != domainingestion.TablePosts {
		t.Fatalf("Command.Tables = %v, want independent copy", command.Tables)
	}
	if command.ArchivePath != filepath.Clean("archive/../source.7z") {
		t.Fatalf("ArchivePath = %q", command.ArchivePath)
	}
	if command.WatermarkPatternsPath != filepath.Clean("patterns/../watermarks.txt") {
		t.Fatalf("WatermarkPatternsPath = %q", command.WatermarkPatternsPath)
	}
}

func TestNewCommandRejectsInvalidRequestValues(t *testing.T) {
	validTables := []domainingestion.Table{domainingestion.TablePosts}
	tests := []struct {
		name      string
		threshold float64
		maxBytes  int
		tables    []domainingestion.Table
		want      error
	}{
		{name: "negative threshold", threshold: -0.1, maxBytes: DefaultMaxRecordBytes, tables: validTables, want: ErrInvalidThreshold},
		{name: "over threshold", threshold: 100.1, maxBytes: DefaultMaxRecordBytes, tables: validTables, want: ErrInvalidThreshold},
		{name: "not a number", threshold: math.NaN(), maxBytes: DefaultMaxRecordBytes, tables: validTables, want: ErrInvalidThreshold},
		{name: "positive infinity", threshold: math.Inf(1), maxBytes: DefaultMaxRecordBytes, tables: validTables, want: ErrInvalidThreshold},
		{name: "negative infinity", threshold: math.Inf(-1), maxBytes: DefaultMaxRecordBytes, tables: validTables, want: ErrInvalidThreshold},
		{name: "small record", threshold: DefaultRejectThresholdPercent, maxBytes: MinRecordBytes - 1, tables: validTables, want: ErrInvalidRecordLimit},
		{name: "large record", threshold: DefaultRejectThresholdPercent, maxBytes: MaxRecordBytes + 1, tables: validTables, want: ErrInvalidRecordLimit},
		{name: "empty tables", threshold: DefaultRejectThresholdPercent, maxBytes: DefaultMaxRecordBytes, tables: nil, want: domainingestion.ErrEmptyTables},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewCommand(domainingestion.Site("stackoverflow.com"), "source.7z", test.tables, false, test.threshold, test.maxBytes, "")
			if !errors.Is(err, test.want) {
				t.Fatalf("NewCommand() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestNewCommandAcceptsInclusiveRecordLimits(t *testing.T) {
	for _, maxBytes := range []int{MinRecordBytes, MaxRecordBytes} {
		_, err := NewCommand(
			domainingestion.Site("stackoverflow.com"),
			"source.7z",
			[]domainingestion.Table{domainingestion.TablePosts},
			false,
			0,
			maxBytes,
			"",
		)
		if err != nil {
			t.Fatalf("NewCommand(maxBytes=%d) error = %v", maxBytes, err)
		}
	}
}
