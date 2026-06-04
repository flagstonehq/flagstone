package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/flagstonehq/flagstone/internal/api/middleware"
	"github.com/flagstonehq/flagstone/internal/storage"
	"github.com/flagstonehq/flagstone/pkg/engine"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type snapshotResponse struct {
	Environment string                         `json:"environment"`
	Flags       map[string]snapshotFlagJSON    `json:"flags"`
	Segments    map[string]snapshotSegmentJSON `json:"segments"`
	FetchedAt   time.Time                      `json:"fetched_at"`
	RequestID   string                         `json:"request_id,omitempty"`
}

type snapshotFlagJSON struct {
	Key                     string        `json:"key"`
	Enabled                 bool          `json:"enabled"`
	FlagType                string        `json:"flag_type"`
	DefaultValue            any           `json:"default_value"`
	EnvironmentDefaultValue any           `json:"environment_default_value,omitempty"`
	HasEnvironmentDefault   bool          `json:"has_environment_default"`
	Version                 int64         `json:"version"`
	Rules                   []engine.Rule `json:"rules"`
}

type snapshotSegmentJSON struct {
	Key        string               `json:"key"`
	Conditions engine.ConditionNode `json:"conditions"`
}

func (s *Server) handleSDKSnapshot(w http.ResponseWriter, r *http.Request) {
	envID := middleware.EnvironmentIDFromContext(r.Context())
	if envID == uuid.Nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Authentication is required.")
		return
	}

	env, err := s.stores.Environments.GetByID(r.Context(), envID)
	if err != nil {
		if errors.Is(err, storage.ErrEnvironmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Environment not found.")
			return
		}
		s.logger.Error("sdk snapshot: get environment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	dbConfigs, err := s.stores.FlagEnvironments.ListByEnvironment(r.Context(), envID)
	if err != nil {
		s.logger.Error("sdk snapshot: list by environment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	flags := make(map[string]snapshotFlagJSON, len(dbConfigs))
	for _, dbc := range dbConfigs {
		fc, err := buildFlagConfigFromJoined(dbc)
		if err != nil {
			s.logger.Warn("sdk snapshot: skip flag config",
				zap.String("flag_key", dbc.FlagKey), zap.Error(err))
			continue
		}
		flags[fc.Key] = snapshotFlagJSON{
			Key:                     fc.Key,
			Enabled:                 fc.Enabled,
			FlagType:                fc.FlagType,
			DefaultValue:            fc.DefaultValue,
			EnvironmentDefaultValue: fc.EnvironmentDefaultValue,
			HasEnvironmentDefault:   fc.HasEnvironmentDefault,
			Version:                 fc.Version,
			Rules:                   fc.Rules,
		}
	}

	dbSegments, err := s.stores.Segments.ListByProject(r.Context(), env.ProjectID)
	if err != nil {
		s.logger.Error("sdk snapshot: list segments", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	segments := make(map[string]snapshotSegmentJSON, len(dbSegments))
	for _, ds := range dbSegments {
		var cond engine.ConditionNode
		if len(ds.Rules) > 0 {
			if err := json.Unmarshal(ds.Rules, &cond); err != nil {
				s.logger.Warn("sdk snapshot: skip segment with invalid rules",
					zap.String("key", ds.Key), zap.Error(err))
				continue
			}
		}
		segments[ds.Key] = snapshotSegmentJSON{
			Key:        ds.Key,
			Conditions: cond,
		}
	}

	resp := snapshotResponse{
		Environment: env.Slug,
		Flags:       flags,
		Segments:    segments,
		FetchedAt:   time.Now().UTC(),
		RequestID:   middleware.RequestIDFromContext(r.Context()),
	}

	middleware.JSON(w, http.StatusOK, resp)
}
