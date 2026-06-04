package sdk

import (
	"errors"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Option configures a Client.
type Option func(*clientOptions)

type clientOptions struct {
	endpoint   string
	apiKey     string
	cacheTTL   time.Duration
	logger     *zap.Logger
	httpClient *http.Client
}

const (
	defaultCacheTTL = 30 * time.Second
	defaultTimeout  = 10 * time.Second
)

// WithEndpoint sets the Flagstone API base URL.
func WithEndpoint(v string) Option {
	return func(o *clientOptions) {
		o.endpoint = v
	}
}

// WithAPIKey sets the SDK API key.
func WithAPIKey(v string) Option {
	return func(o *clientOptions) { o.apiKey = v }
}

// WithCacheTTL sets the snapshot TTL. Default: 30s.
func WithCacheTTL(d time.Duration) Option {
	return func(o *clientOptions) { o.cacheTTL = d }
}

// WithLogger sets a custom zap logger. If unset, a no-op logger is used.
func WithLogger(l *zap.Logger) Option {
	return func(o *clientOptions) { o.logger = l }
}

// WithHTTPClient sets a custom HTTP client (useful for tests and tracing).
func WithHTTPClient(c *http.Client) Option {
	return func(o *clientOptions) { o.httpClient = c }
}

func (o *clientOptions) defaults() {
	if o.cacheTTL == 0 {
		o.cacheTTL = defaultCacheTTL
	}
	if o.logger == nil {
		o.logger = zap.NewNop()
	}
	if o.httpClient == nil {
		o.httpClient = &http.Client{Timeout: defaultTimeout}
	}
}

func (o *clientOptions) validate() error {
	if o.endpoint == "" {
		return errors.New("sdk: WithEndpoint is required")
	}
	if o.apiKey == "" {
		return errors.New("sdk: WithAPIKey is required")
	}
	return nil
}
