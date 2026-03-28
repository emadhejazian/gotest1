# CLAUDE.md

## Project Overview

A RESTful HTTP API for managing users, backed by PostgreSQL. CRUD operations on a `users` resource using clean architecture and sqlc for type-safe SQL.

## Tech Stack

- **Router**: `github.com/go-chi/chi/v5`
- **Database driver**: `github.com/jackc/pgx/v5`
- **Code generation**: sqlc v1.26.0
- **Config**: `github.com/joho/godotenv` (dev only)
- **Logging**: `log/slog` (structured JSON)

## Project Structure

```
cmd/api/main.go           # Entry point: wiring, startup, shutdown
internal/
  config/config.go        # Env var config loading
  server/router.go        # chi router and route registration
  handler/user.go         # HTTP handlers (user CRUD)
  handler/respond.go      # Response encoding helpers
  db/                     # sqlc-generated code (do not edit manually)
db/
  migrations/             # SQL schema DDL
  queries/                # Hand-written SQL queries (input to sqlc)
sqlc.yaml                 # sqlc code generation config
```

## Commands

```bash
# Run
go run ./cmd/api

# Build
go build -o api ./cmd/api

# Regenerate sqlc code (after editing db/queries/ or db/migrations/)
sqlc generate

# Tests
go test ./...
```

## Environment Variables

| Variable       | Required | Default       | Description                   |
|----------------|----------|---------------|-------------------------------|
| `DATABASE_URL` | Yes      | —             | PostgreSQL DSN                |
| `PORT`         | No       | `8080`        | HTTP server port              |
| `APP_ENV`      | No       | `development` | `development|staging|production` |

Copy `.env.example` to `.env` for local development. `.env` is gitignored.

## Database

- Run migrations manually before starting: `db/migrations/001_create_users.sql`
- No auto-migration tool configured (golang-migrate suggested for production)
- Edit SQL in `db/queries/user.sql`, then run `sqlc generate` — never edit `internal/db/` directly

## Architecture Notes

- Thin `main.go`: only startup/shutdown wiring
- Dependency injection via constructors, no globals
- `DBTX` interface in `internal/db/db.go` supports both pool and transaction contexts
