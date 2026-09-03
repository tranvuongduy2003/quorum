package deliveryhttp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"runtime/debug"
	"slices"
	"time"

	response "quorum/internal/adapter/http/routing"

	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-Id"

const maxRequestIDLength = 128

const (
	corsAllowedMethods  = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	corsAllowedHeaders  = "Content-Type, X-Request-Id"
	corsExposedHeaders  = "X-Request-Id, X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset, Retry-After"
	corsPreflightMaxAge = "600"
)

var invalidRequestIDChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}

	w.wroteHeader = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	written, err := w.ResponseWriter.Write(b)
	w.bytes += written

	return written, err
}

func sanitiseRequestID(raw string) string {
	if raw == "" || len(raw) > maxRequestIDLength || invalidRequestIDChars.MatchString(raw) {
		return ""
	}

	return raw
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}

	return host
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := sanitiseRequestID(r.Header.Get(requestIDHeader))
		if requestID == "" {
			requestID = uuid.New().String()
		}

		w.Header().Set(requestIDHeader, requestID)

		ctx := response.WithRequestID(r.Context(), requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func withAccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()

			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			defer func() {
				logger.Info(
					"http_request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", recorder.status,
					"duration_ms", float64(time.Since(startedAt))/float64(time.Millisecond),
					"bytes", recorder.bytes,
					"request_id", response.RequestID(r.Context()),
					"remote_ip", remoteIP(r.RemoteAddr),
					"user_agent", r.UserAgent(),
				)
			}()

			next.ServeHTTP(recorder, r)
		})
	}
}

func withRecovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}

				if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(recovered)
				}

				logger.Error(
					"panic_recovered",
					"request_id", response.RequestID(r.Context()),
					"panic", fmt.Sprint(recovered),
					"stack", string(debug.Stack()),
				)

				response.WriteError(w, r, http.StatusInternalServerError, response.CodeInternal, response.MessageInternal)
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func withCORS(allowed []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Origin")

			origin := r.Header.Get("Origin")
			if origin == "" || !slices.Contains(allowed, origin) {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Expose-Headers", corsExposedHeaders)

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
				w.Header().Set("Access-Control-Max-Age", corsPreflightMaxAge)
				w.WriteHeader(http.StatusNoContent)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func withRequestTimeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if d <= 0 {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
