package sdk

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestEdge_FlagNotFound(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err = c.Bool(context.Background(), "this-flag-does-not-exist", nil)
	if err == nil {
		t.Fatal("expected error for missing flag")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got %v", err)
	}
}

func TestEdge_TypeMismatch(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	c, _ := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err := c.String(context.Background(), "new-checkout", nil)
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
	if !strings.Contains(err.Error(), "not string") {
		t.Fatalf("expected type error mentioning 'not string', got %v", err)
	}
}

func TestEdge_ContextCancelled(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	c, _ := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v, err := c.Bool(ctx, "new-checkout", map[string]any{"user_id": "42"})
	if err != nil {
		t.Fatalf("expected success even with cancelled ctx, got %v", err)
	}
	_ = v
}

func TestEdge_StartCancelledCtx(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	c, _ := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Start(ctx); err == nil {
		t.Fatal("expected Start to fail with cancelled ctx")
	}
	_, err := c.Bool(context.Background(), "new-checkout", nil)
	if err == nil {
		t.Fatal("expected error when snapshot never loaded")
	}
	if !strings.Contains(err.Error(), "not loaded") {
		t.Fatalf("expected 'not loaded' error, got %v", err)
	}
}

func TestEdge_MalformedJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{not valid json"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c, _ := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	if err := c.Start(context.Background()); err == nil {
		t.Fatal("expected Start to fail on malformed JSON")
	}
}

func TestEdge_5xx(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c, _ := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	if err := c.Start(context.Background()); err == nil {
		t.Fatal("expected Start to fail on 5xx")
	}
}

func TestEdge_SSE_Heartbeat(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleSnapshot))
	})
	var conns int32
	mux.HandleFunc("/api/v1/stream", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&conns, 1)
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		if flusher != nil {
			flusher.Flush()
		}
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, _ := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt32(&conns); got != 1 {
		t.Fatalf("expected exactly 1 SSE connection, got %d", got)
	}
}

func TestEdge_SSE_LongIdle(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleSnapshot))
	})
	var conns int32
	mux.HandleFunc("/api/v1/stream", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&conns, 1)
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, _ := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt32(&conns); got != 1 {
		t.Fatalf("expected exactly 1 SSE connection, got %d (client reconnected prematurely)", got)
	}
}

func TestEdge_AllEmpty(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	c, _ := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	got, err := c.All(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(got))
	}
}

func TestEdge_EvalBeforeStart(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	c, _ := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	start := time.Now()
	_, err := c.Bool(context.Background(), "new-checkout", nil)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error before Start")
	}
	if !strings.Contains(err.Error(), "not loaded") {
		t.Fatalf("expected 'not loaded' error pointing at Start(), got %v", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("Bool before Start took %v; expected immediate return", elapsed)
	}
}
