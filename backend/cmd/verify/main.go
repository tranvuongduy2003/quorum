package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	adapterverification "quorum/internal/adapter/repository/postgres/verification"
	"quorum/internal/infrastructure/config"
	"quorum/internal/infrastructure/db"
	usecaseverification "quorum/internal/usecase/verification"
	"strings"
	"syscall"
	"time"
)

const defaultTimeout = 5 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	code := run(ctx, os.Stdout, os.Stderr)
	os.Exit(code)
}

func run(ctx context.Context, stdout io.Writer, stderr io.Writer) int {
	writeErr := func(err error) {
		normalized := strings.Join(strings.Fields(err.Error()), " ")
		fmt.Fprintln(stderr, normalized)
	}

	if _, err := config.LoadEnvFile(); err != nil {
		writeErr(err)
		return 1
	}

	pgConfig, err := config.LoadPostgres()
	if err != nil {
		writeErr(err)
		return 1
	}

	runCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	pool, err := db.NewPostgresPool(runCtx, pgConfig)
	if err != nil {
		writeErr(err)
		return 1
	}
	defer pool.Close()

	repo := adapterverification.NewRepository(pool)
	service := usecaseverification.NewService(repo)

	checks, err := service.Run(runCtx)

	for _, check := range checks {
		fmt.Fprintf(stdout, "check=%s violations=%d\n", check.Name, check.Violations)
	}

	if err != nil {
		writeErr(err)
		return 1
	}

	return 0
}
