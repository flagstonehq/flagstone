package middleware

import (
	"net/http"
	"strings"
)

// RequireJSONContentType rejects POST, PUT, and PATCH requests that do not
// have a Content-Type of application/json.
func RequireJSONContentType() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				ct := r.Header.Get("Content-Type")
				if !hasJSONContentType(ct) {
					Error(w, r, http.StatusUnsupportedMediaType,
						"UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func hasJSONContentType(ct string) bool {
	ct = strings.TrimSpace(ct)
	ct, _, _ = strings.Cut(ct, ";")
	return strings.TrimSpace(ct) == "application/json"
}
