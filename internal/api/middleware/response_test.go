package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/flagstonehq/flagstone/internal/storage"
)

func TestJSON(t *testing.T) {
	w := httptest.NewRecorder()

	JSON(w, http.StatusCreated, map[string]string{"status": "ok"})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]string
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "ok", body["status"])
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r = r.WithContext(WithRequestID(r.Context(), "req_002"))

	Error(w, r, http.StatusBadRequest, "BAD_REQUEST", "something went wrong")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "BAD_REQUEST", body.Error.Code)
	assert.Equal(t, "something went wrong", body.Error.Message)
	assert.Equal(t, "req_002", body.Error.RequestID)
}

func TestError_nilRequestID(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	Error(w, r, http.StatusInternalServerError, "ERR", "msg")

	var body ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Empty(t, body.Error.RequestID)
}

func TestErrorFromDomain_alreadyInitialized(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	ErrorFromDomain(w, r, storage.ErrAlreadyInitialized)

	assert.Equal(t, http.StatusConflict, w.Code)

	var body ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "ALREADY_INITIALIZED", body.Error.Code)
}

func TestErrorFromDomain_duplicateKey(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	ErrorFromDomain(w, r, storage.ErrDuplicateKey)

	assert.Equal(t, http.StatusConflict, w.Code)

	var body ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "DUPLICATE_KEY", body.Error.Code)
}

func TestErrorFromDomain_notFound(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	ErrorFromDomain(w, r, storage.ErrNotFound)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var body ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "NOT_FOUND", body.Error.Code)
}

func TestErrorFromDomain_default(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	ErrorFromDomain(w, r, errors.New("unexpected error"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var body ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "INTERNAL_ERROR", body.Error.Code)
}

func TestErrorFromDomain_nil(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	ErrorFromDomain(w, r, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}
