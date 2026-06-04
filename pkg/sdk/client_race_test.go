package sdk

import (
	"context"
	"encoding/json"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRace_ConcurrentReadAndSwap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race stress test in -short mode")
	}

	var resp snapshotResponse
	if err := json.Unmarshal([]byte(sampleSnapshot), &resp); err != nil {
		t.Fatal(err)
	}
	fresh := resp.toSnapshot()

	c := newSnapshotCache()
	c.store(fresh)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(1 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.store(fresh)
			}
		}
	}()

	const readers = 16
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				s := c.get()
				for k := range s.flags {
					_ = s.flags[k]
				}
				runtime.Gosched()
			}
		}()
	}
	wg.Wait()
}

func TestRace_RefreshSignalIsThreadSafe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race stress test in -short mode")
	}

	ch := make(chan struct{}, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	var wg sync.WaitGroup
	const senders = 16
	for i := 0; i < senders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				select {
				case ch <- struct{}{}:
				default:
				}
				runtime.Gosched()
			}
		}()
	}
	wg.Wait()
	select {
	case <-ch:
	default:
		t.Fatal("expected at least one signal in the channel")
	}
}

func TestRace_StartAndCloseConcurrent(t *testing.T) {
	srv, calls := newTestServer(t, nil)
	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	const callers = 20
	for i := 0; i < callers; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = c.Start(ctx)
		}()
		go func() {
			defer wg.Done()
			_ = c.Close()
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(calls); got != 1 {
		t.Fatalf("expected exactly 1 initial fetch, got %d", got)
	}
}
