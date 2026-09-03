package deliveryhttp

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"

	response "quorum/internal/adapter/http/routing"
	"quorum/internal/usecase/ratelimit"
)

const forwardedForHeader = "X-Forwarded-For"

func withRateLimit(store ratelimit.Store, opts RateLimitOptions, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if !opts.Enabled {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			budget, err := store.Take(r.Context(), clientKey(r, opts.TrustProxyHeaders), opts.Requests, opts.Window)
			if err != nil {
				failMode := "closed"
				if opts.FailOpen {
					failMode = "open"
				}

				logger.Warn(
					"rate_limiter_unavailable",
					"request_id", response.RequestID(r.Context()),
					"error", err.Error(),
					"fail_mode", failMode,
				)

				if opts.FailOpen {
					next.ServeHTTP(w, r)
					return
				}

				response.WriteError(w, r, http.StatusServiceUnavailable, response.CodeServiceUnavailable, response.MessageServiceUnavailable)

				return
			}

			resetSeconds := int(math.Ceil(budget.ResetAfter.Seconds()))

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(budget.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(budget.Remaining()))
			w.Header().Set("X-RateLimit-Reset", strconv.Itoa(resetSeconds))

			if budget.Allowed() {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Retry-After", strconv.Itoa(max(1, resetSeconds)))
			response.WriteError(w, r, http.StatusTooManyRequests, response.CodeRateLimited, response.MessageRateLimited)
		})
	}
}

func clientKey(r *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		forwarded := strings.TrimSpace(strings.Split(r.Header.Get(forwardedForHeader), ",")[0])
		if forwarded != "" {
			return forwarded
		}
	}

	return remoteIP(r.RemoteAddr)
}
