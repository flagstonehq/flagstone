package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	replayBufferSize = 1000
	replayKeyTTL     = 30 * time.Minute
	streamKeyPrefix  = "flagstone:stream:"
	idCounterKey     = "flagstone:stream:counter"
)

// Client represents a single SSE connection.
type Client struct {
	envID       uuid.UUID
	send        chan Event
	lastEventID int64
	apiKeyID    uuid.UUID
}

// Hub manages SSE client connections, routing, and event broadcast.
type Hub struct {
	rdb        *redis.Client
	mu         sync.RWMutex
	clients    map[uuid.UUID]map[*Client]struct{}
	perKey     map[uuid.UUID]int
	perKeyCap  int
	register   chan *Client
	unregister chan *Client
	publish    chan Event
	stop       chan struct{}
	logger     *zap.Logger
}

// HubConfig configures a Hub instance.
type HubConfig struct {
	Redis        *redis.Client
	PerAPIKeyCap int
	Logger       *zap.Logger
}

// NewHub creates and returns a new Hub with the given config.
func NewHub(cfg HubConfig) *Hub {
	if cfg.PerAPIKeyCap <= 0 {
		cfg.PerAPIKeyCap = 10
	}

	return &Hub{
		rdb:        cfg.Redis,
		clients:    make(map[uuid.UUID]map[*Client]struct{}),
		perKey:     make(map[uuid.UUID]int),
		perKeyCap:  cfg.PerAPIKeyCap,
		register:   make(chan *Client),
		unregister: make(chan *Client),
		publish:    make(chan Event, 256),
		stop:       make(chan struct{}),
		logger:     cfg.Logger,
	}
}

// Run starts the hub's main event loop. It processes client registration,
// unregistration, and event publish requests until the context is cancelled.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			close(h.stop)
			return
		case client := <-h.register:
			h.mu.Lock()
			if h.perKey[client.apiKeyID] >= h.perKeyCap {
				close(client.send)
				h.mu.Unlock()
				continue
			}
			if h.clients[client.envID] == nil {
				h.clients[client.envID] = make(map[*Client]struct{})
			}
			h.clients[client.envID][client] = struct{}{}
			h.perKey[client.apiKeyID]++
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.envID][client]; ok {
				delete(h.clients[client.envID], client)
				h.perKey[client.apiKeyID]--
				if len(h.clients[client.envID]) == 0 {
					delete(h.clients, client.envID)
				}
			}
			close(client.send)
			h.mu.Unlock()
		case ev := <-h.publish:
			h.broadcast(ev)
			h.persistToRedis(ev)
		}
	}
}

// Register sends a client to the hub for registration. Returns false if the
// hub is at capacity.
func (h *Hub) Register(c *Client) bool {
	select {
	case h.register <- c:
		return true
	default:
		return false
	}
}

// Unregister sends a client to the hub for unregistration.
func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}

// Publish enqueues an event for broadcast to all matching clients.
func (h *Hub) Publish(ev Event) {
	h.publish <- ev
}

// NewClient creates a Client struct for the given environment, API key, and
// last received event ID.
func (h *Hub) NewClient(envID, apiKeyID uuid.UUID, lastEventID int64) *Client {
	return &Client{
		envID:       envID,
		send:        make(chan Event, 16),
		lastEventID: lastEventID,
		apiKeyID:    apiKeyID,
	}
}

// Replay fetches buffered events from Redis for the environment since the
// given event ID.
func (h *Hub) Replay(ctx context.Context, envID uuid.UUID, sinceID int64) ([]Event, error) {
	key := streamKeyPrefix + envID.String()
	raw, err := h.rdb.LRange(ctx, key, 0, replayBufferSize-1).Result()
	if err != nil {
		return nil, fmt.Errorf("replay: %w", err)
	}
	events := make([]Event, 0, len(raw))
	for _, s := range raw {
		var ev Event
		if err := json.Unmarshal([]byte(s), &ev); err != nil {
			continue
		}
		if ev.ID > sinceID {
			events = append(events, ev)
		}
	}
	return events, nil
}

func (h *Hub) broadcast(ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients[ev.EnvironmentID] {
		select {
		case client.send <- ev:
		default:
			h.logger.Warn("sse client buffer full, dropping",
				zap.String("env_id", ev.EnvironmentID.String()),
				zap.Int64("event_id", ev.ID),
			)
		}
	}
}

func (h *Hub) persistToRedis(ev Event) {
	if h.rdb == nil {
		return
	}
	key := streamKeyPrefix + ev.EnvironmentID.String()
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	pipe := h.rdb.Pipeline()
	pipe.LPush(context.Background(), key, data)
	pipe.LTrim(context.Background(), key, 0, replayBufferSize-1)
	pipe.Expire(context.Background(), key, replayKeyTTL)
	if _, err := pipe.Exec(context.Background()); err != nil {
		h.logger.Error("redis persist event", zap.Error(err))
	}
}
