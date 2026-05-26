package middleware

import (
	"net/http"

	"github.com/thomas-vilte/flagstone/internal/auth"
)

// RequireRole returns a middleware that restricts access to users whose role
// meets or exceeds the minimum required level.
func RequireRole(minimum auth.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromContext(r.Context())
			if claims == nil {
				Error(w, r, http.StatusUnauthorized,
					"INVALID_CREDENTIALS", "Authentication is required.")
				return
			}

			role := auth.ParseRole(claims.Role)
			if !role.AtLeast(minimum) {
				Error(w, r, http.StatusForbidden,
					"INSUFFICIENT_PERMISSIONS", "You do not have permission to perform this action.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
