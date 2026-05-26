package middleware

import (
	"net/http"
	"runtime/debug"

	"go.uber.org/zap"
)

// RecoverPanic catches panics from downstream handlers and returns a 500
// response with a structured error.
func RecoverPanic(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := debug.Stack()
					logger.Error("panic recovered",
						zap.Any("panic", rec),
						zap.String("stack", string(stack)),
					)
					Error(w, r, http.StatusInternalServerError,
						"INTERNAL_ERROR", "An internal server error occurred.")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
