package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/flagstonehq/flagstone/internal/auth"
	"github.com/flagstonehq/flagstone/internal/storage"
	"github.com/google/uuid"
)

// AuthAPIKey validates a Bearer API key by computing its SHA-256 hash and
// looking it up in the database. It injects the environment ID into the
// request context on success.
func AuthAPIKey(stores *storage.Stores) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey, ok := extractBearerToken(r)
			if !ok {
				Error(w, r, http.StatusUnauthorized,
					"INVALID_CREDENTIALS", "Authentication is required.")
				return
			}

			keyHash := auth.HashAPIKey(rawKey)
			key, err := stores.APIKeys.GetByHash(r.Context(), keyHash)
			if err != nil {
				Error(w, r, http.StatusUnauthorized,
					"INVALID_CREDENTIALS", "The provided API key is invalid or revoked.")
				return
			}

			now := time.Now().UTC()
			if key.ExpiresAt != nil && !key.ExpiresAt.After(now) {
				Error(w, r, http.StatusUnauthorized,
					"INVALID_CREDENTIALS", "The provided API key is invalid or revoked.")
				return
			}

			go func(id uuid.UUID, usedAt time.Time, parent context.Context) {
				ctx, cancel := context.WithTimeout(parent, 2*time.Second)
				defer cancel()
				_ = stores.APIKeys.UpdateLastUsed(ctx, id, usedAt)
			}(key.ID, now, r.Context())

			ctx := WithEnvironmentID(r.Context(), key.EnvironmentID)
			ctx = WithAPIKeyID(ctx, key.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
