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
	mp                     metric.MeterProvider
	evaluationsTotal       metric.Int64Counter
	evaluationDuration     metric.Float64Histogram
	engineErrorsTotal      metric.Int64Counter
	engineWarningsTotal    metric.Int64Counter
	snapshotFetchDuration  metric.Float64Histogram
	snapshotFetchTotal     metric.Int64Counter
	sseConnectionsActive   metric.Int64UpDownCounter
	sseEventsPublished     metric.Int64Counter
	dbPoolConnectionsIdle  metric.Int64ObservableGauge
	dbPoolAcquiredTotal    metric.Int64ObservableCounter
	dbPoolAcquireWaitTotal metric.Float64ObservableCounter
}

// DBPoolStats is a backend-agnostic snapshot of pgxpool.Stat() values the
// telemetry layer reports. The caller maps pgxpool.Stat() into this struct so
// the telemetry package stays free of a pgx dependency.
type DBPoolStats struct {
	IdleConns          int64
	AcquiredTotal      int64   // cumulative count of successful acquires
	AcquireWaitSeconds float64 // cumulative time spent waiting on acquires
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

	dbIdle, err := meter.Int64ObservableGauge(
		"flagstone.db.pool.connections.idle",
		metric.WithDescription("Current number of idle DB connections in the pool"),
	)
	if err != nil {
		return nil, fmt.Errorf("flagstone.db.pool.connections.idle: %w", err)
	}

	dbAcq, err := meter.Int64ObservableCounter(
		"flagstone.db.pool.connections.acquired.total",
		metric.WithDescription("Cumulative number of DB connections acquired from the pool"),
	)
	if err != nil {
		return nil, fmt.Errorf("flagstone.db.pool.connections.acquired.total: %w", err)
	}

	dbWaitTotal, err := meter.Float64ObservableCounter(
		"flagstone.db.pool.acquire.wait.duration",
		metric.WithDescription("Cumulative time spent waiting for a DB connection from the pool"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("flagstone.db.pool.acquire.wait.duration: %w", err)
	}

	evalDur, err := meter.Float64Histogram(
		"flagstone.evaluation.duration",
		metric.WithDescription("Duration of flag evaluations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create evaluation.duration histogram: %w", err)
	}

	engineErr, err := meter.Int64Counter(
		"flagstone.engine.errors.total",
		metric.WithDescription("Total number of engine errors, by type"),
	)
	if err != nil {
		return nil, fmt.Errorf("create engine.errors total counter: %w", err)
	}

	engineWarn, err := meter.Int64Counter(
		"flagstone.engine.warnings.total",
		metric.WithDescription("Total number of engine warnings, by type"),
	)
	if err != nil {
		return nil, fmt.Errorf("create engine.warnings total counter: %w", err)
	}

	return &Metrics{
		mp:                     mp,
		evaluationsTotal:       evalTotal,
		evaluationDuration:     evalDur,
		engineErrorsTotal:      engineErr,
		engineWarningsTotal:    engineWarn,
		snapshotFetchDuration:  snapDur,
		snapshotFetchTotal:     snapTotal,
		sseConnectionsActive:   sseActive,
		sseEventsPublished:     ssePub,
		dbPoolConnectionsIdle:  dbIdle,
		dbPoolAcquiredTotal:    dbAcq,
		dbPoolAcquireWaitTotal: dbWaitTotal,
	}, nil
}

// RecordEvaluation increments the evaluation counter with the given attributes.
func (m *Metrics) RecordEvaluation(ctx context.Context, attrs ...attribute.KeyValue) {
	if m == nil || m.evaluationsTotal == nil {
		return
	}
	m.evaluationsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordEvaluationDuration records the latency of a single flag evaluation.
func (m *Metrics) RecordEvaluationDuration(ctx context.Context, duration time.Duration, attrs ...attribute.KeyValue) {
	if m == nil || m.evaluationDuration == nil {
		return
	}
	m.evaluationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordEngineError increments the engine error counter by type.
func (m *Metrics) RecordEngineError(ctx context.Context, errType string, attrs ...attribute.KeyValue) {
	if m == nil || m.engineErrorsTotal == nil {
		return
	}
	m.engineErrorsTotal.Add(ctx, 1, metric.WithAttributes(append(attrs, FlagstoneEngineErrorType.String(errType))...))
}

// RecordEngineWarning increments the engine warning counter by type.
func (m *Metrics) RecordEngineWarning(ctx context.Context, warnType string, attrs ...attribute.KeyValue) {
	if m == nil || m.engineWarningsTotal == nil {
		return
	}
	m.engineWarningsTotal.Add(ctx, 1, metric.WithAttributes(append(attrs, FlagstoneEngineWarningType.String(warnType))...))
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

// RegisterDBPoolStats registers a single observable callback that reports all
// DB pool metrics (idle connections, cumulative acquires, cumulative acquire
// wait). The callback reads a DBPoolStats snapshot — typically mapped from
// pgxpool.Stat() — on each collection cycle.
func (m *Metrics) RegisterDBPoolStats(statFn func() DBPoolStats) error {
	if m == nil || m.dbPoolConnectionsIdle == nil {
		return nil
	}
	_, err := m.mp.Meter("github.com/flagstonehq/flagstone").RegisterCallback(
		func(_ context.Context, obs metric.Observer) error {
			s := statFn()
			obs.ObserveInt64(m.dbPoolConnectionsIdle, s.IdleConns)
			obs.ObserveInt64(m.dbPoolAcquiredTotal, s.AcquiredTotal)
			obs.ObserveFloat64(m.dbPoolAcquireWaitTotal, s.AcquireWaitSeconds)
			return nil
		},
		m.dbPoolConnectionsIdle,
		m.dbPoolAcquiredTotal,
		m.dbPoolAcquireWaitTotal,
	)
	return err
}
