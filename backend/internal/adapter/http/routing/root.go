package routing

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Root struct {
	Middleware []func(http.Handler) http.Handler
}

func Configure(r chi.Router, root Root) {
	r.MethodNotAllowed(func(w http.ResponseWriter, request *http.Request) {
		WriteError(w, request, http.StatusMethodNotAllowed, CodeMethodNotAllowed, MessageMethodNotAllowed)
	})
	r.NotFound(func(w http.ResponseWriter, request *http.Request) {
		WriteError(w, request, http.StatusNotFound, CodeNotFound, MessageNotFound)
	})

	for _, middleware := range root.Middleware {
		r.Use(middleware)
	}
}
