package v1

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	response "quorum/internal/adapter/http/routing"
	domainmessage "quorum/internal/domain/message"
	"quorum/internal/usecase/diagnostics"
)

const maxRequestBodyBytes = 65536

type Diagnostics struct {
	logger  *slog.Logger
	service diagnostics.Service
}

type PingResponse struct {
	Message string `json:"message" binding:"required" example:"pong"`
} // @name PingResponse

type DatabaseTimeResponse struct {
	DatabaseTime string `json:"database_time" binding:"required" format:"date-time" example:"2026-08-27T18:20:31Z"`
} // @name DatabaseTimeResponse

type EchoRequest struct {
	Message string `json:"message" binding:"required" minLength:"1" maxLength:"280" example:"hello"`
} // @name EchoRequest

type EchoResponse struct {
	Message string `json:"message" binding:"required" example:"hello"`
	Length  int    `json:"length" binding:"required" example:"5"`
} // @name EchoResponse

func NewDiagnostics(logger *slog.Logger, service diagnostics.Service) Diagnostics {
	return Diagnostics{logger: logger, service: service}
}

// @ID          getPing
// @Summary     Ping the API
// @Description Cheapest versioned route. It touches no dependency, so it confirms the router, the middleware chain and the rate-limit budget and nothing else.
// @Tags        diagnostics
// @Produce     json
// @Success     200 {object} PingResponse
// @Failure     429 {object} response.ErrorResponse "the client has spent its budget for the current window"
// @Failure     503 {object} response.ErrorResponse "the rate limiter is unavailable and configured to fail closed"
// @Failure     504 {object} response.ErrorResponse
// @Failure     500 {object} response.ErrorResponse
// @Header      all {string} X-Request-Id "correlation id, present on every response"
// @Header      all {integer} X-RateLimit-Limit "requests allowed per window"
// @Header      all {integer} X-RateLimit-Remaining "requests left in the current window"
// @Header      all {integer} X-RateLimit-Reset "seconds until the window resets"
// @Header      429 {integer} Retry-After "seconds to wait before retrying"
// @Router      /api/v1/ping [get]
func (h Diagnostics) Ping(w http.ResponseWriter, r *http.Request) {
	response.WriteJSON(w, http.StatusOK, PingResponse{Message: "pong"})
}

// @ID          echoMessage
// @Summary     Echo a message back
// @Description Validates the message as a domain value object and returns it with its length in runes. The 400 covers two codes: INVALID_JSON for a body that does not parse, and VALIDATION_FAILED for one that parses and breaks a rule, with the offending field named in details.
// @Tags        diagnostics
// @Accept      json
// @Produce     json
// @Param       body body EchoRequest true "the message to echo"
// @Success     200 {object} EchoResponse
// @Failure     400 {object} response.ErrorResponse "INVALID_JSON or VALIDATION_FAILED"
// @Failure     413 {object} response.ErrorResponse "the body is larger than 65536 bytes"
// @Failure     429 {object} response.ErrorResponse "the client has spent its budget for the current window"
// @Failure     503 {object} response.ErrorResponse "the rate limiter is unavailable and configured to fail closed"
// @Failure     504 {object} response.ErrorResponse
// @Failure     500 {object} response.ErrorResponse
// @Header      all {string} X-Request-Id "correlation id, present on every response"
// @Header      all {integer} X-RateLimit-Limit "requests allowed per window"
// @Header      all {integer} X-RateLimit-Remaining "requests left in the current window"
// @Header      all {integer} X-RateLimit-Reset "seconds until the window resets"
// @Header      429 {integer} Retry-After "seconds to wait before retrying"
// @Router      /api/v1/echo [post]
func (h Diagnostics) Echo(w http.ResponseWriter, r *http.Request) {
	var input EchoRequest

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)).Decode(&input); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			response.WriteError(w, r, http.StatusRequestEntityTooLarge, response.CodePayloadTooLarge, response.MessagePayloadTooLarge)
			return
		}

		response.WriteError(w, r, http.StatusBadRequest, response.CodeInvalidJSON, response.MessageInvalidJSON)
		return
	}

	message, err := domainmessage.NewMessage(input.Message)
	if err != nil {
		detail := response.FieldError{Field: "message", Code: response.DetailRequired, Message: "a non-empty message is required"}
		if errors.Is(err, domainmessage.ErrTooLong) {
			detail = response.FieldError{
				Field:   "message",
				Code:    response.DetailTooLong,
				Message: "at most " + strconv.Itoa(domainmessage.MaxMessageLength) + " characters",
			}
		}

		response.WriteAPIError(w, r, h.logger, err, detail)
		return
	}

	response.WriteJSON(w, http.StatusOK, EchoResponse{Message: message.String(), Length: message.Length()})
}

// @ID          getDatabaseTime
// @Summary     Read the database clock
// @Description Runs one query against PostgreSQL and returns its current time, which proves the connection pool works end to end. The 503 carries no detail about the cause; the request id is how to find it in the logs.
// @Tags        diagnostics
// @Produce     json
// @Success     200 {object} DatabaseTimeResponse
// @Failure     429 {object} response.ErrorResponse "the client has spent its budget for the current window"
// @Failure     503 {object} response.ErrorResponse "PostgreSQL is unreachable, or the rate limiter is unavailable and configured to fail closed"
// @Failure     504 {object} response.ErrorResponse
// @Failure     500 {object} response.ErrorResponse
// @Header      all {string} X-Request-Id "correlation id, present on every response"
// @Header      all {integer} X-RateLimit-Limit "requests allowed per window"
// @Header      all {integer} X-RateLimit-Remaining "requests left in the current window"
// @Header      all {integer} X-RateLimit-Reset "seconds until the window resets"
// @Header      429 {integer} Retry-After "seconds to wait before retrying"
// @Router      /api/v1/db/time [get]
func (h Diagnostics) DatabaseTime(w http.ResponseWriter, r *http.Request) {
	now, err := h.service.DatabaseTime(r.Context())
	if err != nil {
		response.WriteAPIError(w, r, h.logger, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, DatabaseTimeResponse{DatabaseTime: now.UTC().Format(time.RFC3339)})
}
