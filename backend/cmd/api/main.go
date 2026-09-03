package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"quorum/internal/infrastructure/config"
	"quorum/internal/infrastructure/di"
	"quorum/internal/infrastructure/logging"
	"syscall"
)

func main() {
	path, err := config.LoadEnvFile()
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	logger, err := logging.New(cfg.Log.Level, cfg.Log.Format, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}

	if path == "" {
		logger.Debug("env_file_absent")
	} else {
		logger.Info("env_file_loaded", "path", path)
	}

	logger.Info("config_loaded", "environment", cfg.Environment, "port", cfg.Server.Port, "log_level", cfg.Log.Level)

	if err := run(cfg, logger); err != nil {
		logger.Error("startup_failed", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := di.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer app.Close()

	if err := app.Run(ctx); err != nil {
		return err
	}

	logger.Info("shutdown_complete")

	return nil
}
