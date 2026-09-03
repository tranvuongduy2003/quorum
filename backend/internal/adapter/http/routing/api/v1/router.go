package v1

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Router struct {
	Middleware  []func(http.Handler) http.Handler
	Diagnostics Diagnostics
	Debug       Debug
}

func Mount(r chi.Router, router Router) {
	r.Route("/v1", func(r chi.Router) {
		for _, middleware := range router.Middleware {
			r.Use(middleware)
		}

		r.Get("/ping", router.Diagnostics.Ping)
		r.Post("/echo", router.Diagnostics.Echo)
		r.Get("/db/time", router.Diagnostics.DatabaseTime)

		if router.Debug.enabled {
			r.Get("/debug/panic", router.Debug.Panic)
			r.Get("/debug/slow", router.Debug.Slow)
		}
	})
}
