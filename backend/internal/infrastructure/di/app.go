package di

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	redishealth "quorum/internal/adapter/cache/redis/health"
	redisratelimit "quorum/internal/adapter/cache/redis/ratelimit"
	deliveryhttp "quorum/internal/adapter/http"
	"quorum/internal/adapter/http/routing"
	postgresdiagnostics "quorum/internal/adapter/repository/postgres/diagnostics"
	postgreshealth "quorum/internal/adapter/repository/postgres/health"
	"quorum/internal/infrastructure/cache"
	"quorum/internal/infrastructure/config"
	"quorum/internal/infrastructure/db"
	"quorum/internal/usecase/diagnostics"
	"quorum/internal/usecase/health"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

type App struct {
	cfg    config.Config
	logger *slog.Logger
	pool   *pgxpool.Pool
	redis  *goredis.Client
	server *http.Server
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	pgPoolCtx, cancel := context.WithTimeout(ctx, cfg.StartupProbeTimeout)
	defer cancel()

	pgPool, err := db.NewPostgresPool(pgPoolCtx, cfg.Postgres)
	if err != nil {
		return nil, err
	}
	logger.Info("postgres_connected", "max_conns", cfg.Postgres.MaxConns, "min_conns", cfg.Postgres.MinConns)

	redisCtx, cancel := context.WithTimeout(ctx, cfg.StartupProbeTimeout)
	defer cancel()

	redisClient, err := cache.NewRedisClient(redisCtx, cfg.Redis)
	if err != nil {
		pgPool.Close()
		return nil, err
	}
	logger.Info("redis_connected")

	pgProbe := postgreshealth.NewProbe(pgPool)

	redisProbe := redishealth.NewProbe(redisClient)
	healthUseCase := health.NewService(time.Now, pgProbe, redisProbe)

	timeRepo := postgresdiagnostics.NewTimeRepository(pgPool)
	diagnosticsUseCase := diagnostics.NewService(timeRepo)

	rateLimiter := redisratelimit.NewRateLimiter(redisClient)

	server := &http.Server{
		Addr: cfg.Addr(),
		Handler: deliveryhttp.NewRouter(deliveryhttp.Options{
			Logger:      logger,
			Health:      healthUseCase,
			Diagnostics: diagnosticsUseCase,
			Limiter:     rateLimiter,
			RateLimit: deliveryhttp.RateLimitOptions{
				Enabled:           cfg.RateLimit.Enabled,
				Requests:          cfg.RateLimit.Requests,
				Window:            cfg.RateLimit.Window,
				FailOpen:          cfg.RateLimit.FailOpen,
				TrustProxyHeaders: cfg.RateLimit.TrustProxyHeaders,
			},
			CORS: deliveryhttp.CORSOptions{
				AllowedOrigins: cfg.CORS.AllowedOrigins,
			},
			Timeouts: deliveryhttp.TimeoutOptions{
				Readiness: cfg.StartupProbeTimeout,
				Request:   cfg.Server.RequestTimeout,
			},
			Features: deliveryhttp.FeatureOptions{
				DebugRoutes: cfg.Features.DebugRoutes,
				Docs:        cfg.Features.Docs,
			},
		}),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	app := &App{
		cfg:    cfg,
		logger: logger,
		pool:   pgPool,
		redis:  redisClient,
		server: server,
	}

	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	errorsChannel := make(chan error, 1)

	go func() {
		errorsChannel <- a.server.ListenAndServe()
	}()

	a.logger.Info("server_listening", "addr", a.cfg.Addr())

	if a.cfg.Features.Docs {
		baseURL := "http://localhost" + a.cfg.Addr()
		a.logger.Info(
			"docs_available",
			"reference", baseURL+routing.ReferencePath,
			"document", baseURL+routing.DocumentPath,
		)
	}

	select {
	case err := <-errorsChannel:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err
	case <-ctx.Done():
		a.logger.Info("shutdown_signal_received", "signal", context.Cause(ctx).Error())

		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.Server.ShutdownTimeout)
		defer cancel()

		startedAt := time.Now()

		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return err
		}

		a.logger.Info("server_stopped", "drain_ms", time.Since(startedAt).Milliseconds())

		return nil
	}
}

func (a *App) Close() {
	if a.redis != nil {
		if err := a.redis.Close(); err != nil {
			a.logger.Warn("redis_close_failed", "error", err)
		}

		a.redis = nil
	}

	if a.pool != nil {
		a.pool.Close()
		a.pool = nil
	}
}
