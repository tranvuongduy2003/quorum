package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	postgrescopybenchmark "quorum/internal/adapter/repository/postgres/copybenchmark"
	"quorum/internal/infrastructure/config"
	"quorum/internal/infrastructure/db"
	usecasecopybenchmark "quorum/internal/usecase/copybenchmark"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const postgresStartupTimeout = 5 * time.Second
const benchmarkHardwarePlaceholder = "replace-with-cpu-ram-and-storage"
const benchmarkResourceLimitsPlaceholder = "replace-with-container-cpu-and-memory-limits-or-none"

type options struct {
	rows        int64
	repetitions int
	output      string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseOptions(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if _, err := config.LoadEnvFile(); err != nil {
		fmt.Fprintf(stderr, "load environment: %v\n", err)
		return 1
	}
	hardware, resourceLimits, err := loadBenchmarkMetadata()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	postgresConfig, err := config.LoadPostgres()
	if err != nil {
		fmt.Fprintf(stderr, "load PostgreSQL configuration: %v\n", err)
		return 1
	}
	startupContext, cancel := context.WithTimeout(ctx, postgresStartupTimeout)
	pool, err := db.NewPostgresPool(startupContext, postgresConfig)
	cancel()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer pool.Close()

	runner := postgrescopybenchmark.NewRunner(pool)
	service := usecasecopybenchmark.NewService(runner)
	report, err := service.Run(ctx, opts.rows, opts.repetitions)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	gitSHA, gitDirty, err := gitIdentity(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "capture Git identity: %v\n", err)
		return 1
	}
	report.Environment.GitSHA = gitSHA
	report.Environment.GitDirty = gitDirty
	report.Environment.Hardware = hardware
	report.Environment.ResourceLimits = resourceLimits
	report.Environment.GoVersion = runtime.Version()
	report.Environment.OSArch = runtime.GOOS + "/" + runtime.GOARCH
	report.Environment.LogicalCPUs = runtime.NumCPU()
	report.Environment.CapturedAt = time.Now().UTC()

	data, err := renderReport(report)
	if err != nil {
		fmt.Fprintf(stderr, "render report: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(opts.output), 0o755); err != nil {
		fmt.Fprintf(stderr, "create report directory: %v\n", err)
		return 1
	}
	if err := os.WriteFile(opts.output, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "write report: %v\n", err)
		return 1
	}
	binaryRowsPerSecond := resultRowsPerSecond(report, usecasecopybenchmark.StrategyBinaryCopy)
	if !report.Publishable {
		fmt.Fprintf(stdout, "output=%s publishable=false threshold_met=%t binary_rows_per_second=%.0f\n", opts.output, report.ThresholdMet, binaryRowsPerSecond)
		return 0
	}
	if !report.ThresholdMet {
		fmt.Fprintf(stdout, "output=%s publishable=true threshold_met=false binary_rows_per_second=%.0f\n", opts.output, binaryRowsPerSecond)
		return 1
	}
	fmt.Fprintf(stdout, "output=%s publishable=true threshold_met=true binary_rows_per_second=%.0f\n", opts.output, binaryRowsPerSecond)
	return 0
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	opts := options{
		rows:        usecasecopybenchmark.PublishedRows,
		repetitions: 3,
		output:      "../docs/benchmarks/SPEC-003-copy-results.md",
	}
	flags := flag.NewFlagSet("quorum-bench-copy", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Int64Var(&opts.rows, "rows", opts.rows, "Rows written by every trial")
	flags.IntVar(&opts.repetitions, "repetitions", opts.repetitions, "Measured trials per strategy")
	flags.StringVar(&opts.output, "output", opts.output, "Markdown report output path")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(opts.output) == "" {
		return options{}, errors.New("output path must not be empty")
	}
	return opts, nil
}

func loadBenchmarkMetadata() (string, string, error) {
	hardware := strings.TrimSpace(os.Getenv("BENCHMARK_HARDWARE"))
	resourceLimits := strings.TrimSpace(os.Getenv("BENCHMARK_RESOURCE_LIMITS"))
	if hardware == "" || hardware == benchmarkHardwarePlaceholder {
		return "", "", errors.New("BENCHMARK_HARDWARE must describe the measured CPU, RAM, and storage")
	}
	if resourceLimits == "" || resourceLimits == benchmarkResourceLimitsPlaceholder {
		return "", "", errors.New("BENCHMARK_RESOURCE_LIMITS must describe Docker CPU and memory limits or state none")
	}
	return hardware, resourceLimits, nil
}

func gitIdentity(ctx context.Context) (string, bool, error) {
	shaOutput, err := exec.CommandContext(ctx, "git", "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return "", false, fmt.Errorf("git rev-parse HEAD: %w: %s", err, strings.TrimSpace(string(shaOutput)))
	}
	sha := strings.TrimSpace(string(shaOutput))
	if sha == "" {
		return "", false, errors.New("git rev-parse HEAD returned an empty SHA")
	}
	statusOutput, err := exec.CommandContext(ctx, "git", "status", "--porcelain", "--untracked-files=all").CombinedOutput()
	if err != nil {
		return "", false, fmt.Errorf("git status: %w: %s", err, strings.TrimSpace(string(statusOutput)))
	}
	return sha, strings.TrimSpace(string(statusOutput)) != "", nil
}

func renderReport(report usecasecopybenchmark.Report) ([]byte, error) {
	var buffer bytes.Buffer
	write := func(format string, args ...any) error {
		_, err := fmt.Fprintf(&buffer, format, args...)
		return err
	}
	if err := write("# SPEC-003 Binary COPY benchmark results\n\n"); err != nil {
		return nil, err
	}
	if err := write("## Environment\n\n| Field | Value |\n|---|---|\n"); err != nil {
		return nil, err
	}
	environmentRows := [][2]string{
		{"Captured UTC", report.Environment.CapturedAt.UTC().Format(time.RFC3339Nano)},
		{"Git SHA", report.Environment.GitSHA},
		{"Git dirty", strconv.FormatBool(report.Environment.GitDirty)},
		{"Hardware", report.Environment.Hardware},
		{"Resource limits", report.Environment.ResourceLimits},
		{"Go version", report.Environment.GoVersion},
		{"OS/architecture", report.Environment.OSArch},
		{"Logical CPUs", strconv.Itoa(report.Environment.LogicalCPUs)},
		{"PostgreSQL", report.Environment.PostgreSQL},
	}
	for _, row := range environmentRows {
		if err := write("| %s | %s |\n", markdownCell(row[0]), markdownCell(row[1])); err != nil {
			return nil, err
		}
	}
	for _, setting := range report.Environment.PostgreSQLSettings {
		if err := write("| PostgreSQL setting `%s` | %s |\n", markdownCell(setting.Name), markdownCell(setting.Value)); err != nil {
			return nil, err
		}
	}
	if err := write("\n## Workload contract\n\n"); err != nil {
		return nil, err
	}
	workload := []string{
		fmt.Sprintf("Rows per trial: %d", report.Rows),
		fmt.Sprintf("Measured trials per strategy: %d", report.Repetitions),
		"Dataset: `site=benchmark.example`; `id=i+1`; `post_id=i%100000+1`; `vote_type_id=2+i%2`; nullable `user_id`; UTC millisecond timestamps; nullable bounty every 1,000 rows; `source_offset=i*64`",
		"Transaction boundary: one transaction and one commit per complete trial",
		"Warm-up: 10,000 binary COPY rows before measured trials",
		"Timer boundary: begins immediately before transaction begin and ends immediately after commit; truncate, source generation checksum, and stored checksum scan are excluded",
	}
	for _, item := range workload {
		if err := write("- %s\n", item); err != nil {
			return nil, err
		}
	}
	if err := write("\n## Raw trials\n\n| Strategy | Run | Rows | Elapsed | Rows/sec | Source checksum | Stored checksum | Eligible |\n|---|---:|---:|---:|---:|---|---|---|\n"); err != nil {
		return nil, err
	}
	for _, trial := range report.Trials {
		if err := write("| %s | %d | %d | %s | %.0f | `%s` | `%s` | %t |\n", markdownCell(string(trial.Strategy)), trial.Run, trial.Rows, markdownCell(trial.Elapsed.String()), trial.RowsPerSecond, markdownCell(trial.SourceChecksum), markdownCell(trial.StoredChecksum), trial.Eligible); err != nil {
			return nil, err
		}
	}
	if err := write("\n## Summary\n\n| Strategy | Rows | Median elapsed | Min-max spread | Median rows/sec | Relative speed | Checksum |\n|---|---:|---:|---:|---:|---:|---|\n"); err != nil {
		return nil, err
	}
	for _, result := range report.Results {
		if err := write("| %s | %d | %s | %s-%s | %.0f | %.2fx | `%s` |\n", markdownCell(string(result.Strategy)), result.Rows, markdownCell(result.MedianElapsed.String()), markdownCell(result.MinElapsed.String()), markdownCell(result.MaxElapsed.String()), result.RowsPerSecond, result.RelativeSpeed, markdownCell(result.Checksum)); err != nil {
			return nil, err
		}
	}
	if err := write("\n## Conclusion\n\n"); err != nil {
		return nil, err
	}
	if err := write("- publishable: %t\n- binary threshold: %.0f rows/sec required; observed %.0f rows/sec; threshold_met: %t\n- known confounds: shared PostgreSQL host, local container and storage state, generated workload excludes XML parsing and decompression\n", report.Publishable, float64(usecasecopybenchmark.MinimumBinaryRowsPerSecond), resultRowsPerSecond(report, usecasecopybenchmark.StrategyBinaryCopy), report.ThresholdMet); err != nil {
		return nil, err
	}
	if err := write("\n## Reproduction\n\n```powershell\nmake up\n$env:BENCHMARK_HARDWARE = '<CPU model, RAM, storage>'\n$env:BENCHMARK_RESOURCE_LIMITS = '<Docker CPU/memory limits or none>'\nmake bench-copy\nmake verify\n```\n"); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func markdownCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\r", "<br>")
	return strings.ReplaceAll(value, "\n", "<br>")
}

func resultRowsPerSecond(report usecasecopybenchmark.Report, strategy usecasecopybenchmark.Strategy) float64 {
	for _, result := range report.Results {
		if result.Strategy == strategy {
			return result.RowsPerSecond
		}
	}
	return 0
}
