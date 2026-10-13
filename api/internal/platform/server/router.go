// Package server builds the HTTP router and server and runs it with graceful
// shutdown.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"agentic-rag/api/internal/platform/health"
)

// Middleware is a standard net/http middleware.
type Middleware = func(http.Handler) http.Handler

// Options configures NewRouter.
type Options struct {
	// Middlewares are applied, in order, to every route. This is the single
	// attachment point for request-id, access log, recover and CORS (M0-T05).
	// chi requires middlewares to be registered before routes, which is why
	// they are passed in here instead of through a later Use call.
	Middlewares []Middleware
}

// NewRouter returns the root router with the probe routes registered.
func NewRouter(opts Options, probes *health.Checker) *chi.Mux {
	r := chi.NewRouter()
	r.Use(opts.Middlewares...)

	r.Get("/healthz", probes.Healthz)
	r.Get("/readyz", probes.Readyz)
	return r
}
