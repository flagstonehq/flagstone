package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/flagstonehq/flagstone/pkg/engine"
	"go.uber.org/zap"
)

// Client is a Flagstone SDK instance scoped to one API key (and thus one
// environment). Safe for concurrent use.
//
// The hot evaluation path (Bool/String/Number/JSON) is lock-free: it
// loads the current *snapshot atomically and runs the engine in memory.
// All network I/O is owned by refreshLoop, a single-writer background
// goroutine started by Start. Eval never blocks on the network.
type Client struct {
	opts          clientOptions
	engine        *engine.Engine
	cache         *snapshotCache
	stream        *streamConn
	refreshSignal chan struct{}
	startOnce     sync.Once
	startErr      error
	started       chan struct{}
	done          chan struct{}
	closeOnce     sync.Once
}

// New constructs a Client. Endpoint and APIKey are required (returned as
// errors). Other options use sensible defaults.
//
// New does not perform any network I/O. Call Start to load the initial
// snapshot and start the background refresh loop.
func New(opts ...Option) (*Client, error) {
	cfg := clientOptions{}
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.defaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &Client{
		opts:          cfg,
		engine:        engine.New(cfg.logger),
		cache:         newSnapshotCache(),
		stream:        newStreamConn(cfg),
		refreshSignal: make(chan struct{}, 1),
		started:       make(chan struct{}),
		done:          make(chan struct{}),
	}, nil
}

// Start performs the initial snapshot fetch (blocking, using the given
// ctx) and then runs a background refresh loop and SSE consumer. It is
// safe to call multiple times; only the first call does the work.
// Returns the error from the initial fetch, if any.
//
// The background goroutines stop when EITHER ctx is cancelled or Close
// is called, whichever happens first.
func (c *Client) Start(ctx context.Context) error {
	c.startOnce.Do(func() {
		c.stream.onFlagChange = c.signalRefresh
		c.stream.onSegmentChange = c.signalRefresh

		// runCtx drives the background loops. As a child of the caller's
		// ctx, it is cancelled automatically when the caller cancels ctx;
		// the goroutine below also cancels it when Close closes c.done.
		runCtx, cancel := context.WithCancel(ctx)
		go func() {
			select {
			case <-c.done:
				cancel()
			case <-runCtx.Done():
			}
		}()

		// The initial fetch uses the caller's ctx (the caller is blocking
		// on it), so Close does not abort it mid-flight.
		snap, err := c.refetch(ctx)
		if err == nil {
			c.cache.store(snap)
		}
		c.startErr = err
		close(c.started)
		go c.stream.run(runCtx)
		go c.refreshLoop(runCtx)
	})
	<-c.started
	return c.startErr
}

// Close stops the SSE stream and the background refresh loop. It is
// idempotent and safe to call concurrently. Evaluations still work
// after Close — they serve the last snapshot from memory.
func (c *Client) Close() error {
	c.closeOnce.Do(func() { close(c.done) })
	return nil
}

// Bool evaluates a boolean flag. It returns def if the flag is missing,
// is not boolean, or the snapshot has not been loaded yet. It never
// returns an error and never blocks on the network — use BoolDetail when
// you need the reason or error.
func (c *Client) Bool(ctx context.Context, key string, def bool, evalCtx map[string]any) bool {
	return c.BoolDetail(ctx, key, def, evalCtx).Value.(bool)
}

// String evaluates a string flag, returning def on any error.
func (c *Client) String(ctx context.Context, key, def string, evalCtx map[string]any) string {
	return c.StringDetail(ctx, key, def, evalCtx).Value.(string)
}

// Number evaluates a numeric flag, returning def on any error.
func (c *Client) Number(ctx context.Context, key string, def float64, evalCtx map[string]any) float64 {
	return c.NumberDetail(ctx, key, def, evalCtx).Value.(float64)
}

// JSON evaluates a flag of any JSON type, returning def on any error.
func (c *Client) JSON(ctx context.Context, key string, def any, evalCtx map[string]any) any {
	return c.JSONDetail(ctx, key, def, evalCtx).Value
}

// All evaluates every flag in the current snapshot. If no snapshot has
// been loaded yet (Start not called or initial fetch failed), returns
// an empty map — never blocks on the network.
func (c *Client) All(_ context.Context, evalCtx map[string]any) (map[string]Value, error) {
	snap := c.cache.get()
	out := make(map[string]Value, len(snap.flags))
	for key, fc := range snap.flags {
		result := c.engine.Evaluate(engine.EvaluateRequest{
			FlagConfig: fc,
			Segments:   snap.segments,
			Context:    evalCtx,
		})
		out[key] = Value{Value: result.Value, Reason: result.Reason}
	}
	return out, nil
}

// Value is the result of evaluating a single flag.
type Value struct {
	Value  any
	Reason engine.Reason
}

// signalRefresh is non-blocking. It is called by the SSE stream when a
// flag_change / segment_change / resync event arrives, and by evalDetail
// when it sees an empty snapshot.
func (c *Client) signalRefresh() {
	select {
	case c.refreshSignal <- struct{}{}:
	default:
	}
}

// refreshLoop owns all writes to the snapshot cache. It reacts to the
// TTL ticker, the SSE-driven refresh signal, and context cancellation.
func (c *Client) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(c.opts.cacheTTL)
	defer ticker.Stop()
	for {
		select {
		case <-c.refreshSignal:
			c.doRefresh(ctx)
		case <-ticker.C:
			c.doRefresh(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) doRefresh(ctx context.Context) {
	if snap, err := c.refetch(ctx); err != nil {
		c.opts.logger.Warn("snapshot refresh failed", zap.Error(err))
	} else {
		c.cache.store(snap)
	}
}

// refetch performs one HTTP GET against /api/v1/sdk/snapshot. The
// returned *snapshot is a fresh allocation; the caller may store it
// without copying.
func (c *Client) refetch(ctx context.Context) (*snapshot, error) {
	var resp snapshotResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/sdk/snapshot", nil, &resp); err != nil {
		return nil, err
	}
	return resp.toSnapshot(), nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method,
		strings.TrimRight(c.opts.endpoint, "/")+path, reqBody)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.opts.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.opts.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, resp.Status)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode: %w", err)
		}
	}
	return nil
}
