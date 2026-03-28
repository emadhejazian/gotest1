# gotest1

A RESTful HTTP API for managing users, built with Go. Provides full CRUD operations backed by PostgreSQL, using [chi](https://github.com/go-chi/chi) for routing and [sqlc](https://sqlc.dev) for type-safe SQL.

---

## Table of Contents

- [Requirements](#requirements)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Database Setup](#database-setup)
- [Running the Server](#running-the-server)
- [API Reference](#api-reference)
- [Project Structure](#project-structure)
- [Development](#development)
- [Architecture](#architecture)

---

## Requirements

- Go 1.22+
- PostgreSQL 14+
- [sqlc](https://sqlc.dev/docs/overview/install) (only needed if modifying SQL queries)

---

## Quick Start

```bash
# 1. Clone and enter the project
git clone https://github.com/emadhejazian/gotest1
cd gotest1

# 2. Copy environment config
cp .env.example .env
# Edit .env and set DATABASE_URL to your PostgreSQL connection string

# 3. Apply the database migration
psql "$DATABASE_URL" -f db/migrations/001_create_users.sql

# 4. Run the server
go run ./cmd/api
# Server starts on http://localhost:8080
```

---

## Configuration

All configuration is read from environment variables. For local development, copy `.env.example` to `.env` — it is loaded automatically on startup and is gitignored.

| Variable       | Required | Default       | Description                                      |
|----------------|----------|---------------|--------------------------------------------------|
| `DATABASE_URL` | **Yes**  | —             | Full PostgreSQL DSN                              |
| `PORT`         | No       | `8080`        | TCP port the HTTP server binds to                |
| `APP_ENV`      | No       | `development` | Runtime environment: `development` / `staging` / `production` |

**Example `.env`:**

```dotenv
DATABASE_URL=postgres://user:password@localhost:5432/gotest1?sslmode=disable
PORT=8080
APP_ENV=development
```

In production, inject these as platform environment variables (Kubernetes secrets, ECS task env, etc.) — no `.env` file is needed or expected.

---

## Database Setup

### Schema

The `users` table is created by `db/migrations/001_create_users.sql`:

```sql
CREATE TABLE IF NOT EXISTS users (
    id         BIGSERIAL    PRIMARY KEY,
    name       TEXT         NOT NULL,
    email      TEXT         NOT NULL UNIQUE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```

### Apply the migration

```bash
psql "$DATABASE_URL" -f db/migrations/001_create_users.sql
```

> The server performs a connection ping on startup and exits immediately if the database is unreachable or misconfigured.

---

## Running the Server

```bash
# Development (uses .env)
go run ./cmd/api

# Production binary
go build -o api ./cmd/api
./api
```

**Startup log (JSON):**

```json
{"time":"...","level":"INFO","msg":"connected to database"}
{"time":"...","level":"INFO","msg":"server starting","addr":":8080","env":"development"}
```

### Graceful shutdown

The server listens for `SIGINT` (Ctrl+C) and `SIGTERM` (sent by Docker / Kubernetes). In-flight requests are given up to 10 seconds to complete before the process exits.

### Health check

```
GET /health
→ 200 OK  {"status":"ok"}
```

Suitable for load balancer liveness probes. Lives outside the versioned API prefix.

---

## API Reference

Base URL: `http://localhost:8080/api/v1`

All request and response bodies are JSON. Successful responses set `Content-Type: application/json`.

### User object

```json
{
  "id":         1,
  "name":       "Alice",
  "email":      "alice@example.com",
  "created_at": "2024-01-15T10:30:00Z"
}
```

---

### List users

```
GET /api/v1/users
```

Returns all users sorted by creation date, newest first.

**Response `200 OK`:**

```json
[
  {
    "id": 2,
    "name": "Bob",
    "email": "bob@example.com",
    "created_at": "2024-01-16T08:00:00Z"
  },
  {
    "id": 1,
    "name": "Alice",
    "email": "alice@example.com",
    "created_at": "2024-01-15T10:30:00Z"
  }
]
```

Returns `[]` (empty array) when no users exist.

**Example:**

```bash
curl http://localhost:8080/api/v1/users
```

---

### Get a user

```
GET /api/v1/users/{id}
```

| Parameter | Type  | Description     |
|-----------|-------|-----------------|
| `id`      | int64 | User ID (path)  |

**Response `200 OK`:**

```json
{
  "id": 1,
  "name": "Alice",
  "email": "alice@example.com",
  "created_at": "2024-01-15T10:30:00Z"
}
```

**Error responses:**

| Status | Condition              |
|--------|------------------------|
| `400`  | `id` is not an integer |
| `404`  | User not found         |

**Example:**

```bash
curl http://localhost:8080/api/v1/users/1
```

---

### Create a user

```
POST /api/v1/users
Content-Type: application/json
```

**Request body:**

```json
{
  "name":  "Alice",
  "email": "alice@example.com"
}
```

| Field   | Type   | Required | Description       |
|---------|--------|----------|-------------------|
| `name`  | string | Yes      | User's full name  |
| `email` | string | Yes      | Unique email address |

**Response `201 Created`:**

```json
{
  "id": 1,
  "name": "Alice",
  "email": "alice@example.com",
  "created_at": "2024-01-15T10:30:00Z"
}
```

**Error responses:**

| Status | Condition                                |
|--------|------------------------------------------|
| `400`  | Malformed JSON or unknown fields         |
| `422`  | `name` or `email` is missing/empty       |
| `500`  | Database error (e.g. duplicate email)    |

**Example:**

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com"}'
```

---

### Delete a user

```
DELETE /api/v1/users/{id}
```

| Parameter | Type  | Description     |
|-----------|-------|-----------------|
| `id`      | int64 | User ID (path)  |

**Response `204 No Content`** — empty body on success.

**Error responses:**

| Status | Condition              |
|--------|------------------------|
| `400`  | `id` is not an integer |
| `500`  | Database error         |

**Example:**

```bash
curl -X DELETE http://localhost:8080/api/v1/users/1
```

---

### Error response format

All error responses share the same shape:

```json
{
  "error": "description of what went wrong"
}
```

---

## Project Structure

```
.
├── cmd/
│   └── api/
│       └── main.go             # Entry point: config, wiring, server lifecycle
├── internal/
│   ├── config/
│   │   └── config.go           # Environment variable loading and validation
│   ├── db/
│   │   ├── db.go               # sqlc-generated DBTX interface
│   │   ├── models.go           # sqlc-generated User model
│   │   └── user.sql.go         # sqlc-generated query methods
│   ├── handler/
│   │   ├── user.go             # HTTP handlers for all user routes
│   │   └── respond.go          # JSON encode/decode helpers
│   └── server/
│       └── router.go           # chi router, middleware, route registration
├── db/
│   ├── migrations/
│   │   └── 001_create_users.sql  # Schema DDL — run once against your database
│   └── queries/
│       └── user.sql            # Hand-written SQL — input to sqlc
├── .env.example                # Environment variable template
├── go.mod
├── go.sum
└── sqlc.yaml                   # sqlc code generation config
```

> **Note:** Never edit files under `internal/db/` directly. They are auto-generated by sqlc from `db/queries/` and `db/migrations/`.

---

## Development

### Run tests

```bash
go test ./...
```

### Regenerate database code

Edit SQL in `db/queries/user.sql`, then regenerate:

```bash
sqlc generate
```

This rewrites `internal/db/` based on the queries and schema defined under `db/`.

### Middleware

The following middleware is applied globally to every request:

| Middleware          | Purpose                                                                 |
|---------------------|-------------------------------------------------------------------------|
| `RequestID`         | Attaches a unique `X-Request-Id` header for log correlation             |
| `RealIP`            | Rewrites `RemoteAddr` from `X-Forwarded-For` / `X-Real-IP` headers     |
| `Recoverer`         | Catches panics, logs them, and returns `500` instead of crashing        |

### Logging

Structured JSON logs are written to stdout via `log/slog`. Each log entry includes a timestamp, level, message, and any key-value attributes added by the handler.

```json
{"time":"2024-01-15T10:30:00Z","level":"ERROR","msg":"GetUser query failed","id":99,"error":"no rows in result set"}
```

For human-friendly output during local development, swap `slog.NewJSONHandler` for `slog.NewTextHandler` in `cmd/api/main.go`.

---

## Architecture

The project follows a layered, dependency-injection approach with no global state:

```
main.go
  └── loads config
  └── opens pgxpool (database connection pool)
  └── creates db.Queries (sqlc wrapper)
  └── builds chi router → registers handlers
  └── starts http.Server with graceful shutdown

server/router.go   — route definitions only, no business logic
handler/user.go    — HTTP concerns: parse request, call db, encode response
internal/db/       — data access layer, generated by sqlc from raw SQL
```

Key design decisions:

- **12-factor config** — all settings from environment variables, validated at startup.
- **sqlc for data access** — SQL is written by hand and type-safe Go code is generated; no ORM reflection overhead.
- **DBTX interface** — handlers receive a `*db.Queries` that wraps either a connection pool or a transaction, enabling transactional testing without changing handler code.
- **Thin `main.go`** — wiring only; makes it easy to add alternative entry points (CLI, worker) that reuse `internal/` packages.
- **Versioned routes** — all endpoints live under `/api/v1`, leaving room for `/api/v2` without removing old endpoints.
