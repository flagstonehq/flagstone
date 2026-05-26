package middleware

import (
	"net/http"
	"strings"

	"github.com/thomas-vilte/flagstone/internal/auth"
)

// AuthJWT validates a Bearer JWT access token and injects the claims into the
// request context.
func AuthJWT(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken, ok := extractBearerToken(r)
			if !ok {
				Error(w, r, http.StatusUnauthorized,
					"INVALID_CREDENTIALS", "Authentication is required.")
				return
			}

			claims, err := auth.ValidateAccessToken(rawToken, secret)
			if err != nil {
				Error(w, r, http.StatusUnauthorized,
					"INVALID_CREDENTIALS", "The provided token is invalid or expired.")
				return
			}

			ctx := WithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearerToken(r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", false
	}
	return strings.TrimSpace(authHeader[len("Bearer "):]), true
}
