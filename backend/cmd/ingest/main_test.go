package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	domainingestion "quorum/internal/domain/ingestion"
	usecaseingestion "quorum/internal/usecase/ingestion"
	"strings"
	"testing"
)

func TestRunWritesHelpWithAllDefaultsToProvidedStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := run(context.Background(), []string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run() code = %d, want 0", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	output := stderr.String()
	for _, fragment := range []string{
		"Usage: quorum-ingest [flags]",
		"--site  default=\"\"",
		"--archive  default=\"\"",
		"--watermark-patterns  default=\"\"",
		"--dry-run  default=false",
		"--reject-threshold  default=0.5",
		"--max-record-bytes  default=8388608",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("help missing %q: %s", fragment, output)
		}
	}
}

func TestRunWritesLocatedRequestErrorsToProvidedStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := run(context.Background(), []string{"--site", "stackoverflow.com", "--tables", "posts,widgets", "--dry-run"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got, want := stderr.String(), "error table=widgets offset=0 cause=unsupported corpus table \"widgets\"\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRunReturnsSyntaxExitCode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := run(context.Background(), []string{"--reject-threshold", "nope"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run() code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), `invalid value "nope" for flag -reject-threshold`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestLoadWatermarkPatternsReturnsNilForEmptyPath(t *testing.T) {
	patterns, err := loadWatermarkPatterns("")
	if err != nil || patterns != nil {
		t.Fatalf("loadWatermarkPatterns() = %#v, %v", patterns, err)
	}
}

func TestLoadWatermarkPatternsNormalizesAndDeduplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patterns.txt")
	if err := os.WriteFile(path, []byte(" first \r\n\nsecond\nfirst\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	patterns, err := loadWatermarkPatterns(path)
	if err != nil {
		t.Fatalf("loadWatermarkPatterns() error = %v", err)
	}
	if got, want := strings.Join(patterns, ","), "first,second"; got != want {
		t.Fatalf("patterns = %q, want %q", got, want)
	}
}

func TestLoadWatermarkPatternsRejectsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patterns.txt")
	if err := os.WriteFile(path, []byte(" \r\n\t\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := loadWatermarkPatterns(path)
	if !errors.Is(err, domainingestion.ErrEmptyWatermarkPatterns) {
		t.Fatalf("loadWatermarkPatterns() error = %v", err)
	}
}

func TestLoadWatermarkPatternsRejectsInvalidUTF8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patterns.txt")
	if err := os.WriteFile(path, []byte{0xff}, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := loadWatermarkPatterns(path); err == nil {
		t.Fatal("loadWatermarkPatterns() error = nil")
	}
}

func TestLoadWatermarkPatternsRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patterns.txt")
	data := bytes.Repeat([]byte{'x'}, MaxWatermarkPatternFileBytes+1)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := loadWatermarkPatterns(path); err == nil {
		t.Fatal("loadWatermarkPatterns() error = nil")
	}
}

func TestRunLocatesPatternFileErrorsAtRequest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patterns.txt")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(context.Background(), []string{
		"--site", "stackoverflow.com",
		"--archive", "unused.7z",
		"--tables", "posts",
		"--watermark-patterns", path,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got, want := stderr.String(), "error table=request offset=0 cause=watermark policy requires at least one pattern\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestWriteRunResultPrintsCompleteSummaryBeforeTerminalError(t *testing.T) {
	summary := usecaseingestion.RunSummary{
		Tables: []usecaseingestion.TableSummary{
			{
				Table:     domainingestion.TablePosts,
				Processed: 2,
				Valid:     1,
				Rejected:  1,
				Rejections: map[domainingestion.ReasonCode]int64{
					domainingestion.ReasonWatermarkPattern: 1,
				},
				LastOffset: 16,
			},
		},
		DryRun:           true,
		ThresholdPercent: 10,
		ObservedPercent:  50,
		Status:           usecaseingestion.RunStatusFailed,
	}
	runErr := usecaseingestion.RunError{
		Table:  "posts",
		Offset: 16,
		Err: usecaseingestion.RejectThresholdError{
			ObservedPercent:  50,
			ThresholdPercent: 10,
		},
	}
	var output bytes.Buffer

	code := writeRunResult(&output, &output, summary, runErr)

	if code != 1 {
		t.Fatalf("writeRunResult() code = %d, want 1", code)
	}
	want := "table=posts processed=2 valid=1 rejected=1 malformed=0\n" +
		"rejection table=posts reason=watermark_pattern count=1\n" +
		"status=failed dry_run=true threshold_percent=10.0000 observed_percent=50.0000\n" +
		"error table=posts offset=16 cause=rejected record percentage 50.0000 exceeds configured threshold 10.0000\n"
	if got := output.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
