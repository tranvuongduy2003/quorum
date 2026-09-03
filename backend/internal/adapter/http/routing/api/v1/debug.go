package v1

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	response "quorum/internal/adapter/http/routing"
)

const (
	defaultSlowMilliseconds = 2000
	maxSlowMilliseconds     = 30000
)

type Debug struct {
	logger  *slog.Logger
	enabled bool
}

func NewDebug(logger *slog.Logger, enabled bool) Debug {
	return Debug{logger: logger, enabled: enabled}
}

// @ID          triggerPanic
// @Summary     Panic on purpose
// @Description Registered only when DEBUG_ROUTES_ENABLED is true, so it is absent in most environments. It exists to exercise the recovery middleware, and it always returns 500 in the standard envelope.
// @Tags        debug
// @Produce     json
// @Failure     500 {object} response.ErrorResponse "always, by design"
// @Failure     429 {object} response.ErrorResponse "the client has spent its budget for the current window"
// @Failure     503 {object} response.ErrorResponse "the rate limiter is unavailable and configured to fail closed"
// @Header      all {string} X-Request-Id "correlation id, present on every response"
// @Header      all {integer} X-RateLimit-Limit "requests allowed per window"
// @Header      all {integer} X-RateLimit-Remaining "requests left in the current window"
// @Header      all {integer} X-RateLimit-Reset "seconds until the window resets"
// @Header      429 {integer} Retry-After "seconds to wait before retrying"
// @Router      /api/v1/debug/panic [get]
func (h Debug) Panic(w http.ResponseWriter, r *http.Request) {
	panic("debug route panicked on purpose")
}

// @ID          getSlowResponse
// @Summary     Sleep for a bounded time
// @Description Registered only when DEBUG_ROUTES_ENABLED is true, so it is absent in most environments. It exists to exercise the per-request timeout: ask for longer than REQUEST_TIMEOUT and the answer is a 504 rather than a slow 200.
// @Tags        debug
// @Produce     json
// @Param       ms query int false "milliseconds to sleep" minimum(0) maximum(30000) default(2000)
// @Success     200 {object} PingResponse
// @Failure     400 {object} response.ErrorResponse "ms is not a whole number of milliseconds between 0 and 30000"
// @Failure     429 {object} response.ErrorResponse "the client has spent its budget for the current window"
// @Failure     503 {object} response.ErrorResponse "the rate limiter is unavailable and configured to fail closed"
// @Failure     504 {object} response.ErrorResponse "the sleep outlasted the per-request timeout"
// @Failure     500 {object} response.ErrorResponse
// @Header      all {string} X-Request-Id "correlation id, present on every response"
// @Header      all {integer} X-RateLimit-Limit "requests allowed per window"
// @Header      all {integer} X-RateLimit-Remaining "requests left in the current window"
// @Header      all {integer} X-RateLimit-Reset "seconds until the window resets"
// @Header      429 {integer} Retry-After "seconds to wait before retrying"
// @Router      /api/v1/debug/slow [get]
func (h Debug) Slow(w http.ResponseWriter, r *http.Request) {
	wait := defaultSlowMilliseconds

	if raw := r.URL.Query().Get("ms"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 || parsed > maxSlowMilliseconds {
			response.WriteError(w, r, http.StatusBadRequest, response.CodeValidationFailed, response.MessageValidationFailed, response.FieldError{
				Field:   "ms",
				Code:    response.DetailOutOfRange,
				Message: "must be a whole number of milliseconds between 0 and " + strconv.Itoa(maxSlowMilliseconds),
			})
			return
		}

		wait = parsed
	}

	timer := time.NewTimer(time.Duration(wait) * time.Millisecond)
	defer timer.Stop()

	select {
	case <-timer.C:
		response.WriteJSON(w, http.StatusOK, PingResponse{Message: "slept"})
	case <-r.Context().Done():
		response.WriteAPIError(w, r, h.logger, r.Context().Err())
	}
}
