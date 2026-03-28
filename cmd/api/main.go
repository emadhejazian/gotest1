// Command api is the entry point for the REST API server.
//
// The main function is intentionally thin: its only jobs are to load
// configuration, wire up shared dependencies, and start (and gracefully
// stop) the HTTP server.  All business logic belongs in internal/.
//
// This separation keeps main.go easy to read and makes it straightforward
// to add alternative entry points later (e.g. a CLI tool or a worker process)
// that reuse the same internal packages without touching the HTTP layer.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/example/gotest1/internal/config"
	"github.com/example/gotest1/internal/db"
	"github.com/example/gotest1/internal/server"
)

func main() {
	// -------------------------------------------------------------------------
	// 1. Load .env (development only)
	// -------------------------------------------------------------------------
	// godotenv.Load is a no-op if .env does not exist, which is the expected
	// behaviour in production where env vars are injected by the platform.
	if err := godotenv.Load(); err != nil {
		slog.Warn("no .env file found; relying on environment variables")
	}

	// -------------------------------------------------------------------------
	// 2. Parse configuration
	// -------------------------------------------------------------------------
	// Validate all required env vars up front so we fail fast with a clear
	// message rather than crashing halfway through startup.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	// -------------------------------------------------------------------------
	// 3. Configure structured logging
	// -------------------------------------------------------------------------
	// JSON handler is machine-readable and works well with log aggregators
	// (Datadog, Loki, CloudWatch, etc.).  For local development you may prefer
	// slog.NewTextHandler for human-friendly output.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger) // handlers can call slog.InfoContext without a ref

	// -------------------------------------------------------------------------
	// 4. Connect to PostgreSQL
	// -------------------------------------------------------------------------
	// pgxpool.Pool manages a pool of connections and is safe for concurrent use.
	// A *pgxpool.Pool satisfies the db.DBTX interface, so it can be passed
	// directly to db.New().
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to create connection pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Ping verifies the database is reachable before we accept any HTTP traffic.
	if err := pool.Ping(context.Background()); err != nil {
		logger.Error("database ping failed", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to database")

	// -------------------------------------------------------------------------
	// 5. Build shared dependencies
	// -------------------------------------------------------------------------
	// db.New wraps the pool in the sqlc-generated Queries struct.
	// Every handler receives the same *Queries instance — it is stateless and
	// safe for concurrent use.
	queries := db.New(pool)

	// -------------------------------------------------------------------------
	// 6. Build the HTTP router
	// -------------------------------------------------------------------------
	router := server.NewRouter(queries, logger)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second, // time to read the full request body
		WriteTimeout: 10 * time.Second, // time to write the full response
		IdleTimeout:  60 * time.Second, // keep-alive connection idle time
	}

	// -------------------------------------------------------------------------
	// 7. Start the server (non-blocking)
	// -------------------------------------------------------------------------
	// ListenAndServe blocks, so we run it in a goroutine and let the main
	// goroutine block on the OS signal channel below.
	go func() {
		logger.Info("server starting", "addr", httpServer.Addr, "env", cfg.Env)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// -------------------------------------------------------------------------
	// 8. Graceful shutdown
	// -------------------------------------------------------------------------
	// Block until we receive SIGINT (Ctrl+C) or SIGTERM (sent by Kubernetes,
	// systemd, Docker, etc. when the process should exit).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received; draining connections...")

	// Give in-flight requests up to 10 seconds to complete before forcing exit.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped cleanly")
}
