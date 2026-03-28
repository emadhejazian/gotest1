package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/example/gotest1/internal/db"
)

// UserHandler groups all HTTP handlers that operate on the User resource.
// It is constructed once in server/router.go and its methods are registered
// as chi route handlers.
type UserHandler struct {
	queries *db.Queries // data-access object produced by sqlc
	logger  *slog.Logger
}

// NewUserHandler is the constructor for UserHandler.
// Accepting dependencies via the constructor (dependency injection) rather
// than reading globals makes this handler straightforward to unit-test.
func NewUserHandler(queries *db.Queries, logger *slog.Logger) *UserHandler {
	return &UserHandler{
		queries: queries,
		logger:  logger,
	}
}

// ---------------------------------------------------------------------------
// GET /api/v1/users
// ---------------------------------------------------------------------------

// ListUsers returns all users as a JSON array.
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.queries.ListUsers(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "ListUsers query failed", "error", err)
		respondError(w, r, http.StatusInternalServerError, "could not fetch users")
		return
	}

	// Return an empty JSON array instead of null when there are no users.
	// This is friendlier for API consumers who range over the result.
	if users == nil {
		users = []db.User{}
	}

	respond(w, r, http.StatusOK, users)
}

// ---------------------------------------------------------------------------
// GET /api/v1/users/{id}
// ---------------------------------------------------------------------------

// GetUser returns a single user by ID.
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "id must be a positive integer")
		return
	}

	user, err := h.queries.GetUser(r.Context(), id)
	if err != nil {
		// pgx.ErrNoRows is the "not found" sentinel — map it to 404.
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, r, http.StatusNotFound, "user not found")
			return
		}
		h.logger.ErrorContext(r.Context(), "GetUser query failed", "id", id, "error", err)
		respondError(w, r, http.StatusInternalServerError, "could not fetch user")
		return
	}

	respond(w, r, http.StatusOK, user)
}

// ---------------------------------------------------------------------------
// POST /api/v1/users
// ---------------------------------------------------------------------------

// createUserRequest is the expected JSON body for the CreateUser endpoint.
// Separating the request shape from the db.CreateUserParams gives us the
// freedom to rename or add fields on either side independently.
type createUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CreateUser inserts a new user and returns the created resource.
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := decode(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Basic validation — in a larger project you would use a validation library.
	if req.Name == "" || req.Email == "" {
		respondError(w, r, http.StatusUnprocessableEntity, "name and email are required")
		return
	}

	user, err := h.queries.CreateUser(r.Context(), db.CreateUserParams{
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		// PostgreSQL error code 23505 = unique_violation (duplicate email).
		// In a real project you would use pgconn.PgError to inspect the code.
		h.logger.ErrorContext(r.Context(), "CreateUser query failed", "error", err)
		respondError(w, r, http.StatusInternalServerError, "could not create user")
		return
	}

	// 201 Created + the full resource so the client has the assigned ID.
	respond(w, r, http.StatusCreated, user)
}

// ---------------------------------------------------------------------------
// DELETE /api/v1/users/{id}
// ---------------------------------------------------------------------------

// DeleteUser removes a user by ID.
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "id must be a positive integer")
		return
	}

	if err := h.queries.DeleteUser(r.Context(), id); err != nil {
		h.logger.ErrorContext(r.Context(), "DeleteUser query failed", "id", id, "error", err)
		respondError(w, r, http.StatusInternalServerError, "could not delete user")
		return
	}

	// 204 No Content is the idiomatic response for a successful DELETE.
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseID extracts the {id} URL parameter set by chi and converts it to int64.
func parseID(r *http.Request) (int64, error) {
	raw := chi.URLParam(r, "id")
	return strconv.ParseInt(raw, 10, 64)
}
