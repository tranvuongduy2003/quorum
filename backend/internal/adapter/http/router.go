package deliveryhttp

import (
	"log/slog"
	"net/http"
	"quorum/docs"
	"quorum/internal/adapter/http/routing"
	"quorum/internal/adapter/http/routing/api"
	"quorum/internal/adapter/http/routing/api/v1"
	"quorum/internal/usecase/diagnostics"
	"quorum/internal/usecase/health"
	"quorum/internal/usecase/ratelimit"
	"time"

	"github.com/go-chi/chi/v5"
)

type Options struct {
	Logger      *slog.Logger
	Health      health.Service
	Diagnostics diagnostics.Service
	Limiter     ratelimit.Store
	CORS        CORSOptions
	Timeouts    TimeoutOptions
	RateLimit   RateLimitOptions
	Features    FeatureOptions
}

type RateLimitOptions struct {
	Enabled           bool
	Requests          int
	Window            time.Duration
	FailOpen          bool
	TrustProxyHeaders bool
}

type CORSOptions struct {
	AllowedOrigins []string
}

type TimeoutOptions struct {
	Readiness time.Duration
	Request   time.Duration
}

type FeatureOptions struct {
	DebugRoutes bool
	Docs        bool
}

func NewRouter(opts Options) http.Handler {
	r := chi.NewRouter()
	routing.Configure(r, routing.Root{
		Middleware: []func(http.Handler) http.Handler{
			withRequestID,
			withAccessLog(opts.Logger),
			withRecovery(opts.Logger),
			withCORS(opts.CORS.AllowedOrigins),
			withRequestTimeout(opts.Timeouts.Request),
		},
	})

	routing.MountSystem(r, routing.NewSystem(opts.Logger, opts.Health, opts.Timeouts.Readiness))

	if opts.Features.Docs {
		routing.MountDocumentation(r, routing.NewDocumentation(docs.OpenAPI, docs.Scalar))
	}

	api.Mount(r, api.Router{
		V1: v1.Router{
			Middleware: []func(http.Handler) http.Handler{
				withRateLimit(opts.Limiter, opts.RateLimit, opts.Logger),
			},
			Diagnostics: v1.NewDiagnostics(opts.Logger, opts.Diagnostics),
			Debug:       v1.NewDebug(opts.Logger, opts.Features.DebugRoutes),
		},
	})

	return r
}
