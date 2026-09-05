package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"quorum/internal/adapter/source/stackexchange"
	domainingestion "quorum/internal/domain/ingestion"
	usecaseingestion "quorum/internal/usecase/ingestion"
	"syscall"
)

type options struct {
	siteRaw, archivePath, tablesRaw, watermarkPatternsPath string
	dryRun                                                 bool
	rejectThresholdPercent                                 float64
	maxRecordBytes                                         int
}

func (o options) command() (usecaseingestion.Command, error) {
	site, err := domainingestion.ParseSite(o.siteRaw)
	if err != nil {
		return usecaseingestion.Command{}, err
	}

	tables, err := domainingestion.ParseTables(o.tablesRaw)
	if err != nil {
		return usecaseingestion.Command{}, err
	}

	rawPath := resolveArchivePath(site, o.archivePath)

	return usecaseingestion.NewCommand(site, rawPath, tables, o.dryRun, o.rejectThresholdPercent, o.maxRecordBytes, o.watermarkPatternsPath)
}

func resolveArchivePath(site domainingestion.Site, explicit string) string {
	if explicit == "" {
		return filepath.Join("data", fmt.Sprintf("%s.7z", site))
	}

	return explicit
}

func locateOptionError(err error) usecaseingestion.RunError {
	var tableErr domainingestion.UnsupportedTableError
	if errors.As(err, &tableErr) {
		return usecaseingestion.RunError{
			Table:  tableErr.Value,
			Offset: 0,
			Err:    err,
		}
	}

	return usecaseingestion.RunError{
		Table:  "request",
		Offset: 0,
		Err:    err,
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	opts, err := parseOptions(args, stderr)

	if errors.Is(err, flag.ErrHelp) {
		return 0
	}

	if err != nil {
		return 2
	}

	cmd, err := opts.command()
	if err != nil {
		runErr := locateOptionError(err)
		fmt.Fprintln(stderr, runErr)
		return 1
	}

	factory := stackexchange.NewFactory()
	service := usecaseingestion.NewService(factory)

	summary, err := service.Run(ctx, cmd)
	if err != nil {
		if len(summary.Tables) > 0 {
			usecaseingestion.PrintSummary(stdout, summary)
		}
		fmt.Fprintln(stderr, err)
		return 1
	}

	usecaseingestion.PrintSummary(stdout, summary)
	return 0
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	fs, opts := newFlagSet(stderr)
	err := fs.Parse(args)
	if err != nil {
		return options{}, err
	}

	return *opts, nil
}

func newFlagSet(stderr io.Writer) (*flag.FlagSet, *options) {
	var opts options

	flags := flag.NewFlagSet("quorum-ingest", flag.ContinueOnError)
	flags.SetOutput(stderr)

	opts.siteRaw = ""
	flags.StringVar(&opts.siteRaw, "site", opts.siteRaw, "Stack Exchange site identifier, such as stackoverflow.com")

	opts.archivePath = ""
	flags.StringVar(&opts.archivePath, "archive", opts.archivePath, "Explicit .7z path; empty derives data/<site>.7z")

	opts.tablesRaw = "posts,users,comments,votes,badges,tags,post_links,post_history"
	flags.StringVar(&opts.tablesRaw, "tables", opts.tablesRaw, "Comma-separated canonical table subset")

	opts.dryRun = false
	flags.BoolVar(&opts.dryRun, "dry-run", opts.dryRun, "Inspect and report without persisting quarantine records")

	opts.rejectThresholdPercent = 0.5
	flags.Float64Var(&opts.rejectThresholdPercent, "reject-threshold", opts.rejectThresholdPercent, "Maximum rejected percentage allowed per table")

	opts.watermarkPatternsPath = ""
	flags.StringVar(&opts.watermarkPatternsPath, "watermark-patterns", opts.watermarkPatternsPath, "Optional UTF-8 file with one literal pattern per line")

	opts.maxRecordBytes = 8388608
	flags.IntVar(&opts.maxRecordBytes, "max-record-bytes", opts.maxRecordBytes, "Maximum decompressed bytes allowed for one source row")

	flags.Usage = func() {
		writeUsage(stderr, flags)
	}

	return flags, &opts
}

func writeUsage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintln(w, "Usage: quorum-ingest [flags]")

	fs.VisitAll(func(f *flag.Flag) {
		defaultValue := f.DefValue
		if defaultValue == "" {
			defaultValue = `""`
		}
		fmt.Fprintf(w, "--%s  default=%s  %s\n", f.Name, defaultValue, f.Usage)
	})
}
