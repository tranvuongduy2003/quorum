package routing

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"quorum/internal/usecase/health"

	"github.com/go-chi/chi/v5"
)

type System struct {
	logger           *slog.Logger
	health           health.Service
	readinessTimeout time.Duration
}

type HealthResponse struct {
	Status string `json:"status" binding:"required" enums:"ok" example:"ok"`
} // @name HealthResponse

type ReadinessResponse struct {
	Status       string                     `json:"status" binding:"required" enums:"ready,not_ready" example:"ready"`
	Dependencies []DependencyStatusResponse `json:"dependencies" binding:"required"`
	CheckedAt    string                     `json:"checked_at" binding:"required" format:"date-time" example:"2026-08-27T18:20:31Z"`
} // @name ReadinessResponse

type DependencyStatusResponse struct {
	Name   string `json:"name" binding:"required" enums:"postgres,redis" example:"postgres"`
	Status string `json:"status" binding:"required" enums:"up,down" example:"up"`
} // @name DependencyStatus

func NewSystem(logger *slog.Logger, service health.Service, readinessTimeout time.Duration) System {
	return System{logger: logger, health: service, readinessTimeout: readinessTimeout}
}

func MountSystem(r chi.Router, handler System) {
	r.Get("/healthz", handler.Liveness)
	r.Get("/readyz", handler.Readiness)
}

// @ID          getLiveness
// @Summary     Liveness probe
// @Description Answers as soon as the process can serve HTTP. It checks no dependency, so it stays 200 while PostgreSQL or Redis are down. Orchestrators restart a container on this one.
// @Tags        health
// @Produce     json
// @Success     200 {object} HealthResponse
// @Header      all {string} X-Request-Id "correlation id, present on every response"
// @Router      /healthz [get]
func (h System) Liveness(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

// @ID          getReadiness
// @Summary     Readiness probe
// @Description Probes every dependency under one shared deadline and reports each by name. The 503 is a report rather than an error: its body is the same shape as the 200, not the error envelope, so a client parses one shape either way. Load balancers remove a pod from rotation on this one.
// @Tags        health
// @Produce     json
// @Success     200 {object} ReadinessResponse
// @Failure     503 {object} ReadinessResponse "one or more dependencies are down"
// @Header      all {string} X-Request-Id "correlation id, present on every response"
// @Router      /readyz [get]
func (h System) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.readinessTimeout)
	defer cancel()

	report := h.health.Readiness(ctx)

	dependencies := make([]DependencyStatusResponse, 0, len(report.Dependencies))
	for _, dependency := range report.Dependencies {
		status := "up"
		if !dependency.Healthy {
			status = "down"
			h.logger.Warn("dependency_unhealthy", "dependency", dependency.Name, "error", dependency.Err)
		}

		dependencies = append(dependencies, DependencyStatusResponse{Name: dependency.Name, Status: status})
	}

	status := "ready"
	code := http.StatusOK
	if !report.Ready {
		status = "not_ready"
		code = http.StatusServiceUnavailable
	}

	WriteJSON(w, code, ReadinessResponse{
		Status:       status,
		Dependencies: dependencies,
		CheckedAt:    report.CheckedAt.UTC().Format(time.RFC3339),
	})
}
