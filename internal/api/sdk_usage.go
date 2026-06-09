package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/flagstonehq/flagstone/internal/api/middleware"
	"github.com/flagstonehq/flagstone/internal/storage"
	"github.com/flagstonehq/flagstone/internal/telemetry"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type sdkUsageEntryJSON struct {
	FlagKey    string    `json:"flag_key"`
	Count      int64     `json:"count"`
	LastReason string    `json:"last_reason"`
	LastEvalAt time.Time `json:"last_evaluated_at"`
}

type sdkUsageRequest struct {
	Entries []sdkUsageEntryJSON `json:"entries"`
}

func (s *Server) handleSDKUsage(w http.ResponseWriter, r *http.Request) {
	envID := middleware.EnvironmentIDFromContext(r.Context())
	if envID == uuid.Nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Authentication is required.")
		return
	}

	var req sdkUsageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid usage report.")
		return
	}

	if len(req.Entries) == 0 {
		if s.metrics != nil {
			s.metrics.RecordSDKUsageReport(r.Context())
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	ctx := r.Context()

	env, err := s.stores.Environments.GetByID(ctx, envID)
	if err != nil {
		if errors.Is(err, storage.ErrEnvironmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Environment not found.")
			return
		}
		s.logger.Error("sdk usage: get environment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	const maxCountPerEntry = 100_000 // guard against malformed/adversarial payloads

	envAttr := telemetry.FlagstoneEnvironment.String(env.Slug)
	for _, entry := range req.Entries {
		n := entry.Count
		if entry.FlagKey == "" || n <= 0 {
			continue
		}
		if n > maxCountPerEntry {
			n = maxCountPerEntry
		}
		attrs := []attribute.KeyValue{
			telemetry.FeatureFlagKey.String(entry.FlagKey),
			telemetry.FeatureFlagResultReason.String(entry.LastReason),
			envAttr,
		}
		s.metrics.RecordEvaluationsBulk(ctx, n, attrs...)
	}

	if s.metrics != nil {
		s.metrics.RecordSDKUsageReport(ctx)
	}
	w.WriteHeader(http.StatusNoContent)
}
