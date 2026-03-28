// Package server wires together the HTTP router, middleware stack, and handlers.
// It is the single place where route paths are defined, making it easy to see
// the full API surface at a glance.
package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/example/gotest1/internal/db"
	"github.com/example/gotest1/internal/handler"
)

// NewRouter builds and returns the root chi router.
// It accepts the shared dependencies (queries, logger) and passes them down
// to each handler via constructors — no globals, no init() magic.
func NewRouter(queries *db.Queries, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	// -----------------------------------------------------------------------
	// Global middleware
	// -----------------------------------------------------------------------
	// middleware.RequestID attaches a unique X-Request-Id header to every
	// request so individual requests can be correlated across log lines.
	r.Use(middleware.RequestID)

	// middleware.RealIP rewrites r.RemoteAddr using X-Forwarded-For or
	// X-Real-IP, which is essential when the API sits behind a load balancer.
	r.Use(middleware.RealIP)

	// middleware.Recoverer catches panics, logs them, and returns a 500 so
	// one bad request cannot crash the entire server.
	r.Use(middleware.Recoverer)

	// -----------------------------------------------------------------------
	// Routes
	// -----------------------------------------------------------------------
	userHandler := handler.NewUserHandler(queries, logger)

	// All application routes are versioned under /api/v1 so we can introduce
	// breaking changes under /api/v2 without removing old endpoints immediately.
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/users", func(r chi.Router) {
			r.Get("/", userHandler.ListUsers)        // GET  /api/v1/users
			r.Post("/", userHandler.CreateUser)      // POST /api/v1/users
			r.Get("/{id}", userHandler.GetUser)      // GET  /api/v1/users/{id}
			r.Delete("/{id}", userHandler.DeleteUser) // DELETE /api/v1/users/{id}
		})
	})

	// Health-check endpoint for load balancer / Kubernetes liveness probes.
	// It intentionally lives outside the versioned prefix.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	})

	return r
}
