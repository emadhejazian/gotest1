// Package config centralises all runtime configuration for the application.
//
// Values are sourced exclusively from environment variables so that the binary
// is environment-agnostic (12-factor app principle). Locally, a .env file
// populated by godotenv provides those variables; in production they are
// injected by the deployment platform (Kubernetes secrets, ECS task env, etc.).
package config

import (
	"fmt"
	"os"
)

// Config holds every setting the application needs to start up.
// Adding a new setting is a three-step process:
//  1. Add the field here.
//  2. Read and validate it in Load().
//  3. Document it in .env.example.
type Config struct {
	// DatabaseURL is the full PostgreSQL DSN, e.g.
	// "postgres://user:pass@host:5432/dbname?sslmode=disable"
	DatabaseURL string

	// Port is the TCP port the HTTP server binds to.
	Port string

	// Env controls runtime behaviour (e.g. log format, error verbosity).
	Env string
}

// Load reads environment variables, validates required ones, and returns a
// ready-to-use *Config.  It returns a descriptive error if any required
// variable is missing so the application fails fast at startup rather than
// at first use.
func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required but not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // sensible default so PORT is not strictly required
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	return &Config{
		DatabaseURL: dbURL,
		Port:        port,
		Env:         env,
	}, nil
}
