package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
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

func TestRenderReportLabelsSmokeAsNotPublishable(t *testing.T) {
	report := reportFixture()
	report.Rows = 10_000
	report.Publishable = false
	report.ThresholdMet = false
	data, err := renderReport(report)
	if err != nil {
		t.Fatalf("renderReport() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "- publishable: false") || !strings.Contains(text, "threshold_met: false") {
		t.Fatalf("smoke publication labels missing:\n%s", text)
	}
}

func TestPublishedEvidenceMatchesMakeTargetAndREADME(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	makefile := readRepositoryFile(t, filepath.Join(repositoryRoot, "Makefile"))
	if !strings.Contains(makefile, "bench-copy:\n\tcd backend && go run ./cmd/benchcopy --rows 1000000 --repetitions 3 --output ../docs/benchmarks/SPEC-003-copy-results.md") {
		t.Fatal("Makefile bench-copy target does not match the publication contract")
	}

	readme := readRepositoryFile(t, filepath.Join(repositoryRoot, "README.md"))
	report := readRepositoryFile(t, filepath.Join(repositoryRoot, "docs", "benchmarks", "SPEC-003-copy-results.md"))
	readmeRow := tableRow(t, readme, "| Binary COPY |")
	reportRow := tableRow(t, report, "| binary COPY | 1000000 |")
	if !strings.Contains(readmeRow, "[SPEC-003 benchmark report](docs/benchmarks/SPEC-003-copy-results.md)") {
		t.Fatalf("README evidence link is missing: %s", readmeRow)
	}
	readmeFields := tableFields(readmeRow)
	reportFields := tableFields(reportRow)
	readmeThroughput := strings.ReplaceAll(strings.TrimSuffix(readmeFields[1], " rows/sec"), ",", "")
	if readmeThroughput != reportFields[4] {
		t.Fatalf("README throughput = %q, report throughput = %q", readmeThroughput, reportFields[4])
	}
	if strings.Count(section(report, "## Raw trials", "## Summary"), "| true |") != 12 {
		t.Fatal("published report does not contain twelve eligible raw trials")
	}
	if !strings.Contains(report, "- publishable: true") || !strings.Contains(report, "threshold_met: true") {
		t.Fatal("published report does not pass publication gates")
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

func readRepositoryFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func tableRow(t *testing.T, contents string, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(contents, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	t.Fatalf("table row with prefix %q not found", prefix)
	return ""
}

func tableFields(row string) []string {
	parts := strings.Split(strings.Trim(row, "|"), "|")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}

func section(contents string, start string, end string) string {
	startIndex := strings.Index(contents, start)
	if startIndex < 0 {
		return ""
	}
	contents = contents[startIndex+len(start):]
	endIndex := strings.Index(contents, end)
	if endIndex < 0 {
		return contents
	}
	return contents[:endIndex]
}
