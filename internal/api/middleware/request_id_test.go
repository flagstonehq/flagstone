package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestID_setsHeader(t *testing.T) {
	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	id := rec.Header().Get("X-Request-ID")
	assert.Contains(t, id, "req_")
	assert.Len(t, id, 20) // "req_" + 16 hex chars
}

func TestRequestID_inContext(t *testing.T) {
	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := RequestIDFromContext(r.Context())
		w.Header().Set("X-Request-ID", id)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	headerID := rec.Header().Get("X-Request-ID")
	assert.NotEmpty(t, headerID)
	assert.Contains(t, headerID, "req_")
}

func TestRequestID_uniquePerRequest(t *testing.T) {
	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/a", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/b", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.NotEqual(t, rec1.Header().Get("X-Request-ID"), rec2.Header().Get("X-Request-ID"))
}
