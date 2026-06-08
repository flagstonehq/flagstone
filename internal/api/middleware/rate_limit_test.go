package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flagstonehq/flagstone/internal/api/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIPRateLimiter_Allow(t *testing.T) {
	rl := middleware.NewIPRateLimiter(3, time.Minute)

	for i := range 3 {
		assert.True(t, rl.Allow("1.2.3.4"), "request %d should be allowed", i+1)
	}
	assert.False(t, rl.Allow("1.2.3.4"), "4th request should be denied")
}

func TestIPRateLimiter_DifferentIPs(t *testing.T) {
	rl := middleware.NewIPRateLimiter(1, time.Minute)

	assert.True(t, rl.Allow("1.1.1.1"))
	assert.False(t, rl.Allow("1.1.1.1"))
	assert.True(t, rl.Allow("2.2.2.2"), "different IP should have its own limit")
}

func TestIPRateLimiter_WindowReset(t *testing.T) {
	rl := middleware.NewIPRateLimiter(1, 50*time.Millisecond)

	require.True(t, rl.Allow("1.2.3.4"))
	require.False(t, rl.Allow("1.2.3.4"))

	time.Sleep(60 * time.Millisecond)
	assert.True(t, rl.Allow("1.2.3.4"), "should be allowed after window resets")
}

func TestRateLimitMiddleware_Blocks(t *testing.T) {
	rl := middleware.NewIPRateLimiter(2, time.Minute)
	mw := middleware.RateLimit(rl, nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := range 2 {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "request %d should pass", i+1)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}
