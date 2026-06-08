package telemetry

import (
	"context"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Semantic convention attribute keys for feature flags (OTel feature-flag semconv).
const (
	FeatureFlagKey           = attribute.Key("feature_flag.key")
	FeatureFlagProviderName  = attribute.Key("feature_flag.provider_name")
	FeatureFlagResultReason  = attribute.Key("feature_flag.result.reason")
	FeatureFlagResultVariant = attribute.Key("feature_flag.result.variant")

	// Flagstone-specific attribute keys.
	FlagstoneEnvironment       = attribute.Key("flagstone.environment")
	FlagstoneEventType         = attribute.Key("flagstone.event.type")
	FlagstoneTargetingKey      = attribute.Key("flagstone.targeting_key")
	FlagstoneStatus            = attribute.Key("flagstone.status") // "success" | "error"
	FlagstoneEngineErrorType   = attribute.Key("flagstone.engine.error_type")
	FlagstoneEngineWarningType = attribute.Key("flagstone.engine.warning_type")
	FlagstoneAuthMethod        = attribute.Key("flagstone.auth.method") // "jwt" | "apikey"
	FlagstoneAuthErrorCode     = attribute.Key("flagstone.auth.error_code")
	FlagstoneAuthEndpoint      = attribute.Key("flagstone.auth.endpoint") // "login" | "refresh"
)

// WrapWithTracing wraps an http.Handler with otelhttp auto-instrumentation.
// Paths in skipPaths are passed through without tracing (useful for /healthz,
// /readyz, etc.).
func WrapWithTracing(next http.Handler, skipPaths ...string) http.Handler {
	skip := make(map[string]bool, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = true
	}

	return otelhttp.NewHandler(next, "flagstone",
		otelhttp.WithFilter(func(r *http.Request) bool {
			return !skip[r.URL.Path]
		}),
	)
}

// NewSpan creates a child span with the given name and attributes from the
// parent context. Returns the new context and the span.
func NewSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return trace.SpanFromContext(ctx).TracerProvider().
		Tracer("github.com/flagstonehq/flagstone").
		Start(ctx, name, trace.WithAttributes(attrs...))
}
