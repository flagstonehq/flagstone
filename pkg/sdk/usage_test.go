package sdk

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageTracker_RecordAccumulates(t *testing.T) {
	tr := &usageTracker{
		entries:  make(map[string]*usageEntry),
		interval: time.Hour,
		stopCh:   make(chan struct{}),
	}

	tr.record("flag-a", "RULE_MATCH")
	tr.record("flag-a", "DEFAULT")
	tr.record("flag-b", "DEFAULT")

	assert.Equal(t, int64(2), tr.entries["flag-a"].Count)
	assert.Equal(t, "DEFAULT", tr.entries["flag-a"].LastReason, "last reason should win")
	assert.Equal(t, int64(1), tr.entries["flag-b"].Count)
}

func TestUsageTracker_FlushSendsPayloadAndResets(t *testing.T) {
	var received []usageEntry
	var calls atomic.Int32

	usageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var report usageReport
		require.NoError(t, json.Unmarshal(body, &report))
		received = report.Entries
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(usageSrv.Close)

	snapshotSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleSnapshot))
	}))
	t.Cleanup(snapshotSrv.Close)

	c, err := New(
		WithEndpoint(usageSrv.URL),
		WithAPIKey("test-key"),
		WithUsageReportingInterval(time.Hour),
	)
	c.opts.endpoint = usageSrv.URL
	require.NoError(t, err)

	c.usageTracker.record("new-checkout", "DEFAULT")
	c.usageTracker.record("new-checkout", "RULE_MATCH")
	c.usageTracker.record("dark-mode", "DISABLED")

	c.usageTracker.flush(context.Background())

	require.Equal(t, int32(1), calls.Load(), "flush should POST exactly once")
	require.Len(t, received, 2)

	byKey := make(map[string]usageEntry, len(received))
	for _, e := range received {
		byKey[e.FlagKey] = e
	}
	assert.Equal(t, int64(2), byKey["new-checkout"].Count)
	assert.Equal(t, "RULE_MATCH", byKey["new-checkout"].LastReason)
	assert.Equal(t, int64(1), byKey["dark-mode"].Count)

	c.usageTracker.flush(context.Background())
	assert.Equal(t, int32(1), calls.Load(), "second flush of empty tracker must not POST")
}

func TestUsageReporting_EndToEnd(t *testing.T) {
	var usageCalls atomic.Int32
	var reportMu sync.Mutex
	var lastReport usageReport

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleSnapshot))
	})
	mux.HandleFunc("/api/v1/sdk/usage", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var report usageReport
		_ = json.Unmarshal(body, &report)
		reportMu.Lock()
		lastReport = report
		reportMu.Unlock()
		usageCalls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := New(
		WithEndpoint(srv.URL),
		WithAPIKey("k"),
		WithUsageReportingInterval(30*time.Millisecond),
	)
	require.NoError(t, err)
	require.NoError(t, c.Start(context.Background()))

	ctx := context.Background()
	c.BoolDetail(ctx, "new-checkout", false, nil)
	c.BoolDetail(ctx, "new-checkout", false, nil)
	c.StringDetail(ctx, "banner-text", "", nil)

	assert.Eventually(t, func() bool {
		return usageCalls.Load() >= 1
	}, 500*time.Millisecond, 10*time.Millisecond)

	require.NoError(t, c.Close())

	reportMu.Lock()
	entries := lastReport.Entries
	reportMu.Unlock()

	byKey := make(map[string]usageEntry)
	for _, e := range entries {
		byKey[e.FlagKey] = e
	}
	assert.GreaterOrEqual(t, byKey["new-checkout"].Count, int64(2))
	assert.Equal(t, int64(1), byKey["banner-text"].Count)
}
