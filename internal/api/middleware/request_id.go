package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// RequestID injects a unique identifier into each request's context and
// response headers.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := generateID()
			w.Header().Set("X-Request-ID", id)
			ctx := WithRequestID(r.Context(), id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "req_unknown"
	}
	return "req_" + hex.EncodeToString(b)
}
