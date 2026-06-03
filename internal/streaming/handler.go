package streaming

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/flagstonehq/flagstone/internal/api/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	heartbeatInterval = 30 * time.Second
)

// Handler handles SSE connections and streams events to SDK clients.
type Handler struct {
	hub    *Hub
	logger *zap.Logger
}

// NewHandler creates a new SSE Handler.
func NewHandler(hub *Hub, logger *zap.Logger) *Handler {
	return &Handler{
		hub:    hub,
		logger: logger,
	}
}

// ServeSSE handles an SSE connection, registers the client with the hub,
// replays missed events, and streams live events until disconnect.
func (h *Handler) ServeSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	envID := middleware.EnvironmentIDFromContext(r.Context())
	if envID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	apiKeyID := middleware.APIKeyIDFromContext(r.Context())
	if apiKeyID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	lastID := parseLastEventID(r.Header.Get("Last-Event-ID"))
	client := h.hub.NewClient(envID, apiKeyID, lastID)
	if !h.hub.Register(client) {
		http.Error(w, "too many connections", http.StatusTooManyRequests)
		return
	}
	defer h.hub.Unregister(client)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	w.WriteHeader(http.StatusOK)

	if lastID > 0 {
		if events, err := h.hub.Replay(r.Context(), envID, lastID); err == nil {
			for _, ev := range events {
				writeSSE(w, ev)
			}
			flusher.Flush()
		} else {
			_, _ = fmt.Fprint(w, "event: resync\ndata: {}\n\n")
			flusher.Flush()
		}
	}

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-client.send:
			if !ok {
				return
			}
			writeSSE(w, ev)
			flusher.Flush()
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, "event: heartbeat\ndata: {\"ts\":%q}\n\n",
				time.Now().UTC().Format(time.RFC3339))
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, ev Event) {
	data, _ := json.Marshal(ev.Payload)
	_, _ = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.ID, ev.Type, data)
}

func parseLastEventID(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
