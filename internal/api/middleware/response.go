package middleware

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/thomas-vilte/flagstone/internal/storage"
)

// ErrorResponse is the standard error envelope returned on non-2xx responses.
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// APIError represents a single structured error.
type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// Error writes a structured JSON error response, extracting the request ID
// from the request context.
func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	reqID := RequestIDFromContext(r.Context())
	JSON(w, status, ErrorResponse{
		Error: APIError{
			Code:      code,
			Message:   message,
			RequestID: reqID,
		},
	})
}

// ErrorFromDomain maps known domain errors to HTTP status codes and error codes.
func ErrorFromDomain(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	switch {
	case errors.Is(err, storage.ErrAlreadyInitialized):
		Error(w, r, http.StatusConflict, "ALREADY_INITIALIZED", "This instance is already set up.")
	case errors.Is(err, storage.ErrDuplicateKey):
		Error(w, r, http.StatusConflict, "DUPLICATE_KEY", "A resource with the given value already exists.")
	case errors.Is(err, storage.ErrNotFound):
		Error(w, r, http.StatusNotFound, "NOT_FOUND", "The requested resource was not found.")
	default:
		Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
	}
}
