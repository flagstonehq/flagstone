package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

type usageEntry struct {
	FlagKey    string    `json:"flag_key"`
	Count      int64     `json:"count"`
	LastReason string    `json:"last_reason"`
	LastEvalAt time.Time `json:"last_evaluated_at"`
}

type usageReport struct {
	Entries []usageEntry `json:"entries"`
}

type usageTracker struct {
	mu       sync.Mutex
	entries  map[string]*usageEntry
	client   *Client
	interval time.Duration
	stopCh   chan struct{}
}

func newUsageTracker(c *Client) *usageTracker {
	return &usageTracker{
		entries:  make(map[string]*usageEntry),
		client:   c,
		interval: c.opts.usageInterval,
		stopCh:   make(chan struct{}),
	}
}

func (t *usageTracker) record(flagKey, reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	e, ok := t.entries[flagKey]
	if !ok {
		t.entries[flagKey] = &usageEntry{
			FlagKey:    flagKey,
			Count:      1,
			LastReason: reason,
			LastEvalAt: now,
		}
		return
	}
	e.Count++
	e.LastReason = reason
	e.LastEvalAt = now
}

func (t *usageTracker) flush(ctx context.Context) {
	t.mu.Lock()
	entries := make([]usageEntry, 0, len(t.entries))
	for _, e := range t.entries {
		entries = append(entries, *e)
	}
	t.entries = make(map[string]*usageEntry)
	t.mu.Unlock()

	if len(entries) == 0 {
		return
	}

	body, err := json.Marshal(usageReport{Entries: entries})
	if err != nil {
		t.client.opts.logger.Warn("usage: marshal", zap.Error(err))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		t.client.opts.endpoint+"/api/v1/sdk/usage", bytes.NewReader(body))
	if err != nil {
		t.client.opts.logger.Warn("usage: new request", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.client.opts.apiKey)

	resp, err := t.client.opts.httpClient.Do(req)
	if err != nil {
		t.client.opts.logger.Warn("usage: report", zap.Error(err))
		return
	}
	_ = resp.Body.Close()
}

func (t *usageTracker) start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.flush(ctx)
			case <-t.stopCh:
				return
			}
		}
	}()
}

func (t *usageTracker) stop(ctx context.Context) {
	close(t.stopCh)
	t.flush(ctx)
}
