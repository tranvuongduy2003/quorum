package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	usecasecopybenchmark "quorum/internal/usecase/copybenchmark"
)

func TestParseOptionsDefaultsAndOverrides(t *testing.T) {
	defaults, err := parseOptions(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseOptions() error = %v", err)
	}
	if defaults.rows != 1_000_000 || defaults.repetitions != 3 || defaults.output != "../docs/benchmarks/SPEC-003-copy-results.md" {
		t.Fatalf("defaults = %+v", defaults)
	}
	overrides, err := parseOptions([]string{"--rows", "10000", "--repetitions", "3", "--output", "result.md"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseOptions() overrides error = %v", err)
	}
	if overrides.rows != 10_000 || overrides.repetitions != 3 || overrides.output != "result.md" {
		t.Fatalf("overrides = %+v", overrides)
	}
}

func TestParseOptionsRejectsEmptyOutputAndPositionals(t *testing.T) {
	if _, err := parseOptions([]string{"--output", " "}, &bytes.Buffer{}); err == nil {
		t.Fatal("empty output accepted")
	}
	if _, err := parseOptions([]string{"extra"}, &bytes.Buffer{}); err == nil {
		t.Fatal("positional argument accepted")
	}
}

func TestLoadBenchmarkMetadataRejectsMissingAndPlaceholderValues(t *testing.T) {
	for _, test := range []struct {
		name     string
		hardware string
		limits   string
	}{
		{name: "missing"},
		{name: "hardware placeholder", hardware: benchmarkHardwarePlaceholder, limits: "none"},
		{name: "limits placeholder", hardware: "CPU, RAM, storage", limits: benchmarkResourceLimitsPlaceholder},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("BENCHMARK_HARDWARE", test.hardware)
			t.Setenv("BENCHMARK_RESOURCE_LIMITS", test.limits)
			if _, _, err := loadBenchmarkMetadata(); err == nil {
				t.Fatal("loadBenchmarkMetadata() accepted invalid values")
			}
		})
	}
	t.Setenv("BENCHMARK_HARDWARE", "CPU, 32 GB RAM, NVMe")
	t.Setenv("BENCHMARK_RESOURCE_LIMITS", "none")
	hardware, limits, err := loadBenchmarkMetadata()
	if err != nil || hardware != "CPU, 32 GB RAM, NVMe" || limits != "none" {
		t.Fatalf("loadBenchmarkMetadata() = %q, %q, %v", hardware, limits, err)
	}
}

func TestRenderReportIncludesRequiredSectionsAndEscapesCells(t *testing.T) {
	data, err := renderReport(reportFixture())
	if err != nil {
		t.Fatalf("renderReport() error = %v", err)
	}
	text := string(data)
	for _, fragment := range []string{
		"## Environment",
		"## Workload contract",
		"## Raw trials",
		"## Summary",
		"## Conclusion",
		"## Reproduction",
		"publishable: true",
		"threshold_met: true",
		"dirty\\|hardware<br>line",
		"make bench-copy",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("rendered report missing %q\n%s", fragment, text)
		}
	}
}

func TestGitIdentityReturnsRepositoryState(t *testing.T) {
	sha, _, err := gitIdentity(context.Background())
	if err != nil {
		t.Fatalf("gitIdentity() error = %v", err)
	}
	if len(sha) != 40 {
		t.Fatalf("Git SHA = %q", sha)
	}
}

func reportFixture() usecasecopybenchmark.Report {
	strategies := []usecasecopybenchmark.Strategy{
		usecasecopybenchmark.StrategySingleInsert,
		usecasecopybenchmark.StrategyBatchedInsert,
		usecasecopybenchmark.StrategyTextCopy,
		usecasecopybenchmark.StrategyBinaryCopy,
	}
	report := usecasecopybenchmark.Report{
		Environment: usecasecopybenchmark.Environment{
			GitSHA:         strings.Repeat("a", 40),
			GitDirty:       true,
			Hardware:       "dirty|hardware\nline",
			ResourceLimits: "none",
			GoVersion:      "go1.25",
			OSArch:         "windows/amd64",
			LogicalCPUs:    8,
			PostgreSQL:     "PostgreSQL 17",
			PostgreSQLSettings: []usecasecopybenchmark.Setting{
				{Name: "fsync", Value: "on"},
			},
			CapturedAt: time.Date(2026, time.September, 13, 1, 2, 3, 4, time.UTC),
		},
		Rows:         1_000_000,
		Repetitions:  3,
		Publishable:  true,
		ThresholdMet: true,
	}
	for _, strategy := range strategies {
		for run := 1; run <= 3; run++ {
			report.Trials = append(report.Trials, usecasecopybenchmark.Trial{
				Strategy:       strategy,
				Run:            run,
				Rows:           1_000_000,
				Elapsed:        time.Second,
				RowsPerSecond:  1_000_000,
				SourceChecksum: "abc",
				StoredChecksum: "abc",
				Eligible:       true,
			})
		}
		report.Results = append(report.Results, usecasecopybenchmark.StrategyResult{
			Strategy:      strategy,
			Rows:          1_000_000,
			MedianElapsed: time.Second,
			MinElapsed:    900 * time.Millisecond,
			MaxElapsed:    1100 * time.Millisecond,
			RowsPerSecond: 1_000_000,
			RelativeSpeed: 1,
			Checksum:      "abc",
		})
	}
	return report
}
