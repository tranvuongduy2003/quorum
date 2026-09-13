package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"quorum/internal/infrastructure/ingestguard"
	"strings"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(run(ctx, os.Stdout, os.Stderr))
}

func run(ctx context.Context, stdout io.Writer, stderr io.Writer) int {
	violations, err := ingestguard.Check(ctx, ".", "./cmd/ingest")
	if err != nil {
		fmt.Fprintln(stderr, strings.Join(strings.Fields(err.Error()), " "))
		return 1
	}

	for _, violation := range violations {
		detail := strings.Join(strings.Fields(violation.Detail), " ")
		fmt.Fprintf(stderr, "violation kind=%s package=%s file=%q detail=%q\n", violation.Kind, violation.Package, violation.File, detail)
	}
	if len(violations) > 0 {
		return 1
	}

	fmt.Fprintln(stdout, "production_ingest_write_guard violations=0")
	return 0
}
