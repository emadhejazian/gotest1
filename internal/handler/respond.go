// Package handler contains HTTP handlers (controllers) for the API.
//
// Each handler struct owns a set of related routes and holds exactly the
// dependencies it needs — no global state.  This makes handlers easy to
// test by swapping real dependencies for fakes.
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// respond encodes v as JSON and writes it to w with the given status code.
// It is an unexported helper shared by all handlers in this package.
func respond(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		// At this point the header is already sent, so we can only log.
		slog.ErrorContext(r.Context(), "failed to encode response", "error", err)
	}
}

// respondError writes a JSON error body: {"error": "message"}.
func respondError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	respond(w, r, status, map[string]string{"error": msg})
}

// decode reads a JSON body from r into dst and returns an error if the body
// is malformed or the Content-Type is wrong.
func decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // reject typos in the request body
	return dec.Decode(dst)
}
