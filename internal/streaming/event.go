package streaming

import (
	"time"

	"github.com/google/uuid"
)

// SSE event type constants.
const (
	EventFlagChange    = "flag_change"
	EventSegmentChange = "segment_change"
	EventResync        = "resync"
	EventHeartbeat     = "heartbeat"
)

// Event is a server-sent event published to SSE clients.
type Event struct {
	ID            int64          `json:"id"`
	Type          string         `json:"type"`
	EnvironmentID uuid.UUID      `json:"environment_id"`
	Payload       map[string]any `json:"payload"`
	Timestamp     time.Time      `json:"timestamp"`
}
