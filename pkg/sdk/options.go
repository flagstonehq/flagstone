package sdk

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

// Option configures a Client.
type Option func(*clientOptions)

type clientOptions struct {
	endpoint       string
	apiKey         string
	cacheTTL       time.Duration
	logger         *zap.Logger
	httpClient     *http.Client
	offline        bool
	bootstrap      []byte
	bootstrapErr   error
	onStatusChange []func(State)
	store          DataStore
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

// WithOffline puts the client in offline mode: New succeeds without an
// endpoint, Start is a no-op, and evaluations always return the supplied
// default with ReasonDefault. Useful for tests and SSR.
func WithOffline(v bool) Option {
	return func(o *clientOptions) { o.offline = v }
}

// WithBootstrap pre-loads the client with a raw snapshot JSON (the exact
// body returned by GET /api/v1/sdk/snapshot). The client can serve
// evaluations immediately, before Start is called or the network is
// reached.
func WithBootstrap(json []byte) Option {
	return func(o *clientOptions) { o.bootstrap = json }
}

// WithBootstrapReader is the io.Reader equivalent of WithBootstrap.
func WithBootstrapReader(r io.Reader) Option {
	return func(o *clientOptions) {
		data, err := io.ReadAll(r)
		if err != nil {
			o.bootstrapErr = fmt.Errorf("sdk: WithBootstrapReader: %w", err)
			return
		}
		o.bootstrap = data
	}
}

// WithBootstrapFile loads bootstrap data from path. A missing file is a
// warning, not a fatal error — the SDK starts and tries the network.
func WithBootstrapFile(path string) Option {
	return func(o *clientOptions) {
		data, err := os.ReadFile(path) //nolint:gosec // path is caller-supplied by design
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Missing file is a warning, not fatal: the SDK starts and
				// tries the network. We can't log here — the logger is set
				// by defaults(), which runs after options are applied.
				return
			}
			o.bootstrapErr = fmt.Errorf("sdk: read bootstrap file %q: %w", path, err)
			return
		}
		o.bootstrap = data
	}
}

// WithOnStatusChange registers a callback that fires every time the
// client's connectivity state changes. Multiple callbacks are supported.
func WithOnStatusChange(cb func(State)) Option {
	return func(o *clientOptions) {
		o.onStatusChange = append(o.onStatusChange, cb)
	}
}

// WithDataStore sets a persistent DataStore. When set, the SDK loads
// the snapshot from the store on startup (in Start, before the first
// network fetch) and saves every successful fetch result to it.
// If unset, no persistence layer runs and behavior is identical to
// previous versions.
func WithDataStore(ds DataStore) Option {
	return func(o *clientOptions) {
		o.store = ds
	}
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
	if o.offline {
		return nil
	}
	if o.endpoint == "" {
		return errors.New("sdk: WithEndpoint is required")
	}
	if o.apiKey == "" {
		return errors.New("sdk: WithAPIKey is required")
	}
	return nil
}
