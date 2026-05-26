package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRecoverPanic_catchesPanic(t *testing.T) {
	handler := RecoverPanic(zap.NewNop())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("crash")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "INTERNAL_ERROR", body.Error.Code)
	assert.Equal(t, "An internal server error occurred.", body.Error.Message)
}

func TestRecoverPanic_passthrough(t *testing.T) {
	handler := RecoverPanic(zap.NewNop())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}
