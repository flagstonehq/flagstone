package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/thomas-vilte/flagstone/internal/api/middleware"
	"github.com/thomas-vilte/flagstone/pkg/engine"
	"github.com/thomas-vilte/flagstone/internal/storage"
	"go.uber.org/zap"
)

type evaluateFlagRequest struct {
	Context map[string]any `json:"context"`
}

type evaluateFlagResponse struct {
	Key       string        `json:"key"`
	Value     any           `json:"value"`
	Reason    engine.Reason `json:"reason"`
	RuleIndex int           `json:"rule_index"`
	RequestID string        `json:"request_id,omitempty"`
}

type evaluateFlagsResponse struct {
	Flags       map[string]engine.EvaluateResult `json:"flags"`
	Environment string                           `json:"environment"`
	EvaluatedAt string                           `json:"evaluated_at"`
	RequestID   string                           `json:"request_id,omitempty"`
}

func (s *Server) handleEvaluateFlag(w http.ResponseWriter, r *http.Request) {
	envID := middleware.EnvironmentIDFromContext(r.Context())
	if envID == uuid.Nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Authentication is required.")
		return
	}

	flagKey := r.PathValue("key")
	if flagKey == "" {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Flag key is required.")
		return
	}

	var req evaluateFlagRequest
	if err := DecodeJSON(r, &req); err != nil {
		s.writeDecodeError(w, r, err)
		return
	}

	if err := validateEvalContext(req.Context); err != nil {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	env, err := s.stores.Environments.GetByID(r.Context(), envID)
	if err != nil {
		if errors.Is(err, storage.ErrEnvironmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Environment not found.")
			return
		}
		s.logger.Error("evaluate: get environment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	flag, err := s.stores.Flags.GetByKey(r.Context(), env.ProjectID, flagKey)
	if err != nil {
		if errors.Is(err, storage.ErrFlagNotFound) {
			middleware.JSON(w, http.StatusOK, evaluateFlagResponse{
				Key:       flagKey,
				Value:     false,
				Reason:    engine.ReasonFlagNotFound,
				RuleIndex: -1,
				RequestID: middleware.RequestIDFromContext(r.Context()),
			})
			return
		}
		s.logger.Error("evaluate: get flag", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	flagEnv, err := s.stores.FlagEnvironments.Get(r.Context(), flag.ID, envID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			middleware.JSON(w, http.StatusOK, evaluateFlagResponse{
				Key:       flag.Key,
				Value:     false,
				Reason:    engine.ReasonFlagNotFound,
				RuleIndex: -1,
				RequestID: middleware.RequestIDFromContext(r.Context()),
			})
			return
		}
		s.logger.Error("evaluate: get flag env", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	segments, err := s.loadEngineSegments(r.Context(), env.ProjectID)
	if err != nil {
		s.logger.Error("evaluate: load segments", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	fc, err := buildFlagConfig(flag, flagEnv)
	if err != nil {
		s.logger.Error("evaluate: build flag config", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	result := s.engine.Evaluate(engine.EvaluateRequest{
		FlagConfig: fc,
		Segments:   segments,
		Context:    req.Context,
	})

	middleware.JSON(w, http.StatusOK, evaluateFlagResponse{
		Key:       flag.Key,
		Value:     result.Value,
		Reason:    result.Reason,
		RuleIndex: result.RuleIndex,
		RequestID: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) handleEvaluateFlags(w http.ResponseWriter, r *http.Request) {
	envID := middleware.EnvironmentIDFromContext(r.Context())
	if envID == uuid.Nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Authentication is required.")
		return
	}

	var req evaluateFlagRequest
	if err := DecodeJSON(r, &req); err != nil {
		s.writeDecodeError(w, r, err)
		return
	}

	if err := validateEvalContext(req.Context); err != nil {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	env, err := s.stores.Environments.GetByID(r.Context(), envID)
	if err != nil {
		if errors.Is(err, storage.ErrEnvironmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Environment not found.")
			return
		}
		s.logger.Error("evaluate: get environment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	dbConfigs, err := s.stores.FlagEnvironments.ListByEnvironment(r.Context(), envID)
	if err != nil {
		s.logger.Error("evaluate: list by environment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	segments, err := s.loadEngineSegments(r.Context(), env.ProjectID)
	if err != nil {
		s.logger.Error("evaluate: load segments", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	flags := make([]engine.FlagConfig, 0, len(dbConfigs))
	for _, dbc := range dbConfigs {
		fc, err := buildFlagConfigFromJoined(dbc)
		if err != nil {
			s.logger.Error("evaluate: skip flag config build", zap.String("flag_key", dbc.FlagKey), zap.Error(err))
			continue
		}
		flags = append(flags, fc)
	}

	results := s.engine.EvaluateAll(flags, segments, req.Context)

	reqID := middleware.RequestIDFromContext(r.Context())
	middleware.JSON(w, http.StatusOK, evaluateFlagsResponse{
		Flags:       results,
		Environment: env.Slug,
		EvaluatedAt: time.Now().UTC().Format(timeFormat),
		RequestID:   reqID,
	})
}

func (s *Server) loadEngineSegments(ctx context.Context, projectID uuid.UUID) (map[string]engine.Segment, error) {
	dbSegments, err := s.stores.Segments.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	segments := make(map[string]engine.Segment, len(dbSegments))
	for _, ds := range dbSegments {
		var cond engine.ConditionNode
		if len(ds.Rules) > 0 {
			if err := json.Unmarshal(ds.Rules, &cond); err != nil {
				s.logger.Warn("evaluate: skip segment with invalid rules", zap.String("key", ds.Key), zap.Error(err))
				continue
			}
		}
		segments[ds.Key] = engine.Segment{
			Key:        ds.Key,
			Conditions: cond,
		}
	}
	return segments, nil
}

func buildFlagConfig(flag *storage.Flag, flagEnv *storage.FlagEnvironment) (engine.FlagConfig, error) {
	var rules []engine.Rule
	if len(flagEnv.Rules) > 0 {
		if err := json.Unmarshal(flagEnv.Rules, &rules); err != nil {
			return engine.FlagConfig{}, fmt.Errorf("unmarshal rules: %w", err)
		}
	}

	defaultValue, err := parseDefaultValue(flag.Type, flag.DefaultValue)
	if err != nil {
		return engine.FlagConfig{}, fmt.Errorf("parse default value: %w", err)
	}

	fc := engine.FlagConfig{
		Key:          flag.Key,
		Enabled:      flagEnv.Enabled,
		FlagType:     flag.Type,
		DefaultValue: defaultValue,
		Rules:        rules,
		Version:      flagEnv.Version,
	}

	if flagEnv.DefaultValue != nil && len(*flagEnv.DefaultValue) > 0 {
		envVal, err := parseDefaultValue(flag.Type, *flagEnv.DefaultValue)
		if err != nil {
			return engine.FlagConfig{}, fmt.Errorf("parse env default value: %w", err)
		}
		fc.EnvironmentDefaultValue = envVal
		fc.HasEnvironmentDefault = true
	}

	return fc, nil
}

func buildFlagConfigFromJoined(dbc storage.FlagEnvironmentConfig) (engine.FlagConfig, error) {
	var rules []engine.Rule
	if len(dbc.Rules) > 0 {
		if err := json.Unmarshal(dbc.Rules, &rules); err != nil {
			return engine.FlagConfig{}, fmt.Errorf("unmarshal rules: %w", err)
		}
	}

	defaultValue, err := parseDefaultValue(dbc.FlagType, dbc.FlagDefaultValue)
	if err != nil {
		return engine.FlagConfig{}, fmt.Errorf("parse default value: %w", err)
	}

	fc := engine.FlagConfig{
		Key:          dbc.FlagKey,
		Enabled:      dbc.Enabled,
		FlagType:     dbc.FlagType,
		DefaultValue: defaultValue,
		Rules:        rules,
		Version:      dbc.Version,
	}

	if dbc.EnvironmentDefaultValue != nil && len(*dbc.EnvironmentDefaultValue) > 0 {
		envVal, err := parseDefaultValue(dbc.FlagType, *dbc.EnvironmentDefaultValue)
		if err != nil {
			return engine.FlagConfig{}, fmt.Errorf("parse env default value: %w", err)
		}
		fc.EnvironmentDefaultValue = envVal
		fc.HasEnvironmentDefault = true
	}

	return fc, nil
}

func parseDefaultValue(flagType string, raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	switch flagType {
	case "boolean":
		var v bool
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return v, nil
	case "number":
		var v float64
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return v, nil
	case "string":
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return v, nil
	default:
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return v, nil
	}
}

func validateEvalContext(ctx map[string]any) error {
	if ctx == nil {
		return nil
	}
	if len(ctx) > 100 {
		return fmt.Errorf("context must have at most 100 keys")
	}
	for k, v := range ctx {
		if len(k) > 128 {
			return fmt.Errorf("context key %q exceeds maximum length of 128 characters", k)
		}
		if s, ok := v.(string); ok && len(s) > 1024 {
			return fmt.Errorf("context value for key %q exceeds maximum length of 1024 characters", k)
		}
		switch tv := v.(type) {
		case string, float64, bool:
		case []any:
			for _, item := range tv {
				if _, ok := item.(string); !ok {
					return fmt.Errorf("context key %q: array must contain only strings", k)
				}
			}
		default:
			return fmt.Errorf("context key %q has an unsupported type", k)
		}
	}
	return nil
}
