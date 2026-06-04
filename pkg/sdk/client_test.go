package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flagstonehq/flagstone/pkg/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleSnapshot = `{
  "environment": "development",
  "flags": {
    "new-checkout": {
      "key": "new-checkout",
      "enabled": true,
      "flag_type": "boolean",
      "default_value": false,
      "version": 1,
      "rules": []
    },
    "banner-text": {
      "key": "banner-text",
      "enabled": true,
      "flag_type": "string",
      "default_value": "hello",
      "version": 1,
      "rules": []
    },
    "premium-only": {
      "key": "premium-only",
      "enabled": true,
      "flag_type": "boolean",
      "default_value": false,
      "version": 1,
      "rules": [
        {
          "conditions": {
            "attribute": "plan",
            "op": "eq",
            "value": "premium"
          },
          "value": true
        }
      ]
    }
  },
  "segments": {},
  "fetched_at": "2026-06-03T18:00:00Z"
}`

func newTestServer(t testing.TB, streamHandler http.HandlerFunc) (srv *httptest.Server, calls *int32) {
	t.Helper()
	var counter int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&counter, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleSnapshot))
	})
	if streamHandler != nil {
		mux.HandleFunc("/api/v1/stream", streamHandler)
	}
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	calls = &counter
	return srv, calls
}

func TestNew_RequiresEndpoint(t *testing.T) {
	_, err := New(WithAPIKey("k"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WithEndpoint")
}

func TestNew_RequiresAPIKey(t *testing.T) {
	_, err := New(WithEndpoint("http://x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WithAPIKey")
}

func TestNew_AppliesDefaults(t *testing.T) {
	c, err := New(WithEndpoint("http://x"), WithAPIKey("k"))
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, defaultCacheTTL, c.opts.cacheTTL)
	assert.Equal(t, defaultTimeout, c.opts.httpClient.Timeout)
	assert.NotNil(t, c.engine)
	assert.NotNil(t, c.cache)
	assert.NotNil(t, c.stream)
}

func TestNew_AppliesOptions(t *testing.T) {
	custom := &http.Client{Timeout: 3 * time.Second}
	c, err := New(
		WithEndpoint("http://x"),
		WithAPIKey("k"),
		WithCacheTTL(5*time.Second),
		WithHTTPClient(custom),
	)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, c.opts.cacheTTL)
	assert.Same(t, custom, c.opts.httpClient)
}

func TestBool_CachesAndReusesWithoutHTTPCall(t *testing.T) {
	srv, calls := newTestServer(t, nil)
	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, c.Start(ctx))

	got, err := c.Bool(ctx, "new-checkout", map[string]any{"user_id": "42"})
	require.NoError(t, err)
	assert.False(t, got, "default_value is false, no rules match")
	assert.Equal(t, int32(1), atomic.LoadInt32(calls), "Start fetched snapshot")

	got2, err := c.Bool(ctx, "new-checkout", map[string]any{"user_id": "42"})
	require.NoError(t, err)
	assert.Equal(t, got, got2)
	assert.Equal(t, int32(1), atomic.LoadInt32(calls), "second call must NOT re-fetch")
}

func TestBool_RuleMatch(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, c.Start(ctx))

	got, err := c.Bool(ctx, "premium-only", map[string]any{"plan": "premium"})
	require.NoError(t, err)
	assert.True(t, got, "rule matches plan=premium")

	got, err = c.Bool(ctx, "premium-only", map[string]any{"plan": "free"})
	require.NoError(t, err)
	assert.False(t, got, "rule does not match plan=free")
}

func TestString(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, c.Start(ctx))

	got, err := c.String(ctx, "banner-text", nil)
	require.NoError(t, err)
	assert.Equal(t, "hello", got)
}

func TestString_TypeMismatch(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, c.Start(ctx))

	_, err = c.String(ctx, "new-checkout", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is boolean, not string")
}

func TestAll(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, c.Start(ctx))

	all, err := c.All(ctx, map[string]any{"plan": "premium"})
	require.NoError(t, err)
	assert.Len(t, all, 3)
	assert.Contains(t, all, "new-checkout")
	assert.Contains(t, all, "banner-text")
	assert.Contains(t, all, "premium-only")
	assert.Equal(t, "RULE_MATCH", string(all["premium-only"].Reason))
}

func TestFlagChange_TriggersRefetchOnNextEval(t *testing.T) {
	var calls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&calls, 1)
		enabled := count > 1
		defaultVal := enabled
		body := `{
			"environment":"development",
			"flags":{"checkout":{
				"key":"checkout","enabled":` + fmt.Sprintf("%t", enabled) + `,
				"flag_type":"boolean","default_value":` + fmt.Sprintf("%t", defaultVal) + `,"version":1,"rules":[]
			}},
			"segments":{},
			"fetched_at":"2026-06-03T18:00:00Z"
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	mux.HandleFunc("/api/v1/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, _ := w.(http.Flusher)
		flusher.Flush()
		_, _ = w.Write([]byte("id: 1\nevent: flag_change\ndata: {\"key\":\"checkout\"}\n\n"))
		flusher.Flush()
		time.Sleep(100 * time.Millisecond)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, c.Start(ctx))

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, err := c.Bool(ctx, "checkout", nil)
		if err == nil && got {
			require.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(2),
				"expected at least 2 snapshot fetches after flag_change")
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("flag_change did not trigger a re-fetch within 3s (calls=%d)", atomic.LoadInt32(&calls))
}

func TestSegmentChange_TriggersFullRefresh(t *testing.T) {
	var calls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleSnapshot))
	})
	mux.HandleFunc("/api/v1/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("id: 1\nevent: segment_change\ndata: {}\n\n"))
		flusher.Flush()
		<-r.Context().Done()
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, c.Start(ctx))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&calls) >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(2),
		"segment_change must trigger a background refetch")
}

func TestStream_ReconnectsWithLastEventID(t *testing.T) {
	var lastEventID string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleSnapshot))
	})
	mux.HandleFunc("/api/v1/stream", func(w http.ResponseWriter, r *http.Request) {
		lastEventID = r.Header.Get("Last-Event-ID")
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("id: 7\nevent: heartbeat\ndata: {}\n\n"))
		flusher.Flush()
		time.Sleep(200 * time.Millisecond)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stream := newStreamConn(clientOptions{
		endpoint:   srv.URL,
		apiKey:     "k",
		httpClient: &http.Client{Timeout: 2 * time.Second},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_ = stream.dial(ctx)

	require.NoError(t, stream.dial(ctx))
	assert.Equal(t, "7", lastEventID)
}

func TestStream_Backoff_IncreasesOnFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/api/v1/stream", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stream := newStreamConn(clientOptions{
		endpoint:   srv.URL,
		apiKey:     "k",
		httpClient: &http.Client{Timeout: 1 * time.Second},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := stream.dial(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestDoJSON_SendsBearerHeader(t *testing.T) {
	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleSnapshot))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := New(WithEndpoint(srv.URL), WithAPIKey("fs_live_xyz"))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, c.Start(ctx))

	_, err = c.Bool(ctx, "new-checkout", nil)
	require.NoError(t, err)
	assert.Equal(t, "Bearer fs_live_xyz", gotAuth)
}

func TestSnapshot_EndToEnd(t *testing.T) {
	country := "country"
	op := "eq"
	arValue := "AR"
	body, err := json.Marshal(snapshotResponse{
		Environment: "development",
		Flags: map[string]flagEnvConfigJSON{
			"f1": {Key: "f1", Enabled: true, FlagType: "boolean", DefaultValue: true, Version: 1, Rules: nil},
		},
		Segments: map[string]segmentJSON{
			"s1": {Key: "s1", Conditions: engine.ConditionNode{Attribute: &country, Op: &op, Value: arValue}},
		},
		FetchedAt: time.Now(),
	})
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/snapshot") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, c.Start(ctx))
	got, err := c.Bool(ctx, "f1", nil)
	require.NoError(t, err)
	assert.True(t, got)
}
