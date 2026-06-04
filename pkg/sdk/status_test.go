package sdk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatus_InitializingBeforeStart(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)
	assert.Equal(t, StateInitializing, c.Status().State())
	assert.False(t, c.Status().Initialized())
}

func TestStatus_ConnectedAfterFetch(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)
	require.NoError(t, c.Start(context.Background()))
	assert.Equal(t, StateConnected, c.Status().State())
	assert.True(t, c.Status().Initialized())
	assert.Nil(t, c.Status().LastError())
	assert.False(t, c.Status().LastUpdated().IsZero())
}

func TestStatus_ErrorOnFailedFetch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)
	_ = c.Start(context.Background())
	assert.Equal(t, StateError, c.Status().State())
	assert.True(t, c.Status().Initialized(), "Error is still a known state")
	assert.NotNil(t, c.Status().LastError())
}

func TestStatus_StaleAfterRefreshFail(t *testing.T) {
	var calls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(sampleSnapshot))
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/v1/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("id: 1\nevent: flag_change\ndata: {\"key\":\"new-checkout\"}\n\n"))
		flusher.Flush()
		<-r.Context().Done()
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"), WithCacheTTL(24*time.Hour))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, c.Start(ctx))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.Status().State() == StateStale {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	assert.Equal(t, StateStale, c.Status().State())
	assert.NotNil(t, c.Status().LastError())
}

func TestStatus_Offline(t *testing.T) {
	c, err := New(WithOffline(true))
	require.NoError(t, err)
	assert.Equal(t, StateOffline, c.Status().State())
	assert.True(t, c.Status().Initialized())
	assert.Nil(t, c.Status().LastError())
}

func TestWaitForReady_Success(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)

	require.NoError(t, c.Start(context.Background()))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	assert.NoError(t, c.Status().WaitForReady(ctx))
}

func TestWaitForReady_Timeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)

	go func() { _ = c.Start(context.Background()) }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = c.Status().WaitForReady(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestOnStatusChange_FiresOnTransition(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	var states []State
	var mu int32

	c, err := New(
		WithEndpoint(srv.URL),
		WithAPIKey("k"),
		WithOnStatusChange(func(s State) {
			atomic.StoreInt32(&mu, int32(s))
			states = append(states, s)
		}),
	)
	require.NoError(t, err)
	require.NoError(t, c.Start(context.Background()))

	assert.Equal(t, int32(StateConnected), atomic.LoadInt32(&mu))
	assert.Contains(t, states, StateConnected)
}

func TestAddOnStatusChange_PostNew(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)

	var fired int32
	c.AddOnStatusChange(func(s State) {
		if s == StateConnected {
			atomic.StoreInt32(&fired, 1)
		}
	})

	require.NoError(t, c.Start(context.Background()))
	assert.Equal(t, int32(1), atomic.LoadInt32(&fired))
}

func TestStatus_Bootstrap_IsConnected(t *testing.T) {
	c, err := New(
		WithEndpoint("http://127.0.0.1:1"),
		WithAPIKey("k"),
		WithBootstrap([]byte(sampleSnapshot)),
	)
	require.NoError(t, err)
	assert.Equal(t, StateConnected, c.Status().State())
	assert.True(t, c.Status().Initialized())
}
