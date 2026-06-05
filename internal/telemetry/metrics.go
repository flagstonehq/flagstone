package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds all Flagstone-specific metric instruments.
// A nil *Metrics is safe to use (all methods become no-ops)
// so tests that don't need observability can pass nil.
type Metrics struct {
	mp                    metric.MeterProvider
	evaluationsTotal      metric.Int64Counter
	snapshotFetchDuration metric.Float64Histogram
	snapshotFetchTotal    metric.Int64Counter
	sseConnectionsActive  metric.Int64UpDownCounter
	sseEventsPublished    metric.Int64Counter
	dbPoolAcquireWaitDur  metric.Float64Histogram
	dbPoolConnectionsIdle metric.Int64ObservableGauge
	dbPoolConnectionsAcq  metric.Int64UpDownCounter
}

// NewMetrics creates all Flagstone metric instruments from the given
// MeterProvider. Callers should pass the result of otel.GetMeterProvider()
// (or a no-op provider).
func NewMetrics(mp metric.MeterProvider) (*Metrics, error) {
	meter := mp.Meter("github.com/flagstonehq/flagstone")

	evalTotal, err := meter.Int64Counter(
		"flagstone.evaluations.total",
		metric.WithDescription("Total number of flag evaluations"),
	)
	if err != nil {
		return nil, fmt.Errorf("flagstone.evaluations.total: %w", err)
	}

	snapDur, err := meter.Float64Histogram(
		"flagstone.snapshot.fetch.duration",
		metric.WithDescription("Duration of snapshot fetch operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("flagstone.snapshot.fetch.duration: %w", err)
	}

	snapTotal, err := meter.Int64Counter(
		"flagstone.snapshot.fetch.total",
		metric.WithDescription("Total number of snapshot fetch operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("flagstone.snapshot.fetch.total: %w", err)
	}

	sseActive, err := meter.Int64UpDownCounter(
		"flagstone.sse.connections.active",
		metric.WithDescription("Current number of active SSE connections"),
	)
	if err != nil {
		return nil, fmt.Errorf("flagstone.sse.connections.active: %w", err)
	}

	ssePub, err := meter.Int64Counter(
		"flagstone.sse.events.published.total",
		metric.WithDescription("Total number of SSE events published"),
	)
	if err != nil {
		return nil, fmt.Errorf("flagstone.sse.events.published.total: %w", err)
	}

	dbWaitDur, err := meter.Float64Histogram(
		"flagstone.db.pool.acquire.wait.duration",
		metric.WithDescription("Time spent waiting for a DB connection from the pool"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("flagstone.db.pool.acquire.wait.duration: %w", err)
	}

	dbIdle, err := meter.Int64ObservableGauge(
		"flagstone.db.pool.connections.idle",
		metric.WithDescription("Current number of idle DB connections in the pool"),
	)
	if err != nil {
		return nil, fmt.Errorf("flagstone.db.pool.connections.idle: %w", err)
	}

	dbAcq, err := meter.Int64UpDownCounter(
		"flagstone.db.pool.connections.acquired.total",
		metric.WithDescription("Total number of DB connections acquired from the pool"),
	)
	if err != nil {
		return nil, fmt.Errorf("flagstone.db.pool.connections.acquired.total: %w", err)
	}

	return &Metrics{
		mp:                    mp,
		evaluationsTotal:      evalTotal,
		snapshotFetchDuration: snapDur,
		snapshotFetchTotal:    snapTotal,
		sseConnectionsActive:  sseActive,
		sseEventsPublished:    ssePub,
		dbPoolAcquireWaitDur:  dbWaitDur,
		dbPoolConnectionsIdle: dbIdle,
		dbPoolConnectionsAcq:  dbAcq,
	}, nil
}

// RecordEvaluation increments the evaluation counter with the given attributes.
func (m *Metrics) RecordEvaluation(ctx context.Context, attrs ...attribute.KeyValue) {
	if m == nil || m.evaluationsTotal == nil {
		return
	}
	m.evaluationsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordSnapshotFetch records the duration and increments the count of snapshot fetches.
func (m *Metrics) RecordSnapshotFetch(ctx context.Context, duration time.Duration, attrs ...attribute.KeyValue) {
	if m == nil || m.snapshotFetchDuration == nil {
		return
	}
	m.snapshotFetchDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.snapshotFetchTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// AddSSEConnectionDelta adjusts the active SSE connection count by delta (+1 or -1).
func (m *Metrics) AddSSEConnectionDelta(ctx context.Context, delta int64) {
	if m == nil || m.sseConnectionsActive == nil {
		return
	}
	m.sseConnectionsActive.Add(ctx, delta)
}

// RecordSSEEvent increments the published events counter.
func (m *Metrics) RecordSSEEvent(ctx context.Context, eventType string) {
	if m == nil || m.sseEventsPublished == nil {
		return
	}
	m.sseEventsPublished.Add(ctx, 1,
		metric.WithAttributes(FlagstoneEventType.String(eventType)),
	)
}

// RecordDBPoolAcquireWait records the pool acquire wait duration.
func (m *Metrics) RecordDBPoolAcquireWait(ctx context.Context, duration time.Duration) {
	if m == nil || m.dbPoolAcquireWaitDur == nil {
		return
	}
	m.dbPoolAcquireWaitDur.Record(ctx, duration.Seconds())
}

// RegisterDBPoolGauge registers an observable callback that reports idle
// connections. The callback reads from the pgxpool.Stat() result.
func (m *Metrics) RegisterDBPoolGauge(_ context.Context, statFn func() int64) error {
	if m == nil || m.dbPoolConnectionsIdle == nil {
		return nil
	}
	_, err := m.mp.Meter("github.com/flagstonehq/flagstone").RegisterCallback(
		func(_ context.Context, obs metric.Observer) error {
			obs.ObserveInt64(m.dbPoolConnectionsIdle, statFn())
			return nil
		},
		m.dbPoolConnectionsIdle,
	)
	return err
}

// AddDBConnectionAcquired increments the acquired connections counter.
func (m *Metrics) AddDBConnectionAcquired(ctx context.Context, delta int64) {
	if m == nil || m.dbPoolConnectionsAcq == nil {
		return
	}
	m.dbPoolConnectionsAcq.Add(ctx, delta)
}
