package sdk

import (
	"time"

	"github.com/flagstonehq/flagstone/pkg/engine"
)

// snapshotResponse matches GET /api/v1/sdk/snapshot.
type snapshotResponse struct {
	Environment string                       `json:"environment"`
	Flags       map[string]flagEnvConfigJSON `json:"flags"`
	Segments    map[string]segmentJSON       `json:"segments"`
	FetchedAt   time.Time                    `json:"fetched_at"`
}

type flagEnvConfigJSON struct {
	Key                     string        `json:"key"`
	Enabled                 bool          `json:"enabled"`
	FlagType                string        `json:"flag_type"`
	DefaultValue            any           `json:"default_value"`
	EnvironmentDefaultValue any           `json:"environment_default_value,omitempty"`
	HasEnvironmentDefault   bool          `json:"has_environment_default"`
	Version                 int64         `json:"version"`
	Rules                   []engine.Rule `json:"rules"`
}

type segmentJSON struct {
	Key        string               `json:"key"`
	Conditions engine.ConditionNode `json:"conditions"`
}

func (r snapshotResponse) toSnapshot() *snapshot {
	s := newSnapshot()
	for k, v := range r.Flags {
		s.flags[k] = engine.FlagConfig{
			Key:                     v.Key,
			Enabled:                 v.Enabled,
			FlagType:                v.FlagType,
			DefaultValue:            v.DefaultValue,
			EnvironmentDefaultValue: v.EnvironmentDefaultValue,
			HasEnvironmentDefault:   v.HasEnvironmentDefault,
			Version:                 v.Version,
			Rules:                   v.Rules,
		}
	}
	for k, v := range r.Segments {
		s.segments[k] = engine.Segment{
			Key:        v.Key,
			Conditions: v.Conditions,
		}
	}
	s.fetchedAt = time.Now()
	return s
}
