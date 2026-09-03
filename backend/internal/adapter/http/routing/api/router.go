package api

import (
	"quorum/internal/adapter/http/routing/api/v1"

	"github.com/go-chi/chi/v5"
)

type Router struct {
	V1 v1.Router
}

func Mount(r chi.Router, api Router) {
	r.Route("/api", func(r chi.Router) {
		v1.Mount(r, api.V1)
	})
}
