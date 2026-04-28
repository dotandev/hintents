// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"fmt"
	"os"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
)

// ClientOption is a functional option for configuring a Client.
// It follows the idiomatic Go functional options pattern, allowing flexible
// and elegant configuration without exposing internal builder state.
// Options like WithNetwork and WithToken are evaluated during NewClient,
// and validation is deferred until all options are applied.
type ClientOption func(*clientBuilder)

type clientBuilder struct {
	network          Network
	token            string
	horizonURL       string
	sorobanURL       string
	altURLs          []string
	cacheEnabled     bool
	methodTelemetry  MethodTelemetry
	config           *NetworkConfig
	httpClient       HTTPClient
	requestTimeout   time.Duration
	middlewares      []Middleware
	loggingEnabled   bool
	failureThreshold int
	retryTimeout     int
}

const defaultHTTPTimeout = 15 * time.Second

func newBuilder() *clientBuilder {
	return &clientBuilder{
		network:          Mainnet,
		cacheEnabled:     true,
		methodTelemetry:  defaultMethodTelemetry(),
		requestTimeout:   defaultHTTPTimeout,
		failureThreshold: 5,
		retryTimeout:     60,
	}
}

func WithNetwork(net Network) ClientOption {
	return func(b *clientBuilder) {
		if net == "" {
			net = Mainnet
		}
		b.network = net
	}
}

func WithToken(token string) ClientOption {
	return func(b *clientBuilder) {
		b.token = token
	}
}

func WithHorizonURL(url string) ClientOption {
	return func(b *clientBuilder) {
		b.horizonURL = url
		b.altURLs = []string{url}
	}
}

func WithAltURLs(urls []string) ClientOption {
	return func(b *clientBuilder) {
		if len(urls) > 0 {
			b.altURLs = urls
			b.horizonURL = urls[0]
		}
	}
}

func WithSorobanURL(url string) ClientOption {
	return func(b *clientBuilder) {
		b.sorobanURL = url
	}
}

func WithNetworkConfig(cfg NetworkConfig) ClientOption {
	return func(b *clientBuilder) {
		b.config = &cfg
		b.network = Network(cfg.Name)
		b.horizonURL = cfg.HorizonURL
		b.sorobanURL = cfg.SorobanRPCURL
	}
}

func WithCacheEnabled(enabled bool) ClientOption {
	return func(b *clientBuilder) {
		b.cacheEnabled = enabled
	}
}

// WithRequestTimeout sets a custom HTTP request timeout for all RPC calls.
// Use this to override the default 15-second timeout, for example on slow connections.
// A value of 0 disables the timeout (not recommended for production use).
func WithRequestTimeout(d time.Duration) ClientOption {
	return func(b *clientBuilder) {
		b.requestTimeout = d
	}
}

func WithHTTPClient(client HTTPClient) ClientOption {
	return func(b *clientBuilder) {
		b.httpClient = client
	}
}

// WithMethodTelemetry injects an optional telemetry hook for SDK method timings.
// If nil is provided, a no-op implementation is used.
func WithMethodTelemetry(telemetry MethodTelemetry) ClientOption {
	return func(b *clientBuilder) {
		if telemetry == nil {
			telemetry = defaultMethodTelemetry()
		}
		b.methodTelemetry = telemetry
	}
}

func WithMiddleware(middlewares ...Middleware) ClientOption {
	return func(b *clientBuilder) {
		b.middlewares = append(b.middlewares, middlewares...)
	}
}

// WithLoggingEnabled enables or disables the built-in LoggingMiddleware.
// When enabled, every outbound HTTP request is logged at INFO level with its
// method, URL, response status, and round-trip latency. The logging middleware
// is always placed outermost so it observes the full logical request duration.
func WithLoggingEnabled(enabled bool) ClientOption {
	return func(b *clientBuilder) {
		b.loggingEnabled = enabled
	}
}

// WithCircuitBreakerThreshold sets the number of failures before the circuit breaker opens.
func WithCircuitBreakerThreshold(threshold int) ClientOption {
	return func(b *clientBuilder) {
		if threshold > 0 {
			b.failureThreshold = threshold
		}
	}
}

// WithCircuitBreakerTimeout sets the duration in seconds to wait before retrying a failed endpoint.
func WithCircuitBreakerTimeout(timeout int) ClientOption {
	return func(b *clientBuilder) {
		if timeout > 0 {
			b.retryTimeout = timeout
		}
	}
}

func NewClient(opts ...ClientOption) (*Client, error) {
	builder := newBuilder()

	if builder.token == "" {
		builder.token = os.Getenv("ERST_RPC_TOKEN")
	}

	for _, opt := range opts {
		opt(builder)
	}

	if err := builder.validate(); err != nil {
		return nil, err
	}

	return builder.build()
}

func (b *clientBuilder) validate() error {
	if b.network == "" {
		b.network = Mainnet
	}

	if b.config != nil {
		if err := ValidateNetworkConfig(*b.config); err != nil {
			return errors.WrapValidationError(fmt.Sprintf("invalid network config: %v", err))
		}
	}

	if b.horizonURL != "" {
		if err := isValidURL(b.horizonURL); err != nil {
			return errors.WrapValidationError(fmt.Sprintf("invalid HorizonURL: %v", err))
		}
	}
	for _, url := range b.altURLs {
		if err := isValidURL(url); err != nil {
			return errors.WrapValidationError(fmt.Sprintf("invalid URL in altURLs: %v", err))
		}
	}
	if b.sorobanURL != "" {
		if err := isValidURL(b.sorobanURL); err != nil {
			return errors.WrapValidationError(fmt.Sprintf("invalid SorobanURL: %v", err))
		}
	}

	if b.horizonURL == "" && b.sorobanURL == "" {
		b.horizonURL = b.getDefaultHorizonURL(b.network)
	}

	return nil
}

func (b *clientBuilder) getDefaultHorizonURL(net Network) string {
	switch net {
	case Testnet:
		return TestnetHorizonURL
	case Futurenet:
		return FuturenetHorizonURL
	default:
		return MainnetHorizonURL
	}
}

func (b *clientBuilder) getDefaultSorobanURL(net Network) string {
	switch net {
	case Testnet:
		return TestnetSorobanURL
	case Futurenet:
		return FuturenetSorobanURL
	default:
		return MainnetSorobanURL
	}
}

func (b *clientBuilder) getConfig(net Network) NetworkConfig {
	switch net {
	case Testnet:
		return TestnetConfig
	case Futurenet:
		return FuturenetConfig
	default:
		return MainnetConfig
	}
}

func (b *clientBuilder) build() (*Client, error) {
	if b.sorobanURL == "" {
		b.sorobanURL = b.getDefaultSorobanURL(b.network)
	}

	if b.config == nil {
		cfg := b.getConfig(b.network)
		b.config = &cfg
	}

	if b.horizonURL == "" {
		b.horizonURL = b.config.HorizonURL
	}

	if len(b.altURLs) == 0 {
		b.altURLs = []string{b.horizonURL}
	}

	if b.httpClient == nil {
		mws := b.middlewares
		if b.loggingEnabled {
			// Prepend so the logging middleware is outermost in the chain,
			// ensuring it captures the full round-trip including all user middlewares.
			mws = append([]Middleware{NewLoggingMiddleware()}, mws...)
		}
		b.httpClient = createHTTPClient(b.token, b.requestTimeout, mws...)
	}

	return &Client{
		HorizonURL: b.horizonURL,
		Horizon: &horizonclient.Client{
			HorizonURL: b.horizonURL,
			HTTP:       b.httpClient,
		},
		Network:          b.network,
		SorobanURL:       b.sorobanURL,
		AltURLs:          b.altURLs,
		httpClient:       b.httpClient,
		token:            b.token,
		Config:           *b.config,
		CacheEnabled:     b.cacheEnabled,
		methodTelemetry:  b.methodTelemetry,
		failures:         make(map[string]int),
		lastFailure:      make(map[string]time.Time),
		FailureThreshold: b.failureThreshold,
		RetryTimeout:     b.retryTimeout,
		middlewares:      b.middlewares,
		healthCollector:  NewHealthCollector(),
	}, nil
}
