package sdk

import (
	"bufio"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flagstonehq/flagstone/pkg/engine"
)

func buildBenchSnapshot(n int) *snapshot {
	s := newSnapshot()
	for i := 0; i < n; i++ {
		key := "flag_" + strconv.Itoa(i)
		s.flags[key] = engine.FlagConfig{
			Key:          key,
			Enabled:      true,
			FlagType:     "boolean",
			DefaultValue: false,
			Version:      1,
		}
	}
	return s
}

func BenchmarkBool_Cached(b *testing.B) {
	srv, _ := newTestServer(b, nil)
	c, _ := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	c.cache.store(buildBenchSnapshot(1))
	evalCtx := map[string]any{"user_id": "42"}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = c.Bool(context.Background(), "flag_0", evalCtx)
		}
	})
}

func BenchmarkBool_Cold(b *testing.B) {
	srv, _ := newTestServer(b, nil)
	evalCtx := map[string]any{"user_id": "42"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, _ := New(WithEndpoint(srv.URL), WithAPIKey("k"))
		_, _ = c.Bool(context.Background(), "new-checkout", evalCtx)
	}
}

func BenchmarkAll_100Flags(b *testing.B) {
	srv, _ := newTestServer(b, nil)
	c, _ := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	c.cache.store(buildBenchSnapshot(100))
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.All(ctx, map[string]any{"user_id": "42"})
	}
}

func BenchmarkAll_Parallel(b *testing.B) {
	srv, _ := newTestServer(b, nil)
	c, _ := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	c.cache.store(buildBenchSnapshot(100))
	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = c.All(ctx, map[string]any{"user_id": "42"})
		}
	})
}

func BenchmarkInvalidate_ConcurrentReaders(b *testing.B) {
	srv, _ := newTestServer(b, nil)
	c, _ := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	c.cache.store(buildBenchSnapshot(1))
	evalCtx := map[string]any{"user_id": "42"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	snap := buildBenchSnapshot(1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(100 * time.Microsecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.cache.store(snap)
			}
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = c.Bool(context.Background(), "flag_0", evalCtx)
		}
	})
	b.StopTimer()
	cancel()
	wg.Wait()
}

func BenchmarkSSE_Parse(b *testing.B) {
	const event = "id: 12345\nevent: flag_change\ndata: {\"key\":\"new-checkout\",\"action\":\"toggled\"}\n\n"
	input := strings.Repeat(event, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := strings.NewReader(input)
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		var ev streamEvent
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case line == "":
				_ = ev
				ev = streamEvent{}
			case strings.HasPrefix(line, "id:"):
				ev.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			case strings.HasPrefix(line, "event:"):
				ev.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				_ = json.Unmarshal([]byte(data), &ev.Data)
			}
		}
	}
}
