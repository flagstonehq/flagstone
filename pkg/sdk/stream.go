package sdk

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	sseBackoffStart   = 1 * time.Second
	sseBackoffCeiling = 30 * time.Second
	sseResyncAfter    = 3
)

// streamEvent is one parsed SSE message.
type streamEvent struct {
	ID    string
	Event string
	Data  map[string]any
}

// streamConn is a single SSE consumer. It runs in its own goroutine and
// reports flag and segment changes via the onFlagChange / onSegmentChange
// callbacks, which the Client wires to signalRefresh.
type streamConn struct {
	endpoint        string
	apiKey          string
	httpClient      *http.Client
	logger          *zap.Logger
	onFlagChange    func()
	onSegmentChange func()
	lastEventID     string
}

func newStreamConn(opts clientOptions) *streamConn {
	return &streamConn{
		endpoint:   opts.endpoint,
		apiKey:     opts.apiKey,
		httpClient: opts.httpClient,
		logger:     opts.logger,
	}
}

// run blocks until ctx is cancelled. It dials the SSE endpoint, processes
// events, and reconnects with exponential backoff.
func (s *streamConn) run(ctx context.Context) {
	backoff := sseBackoffStart
	consecutiveFails := 0
	for {
		if err := s.dial(ctx); err != nil {
			consecutiveFails++
			s.logger.Warn("SSE dial failed",
				zap.Error(err),
				zap.Int("consecutive_failures", consecutiveFails),
			)
		} else {
			consecutiveFails = 0
			backoff = sseBackoffStart
		}
		if consecutiveFails > sseResyncAfter && s.onSegmentChange != nil {
			// Too many failures in a row: force a full resync on the
			// next read, since the SDK might be missing events.
			s.onSegmentChange()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > sseBackoffCeiling {
			backoff = sseBackoffCeiling
		}
	}
}

func (s *streamConn) dial(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(s.endpoint, "/")+"/api/v1/stream", http.NoBody)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if s.lastEventID != "" {
		req.Header.Set("Last-Event-ID", s.lastEventID)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var ev streamEvent
	var data strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			// Blank line dispatches the accumulated event.
			if data.Len() > 0 {
				_ = json.Unmarshal([]byte(data.String()), &ev.Data)
			}
			s.handleEvent(ev)
			ev = streamEvent{}
			data.Reset()
		case strings.HasPrefix(line, ":"):
			// Comment line (e.g. a keep-alive). Ignored per the SSE spec.
		case strings.HasPrefix(line, "id:"):
			ev.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		case strings.HasPrefix(line, "event:"):
			ev.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			// SSE allows multiple data: lines; they are joined with "\n",
			// and a single leading space after the colon is stripped.
			d := strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(d)
		}
	}
	return scanner.Err()
}

func (s *streamConn) handleEvent(ev streamEvent) {
	if ev.ID != "" {
		s.lastEventID = ev.ID
	}
	switch ev.Event {
	case "flag_change":
		if s.onFlagChange != nil {
			s.onFlagChange()
		}
	case "segment_change", "resync":
		if s.onSegmentChange != nil {
			s.onSegmentChange()
		}
	}
}
