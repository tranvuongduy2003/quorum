package routing

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"quorum/internal/domain/message"
	"quorum/internal/domain/user"
	"quorum/internal/usecase/availability"
)

const (
	CodeValidationFailed   = "VALIDATION_FAILED"
	CodeInvalidJSON        = "INVALID_JSON"
	CodeNotFound           = "NOT_FOUND"
	CodeMethodNotAllowed   = "METHOD_NOT_ALLOWED"
	CodePayloadTooLarge    = "PAYLOAD_TOO_LARGE"
	CodeRateLimited        = "RATE_LIMITED"
	CodeRequestTimeout     = "REQUEST_TIMEOUT"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	CodeInternal           = "INTERNAL"
)

const (
	MessageValidationFailed   = "the request contains one or more invalid values"
	MessageInvalidJSON        = "the request body is not valid JSON"
	MessageNotFound           = "the requested resource does not exist"
	MessageMethodNotAllowed   = "this method is not allowed for this path"
	MessagePayloadTooLarge    = "the request body is too large"
	MessageRateLimited        = "too many requests, please slow down"
	MessageRequestTimeout     = "the request took too long to complete"
	MessageServiceUnavailable = "the service is temporarily unavailable, please retry"
	MessageInternal           = "an unexpected error occurred"
)

const (
	DetailRequired   = "REQUIRED"
	DetailTooLong    = "TOO_LONG"
	DetailOutOfRange = "OUT_OF_RANGE"
)

type contextKey int

const requestIDKey contextKey = 0

type APIError struct {
	Status  int
	Code    string
	Message string
}

type ErrorResponse struct {
	Error ErrorBody `json:"error" binding:"required"`
} // @name ErrorResponse

type ErrorBody struct {
	Code      string       `json:"code" binding:"required" enums:"INVALID_JSON,VALIDATION_FAILED,NOT_FOUND,METHOD_NOT_ALLOWED,PAYLOAD_TOO_LARGE,RATE_LIMITED,REQUEST_TIMEOUT,SERVICE_UNAVAILABLE,INTERNAL" example:"VALIDATION_FAILED"`
	Message   string       `json:"message" binding:"required" example:"the request contains one or more invalid values"`
	RequestID string       `json:"request_id" binding:"required" example:"9f1c0c2e-1f2a-4a9b-8f0e-2d6a3f7b1c44"`
	Details   []FieldError `json:"details,omitempty"`
} // @name ErrorBody

type FieldError struct {
	Field   string `json:"field" binding:"required" example:"message"`
	Code    string `json:"code" binding:"required" enums:"REQUIRED,TOO_LONG,OUT_OF_RANGE" example:"TOO_LONG"`
	Message string `json:"message" binding:"required" example:"at most 280 characters"`
} // @name FieldError

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestID(ctx context.Context) string {
	requestID, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return ""
	}

	return requestID
}

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details ...FieldError) {
	WriteJSON(w, status, ErrorResponse{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: RequestID(r.Context()),
			Details:   details,
		},
	})
}

func WriteAPIError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error, details ...FieldError) {
	requestID := RequestID(r.Context())

	if errors.Is(err, context.Canceled) {
		logger.Debug("client_disconnected", "request_id", requestID)
		return
	}

	apiErr := MapError(err)

	record := logger.With(
		"request_id", requestID,
		"code", apiErr.Code,
		"status", apiErr.Status,
		"error", err.Error(),
	)

	if apiErr.Status >= http.StatusInternalServerError {
		record.Error("request_failed")
	} else {
		record.Warn("request_failed")
	}

	WriteError(w, r, apiErr.Status, apiErr.Code, apiErr.Message, details...)
}

func MapError(err error) APIError {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return APIError{Status: http.StatusGatewayTimeout, Code: CodeRequestTimeout, Message: MessageRequestTimeout}
	case errors.Is(err, message.ErrEmpty),
		errors.Is(err, message.ErrTooLong),
		errors.Is(err, user.ErrInvalidID),
		errors.Is(err, user.ErrInvalidName),
		errors.Is(err, user.ErrInvalidEmail):
		return APIError{Status: http.StatusBadRequest, Code: CodeValidationFailed, Message: MessageValidationFailed}
	case errors.Is(err, user.ErrNotFound):
		return APIError{Status: http.StatusNotFound, Code: CodeNotFound, Message: MessageNotFound}
	case errors.Is(err, availability.ErrUnavailable):
		return APIError{Status: http.StatusServiceUnavailable, Code: CodeServiceUnavailable, Message: MessageServiceUnavailable}
	default:
		return APIError{Status: http.StatusInternalServerError, Code: CodeInternal, Message: MessageInternal}
	}
}
