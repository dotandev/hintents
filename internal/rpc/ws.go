// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package rpc provides access to Stellar Horizon and Soroban RPC endpoints.
// ws.go implements incoming RPC WebSocket connection rate limiting and
// server primitives to protect against connection exhaustion attacks per IP address.
package rpc

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	interrors "github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// Default configuration parameters for the WebSocket rate limiter.
const (
	// DefaultMaxConcurrentWS is the default maximum concurrent active WebSocket
	// connections allowed from a single client IP address.
	DefaultMaxConcurrentWS = 10

	// DefaultWSBurstCapacity is the maximum burst capacity for the token bucket.
	DefaultWSBurstCapacity = 10.0

	// DefaultWSRefillRate is the rate at which tokens refill per second.
	DefaultWSRefillRate = 5.0

	// DefaultWSCleanupInterval is the period between eviction sweeps of idle IP entries.
	DefaultWSCleanupInterval = 5 * time.Minute

	// DefaultWSEntryTTL is the duration an idle IP entry (with 0 active connections)
	// is kept in memory before being evicted.
	DefaultWSEntryTTL = 10 * time.Minute
)

// Common rate limiting errors.
var (
	// ErrRateLimitExceeded indicates that the token bucket is exhausted for the client IP.
	ErrRateLimitExceeded = errors.New("rate limit exceeded: token bucket exhausted")

	// ErrMaxConcurrentConnectionsExceeded indicates that the client IP has reached its
	// maximum allowed concurrent active WebSocket connections.
	ErrMaxConcurrentConnectionsExceeded = errors.New("rate limit exceeded: max concurrent websocket connections reached")

	// ErrNotWebSocketHandshake indicates that an incoming HTTP request is not a valid
	// WebSocket upgrade request.
	ErrNotWebSocketHandshake = errors.New("websocket: not a websocket upgrade handshake")
)

// WSRateLimitError describes a rate limit or concurrency limit rejection with context.
type WSRateLimitError struct {
	IP            string
	Reason        string
	MaxConcurrent int
	Active        int
}

// Error implements the error interface.
func (e *WSRateLimitError) Error() string {
	return fmt.Sprintf("websocket connection rejected for IP %s: %s (active: %d, max: %d)",
		e.IP, e.Reason, e.Active, e.MaxConcurrent)
}

// Is reports whether the error matches ErrRateLimitExceeded or interrors.ErrRateLimitExceeded.
func (e *WSRateLimitError) Is(target error) bool {
	return target == ErrRateLimitExceeded ||
		target == ErrMaxConcurrentConnectionsExceeded ||
		target == interrors.ErrRateLimitExceeded
}

// ---------------------------------------------------------------------------
// TokenBucket
// ---------------------------------------------------------------------------

// TokenBucket implements a standard thread-safe token bucket rate limiting algorithm.
type TokenBucket struct {
	capacity   float64
	refillRate float64 // tokens per second
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucket creates a new TokenBucket initialized to full capacity.
func NewTokenBucket(capacity, refillRate float64) *TokenBucket {
	if capacity < 0 {
		capacity = 0
	}
	if refillRate < 0 {
		refillRate = 0
	}
	return &TokenBucket{
		capacity:   capacity,
		refillRate: refillRate,
		tokens:     capacity,
		lastRefill: time.Now(),
	}
}

// refillLocked updates the available token count based on time elapsed since lastRefill.
// Must be called while holding tb.mu.
func (tb *TokenBucket) refillLocked(now time.Time) {
	if tb.lastRefill.IsZero() {
		tb.lastRefill = now
		return
	}
	elapsed := now.Sub(tb.lastRefill).Seconds()
	if elapsed <= 0 {
		return
	}
	tb.tokens = math.Min(tb.capacity, tb.tokens+elapsed*tb.refillRate)
	tb.lastRefill = now
}

// Allow attempts to consume 1 token from the bucket.
// Returns true if a token was consumed, or false if the bucket was empty.
func (tb *TokenBucket) Allow() bool {
	return tb.AllowN(1.0)
}

// AllowN attempts to consume n tokens from the bucket.
// Returns true if n tokens were available and consumed, or false otherwise.
func (tb *TokenBucket) AllowN(n float64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refillLocked(time.Now())

	if tb.tokens >= n {
		tb.tokens -= n
		return true
	}
	return false
}

// Tokens returns the current number of available tokens after refilling to the present moment.
func (tb *TokenBucket) Tokens() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refillLocked(time.Now())
	return tb.tokens
}

// Capacity returns the maximum capacity of the bucket.
func (tb *TokenBucket) Capacity() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.capacity
}

// RefillRate returns the refill rate in tokens per second.
func (tb *TokenBucket) RefillRate() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.refillRate
}

// Reset refills the bucket to full capacity and resets the timestamp.
func (tb *TokenBucket) Reset() {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.tokens = tb.capacity
	tb.lastRefill = time.Now()
}

// ---------------------------------------------------------------------------
// WSRateLimiter
// ---------------------------------------------------------------------------

// WSRateLimiterConfig holds configuration settings for WSRateLimiter.
type WSRateLimiterConfig struct {
	// MaxConcurrent is the maximum allowed concurrent active WebSocket connections
	// for any single client IP address. If <= 0, concurrent connections are unlimited.
	MaxConcurrent int

	// Capacity is the burst capacity of the token bucket per client IP.
	Capacity float64

	// RefillRate is the rate at which tokens replenish (tokens per second) per client IP.
	RefillRate float64

	// CleanupInterval specifies how frequently idle IP entries are swept and evicted.
	// Defaults to DefaultWSCleanupInterval if <= 0.
	CleanupInterval time.Duration

	// EntryTTL specifies how long an idle entry (with 0 active connections) may remain
	// in memory before being evicted during cleanup sweeps.
	// Defaults to DefaultWSEntryTTL if <= 0.
	EntryTTL time.Duration
}

// DefaultWSRateLimiterConfig returns sensible production defaults for WSRateLimiterConfig.
func DefaultWSRateLimiterConfig() WSRateLimiterConfig {
	return WSRateLimiterConfig{
		MaxConcurrent:   DefaultMaxConcurrentWS,
		Capacity:        DefaultWSBurstCapacity,
		RefillRate:      DefaultWSRefillRate,
		CleanupInterval: DefaultWSCleanupInterval,
		EntryTTL:        DefaultWSEntryTTL,
	}
}

// wsIPEntry tracks the rate limiter and concurrent connection state for a single client IP.
type wsIPEntry struct {
	bucket            *TokenBucket
	activeConnections int
	lastSeen          time.Time
}

// WSRateLimiter restricts incoming WebSocket connections per client IP address.
// It combines a token-bucket rate limiter (to protect against rapid connection bursts)
// with a concurrent connection limiter (to protect against slow connection exhaustion attacks).
type WSRateLimiter struct {
	cfg       WSRateLimiterConfig
	entries   map[string]*wsIPEntry
	mu        sync.RWMutex
	stopCh    chan struct{}
	closeOnce sync.Once
}

// IPRateLimiter is an alias for WSRateLimiter.
type IPRateLimiter = WSRateLimiter

// NewWSRateLimiter creates and initializes a new WSRateLimiter.
func NewWSRateLimiter(cfg WSRateLimiterConfig) *WSRateLimiter {
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = DefaultWSCleanupInterval
	}
	if cfg.EntryTTL <= 0 {
		cfg.EntryTTL = DefaultWSEntryTTL
	}
	if cfg.Capacity <= 0 {
		cfg.Capacity = DefaultWSBurstCapacity
	}
	if cfg.RefillRate <= 0 {
		cfg.RefillRate = DefaultWSRefillRate
	}
	if cfg.MaxConcurrent < 0 {
		cfg.MaxConcurrent = 0
	}

	limiter := &WSRateLimiter{
		cfg:     cfg,
		entries: make(map[string]*wsIPEntry),
		stopCh:  make(chan struct{}),
	}

	go limiter.cleanupLoop()

	return limiter
}

// getOrCreateEntryLocked returns the wsIPEntry for the given IP, creating one if necessary.
// Must be called with l.mu held.
func (l *WSRateLimiter) getOrCreateEntryLocked(ip string, now time.Time) *wsIPEntry {
	entry, exists := l.entries[ip]
	if !exists {
		entry = &wsIPEntry{
			bucket:   NewTokenBucket(l.cfg.Capacity, l.cfg.RefillRate),
			lastSeen: now,
		}
		l.entries[ip] = entry
	}
	entry.lastSeen = now
	return entry
}

// Acquire requests permission to establish a new WebSocket connection for the given client IP.
//
// If successful, it consumes a token from the IP's token bucket, increments the active
// connection count, and returns an idempotent release function that MUST be invoked
// when the connection closes.
//
// If the IP has reached MaxConcurrent connections or the token bucket is exhausted,
// an appropriate error is returned.
func (l *WSRateLimiter) Acquire(ip string) (func(), error) {
	ip = NormalizeIP(ip)
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry := l.getOrCreateEntryLocked(ip, now)

	// 1. Check concurrent connection limit (connection exhaustion protection)
	if l.cfg.MaxConcurrent > 0 && entry.activeConnections >= l.cfg.MaxConcurrent {
		return nil, &WSRateLimitError{
			IP:            ip,
			Reason:        "max concurrent websocket connections reached",
			MaxConcurrent: l.cfg.MaxConcurrent,
			Active:        entry.activeConnections,
		}
	}

	// 2. Check token bucket rate limit (burst / connection storm protection)
	if !entry.bucket.Allow() {
		return nil, &WSRateLimitError{
			IP:            ip,
			Reason:        "token bucket exhausted",
			MaxConcurrent: l.cfg.MaxConcurrent,
			Active:        entry.activeConnections,
		}
	}

	// 3. Acquire connection slot
	entry.activeConnections++

	var once sync.Once
	release := func() {
		once.Do(func() {
			l.Release(ip)
		})
	}

	return release, nil
}

// TryAcquire is a non-blocking convenience method that returns whether a connection
// was successfully acquired, along with the corresponding release function.
func (l *WSRateLimiter) TryAcquire(ip string) (func(), bool) {
	release, err := l.Acquire(ip)
	if err != nil {
		return nil, false
	}
	return release, true
}

// Release decrements the active connection count for the given IP address.
func (l *WSRateLimiter) Release(ip string) {
	ip = NormalizeIP(ip)
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry, exists := l.entries[ip]; exists {
		if entry.activeConnections > 0 {
			entry.activeConnections--
		}
		entry.lastSeen = time.Now()
	}
}

// Allow reports whether a connection attempt from the given IP would currently be accepted
// without actually acquiring a connection slot or consuming a token.
func (l *WSRateLimiter) Allow(ip string) bool {
	ip = NormalizeIP(ip)
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry := l.getOrCreateEntryLocked(ip, now)

	if l.cfg.MaxConcurrent > 0 && entry.activeConnections >= l.cfg.MaxConcurrent {
		return false
	}

	return entry.bucket.Tokens() >= 1.0
}

// ActiveConnections returns the current number of active connections for the given IP.
func (l *WSRateLimiter) ActiveConnections(ip string) int {
	ip = NormalizeIP(ip)
	l.mu.RLock()
	defer l.mu.RUnlock()

	if entry, exists := l.entries[ip]; exists {
		return entry.activeConnections
	}
	return 0
}

// AvailableTokens returns the number of tokens currently in the bucket for the given IP.
func (l *WSRateLimiter) AvailableTokens(ip string) float64 {
	ip = NormalizeIP(ip)
	l.mu.RLock()
	defer l.mu.RUnlock()

	if entry, exists := l.entries[ip]; exists {
		return entry.bucket.Tokens()
	}
	return l.cfg.Capacity
}

// TotalActiveConnections returns the total number of active connections across all IPs.
func (l *WSRateLimiter) TotalActiveConnections() int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	total := 0
	for _, entry := range l.entries {
		total += entry.activeConnections
	}
	return total
}

// TotalTrackedIPs returns the number of unique IP addresses currently tracked in memory.
func (l *WSRateLimiter) TotalTrackedIPs() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// Reset clears all tracked IP entries.
func (l *WSRateLimiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make(map[string]*wsIPEntry)
}

// Close terminates the background cleanup goroutine.
func (l *WSRateLimiter) Close() {
	l.closeOnce.Do(func() {
		close(l.stopCh)
	})
}

// cleanupLoop periodically removes idle IP entries to prevent unbounded memory growth.
func (l *WSRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(l.cfg.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			now := time.Now()
			for ip, entry := range l.entries {
				if entry.activeConnections == 0 && now.Sub(entry.lastSeen) > l.cfg.EntryTTL {
					delete(l.entries, ip)
				}
			}
			l.mu.Unlock()
		case <-l.stopCh:
			return
		}
	}
}

// ---------------------------------------------------------------------------
// IP Extraction and Normalization
// ---------------------------------------------------------------------------

// ExtractIP extracts the client IP address from an incoming HTTP request.
// It inspects headers in the following priority:
// 1. X-Forwarded-For (leftmost IP)
// 2. X-Real-IP
// 3. r.RemoteAddr
//
// The returned IP is normalized and stripped of any port numbers.
func ExtractIP(r *http.Request) string {
	// 1. Check X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return NormalizeIP(ip)
			}
		}
	}

	// 2. Check X-Real-IP
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		ip := strings.TrimSpace(xrip)
		if ip != "" {
			return NormalizeIP(ip)
		}
	}

	// 3. Fall back to RemoteAddr
	return NormalizeIP(r.RemoteAddr)
}

// GetClientIP is an alias for ExtractIP.
func GetClientIP(r *http.Request) string {
	return ExtractIP(r)
}

// NormalizeIP cleans and formats an IP string, stripping port numbers and brackets.
func NormalizeIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "unknown"
	}

	// Try splitting host:port
	host, _, err := net.SplitHostPort(raw)
	if err == nil {
		raw = host
	}

	// Strip IPv6 brackets if present
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")

	// Validate and parse standard IP
	if parsed := net.ParseIP(raw); parsed != nil {
		return parsed.String()
	}

	return strings.ToLower(raw)
}

// ---------------------------------------------------------------------------
// WebSocket Connection & Server Primitives
// ---------------------------------------------------------------------------

// WSConn wraps a hijacked network connection with WebSocket framing capabilities
// and automatic rate limiter release upon closure.
type WSConn struct {
	Conn      net.Conn
	Reader    *bufio.Reader
	Writer    *bufio.Writer
	ClientIP  string
	releaseFn func()
	closed    int32
}

// ReadMessage reads a complete WebSocket text or binary message from the client.
func (c *WSConn) ReadMessage() ([]byte, error) {
	return wsReadMessage(c.Reader)
}

// WriteMessage sends an unmasked text frame to the client per RFC 6455 §5.1.
func (c *WSConn) WriteMessage(payload []byte) error {
	return wsWriteServerFrame(c.Writer, payload)
}

// Close closes the underlying network connection and releases the concurrency slot in the limiter.
func (c *WSConn) Close() error {
	if atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		if c.releaseFn != nil {
			c.releaseFn()
		}
		// Send close frame to peer (best effort)
		_ = wsWriteServerCloseFrame(c.Writer)
		return c.Conn.Close()
	}
	return nil
}

// RemoteAddr returns the remote network address of the connection.
func (c *WSConn) RemoteAddr() net.Addr {
	return c.Conn.RemoteAddr()
}

// wsWriteServerFrame writes an unmasked WebSocket frame to w.
func wsWriteServerFrame(w *bufio.Writer, payload []byte) error {
	var header [10]byte
	header[0] = 0x81 // FIN=1, opcode=0x1 (text)
	headerLen := 2

	pLen := len(payload)
	switch {
	case pLen < 126:
		header[1] = byte(pLen)
	case pLen <= 0xFFFF:
		header[1] = 126
		header[2] = byte(pLen >> 8)
		header[3] = byte(pLen)
		headerLen = 4
	default:
		header[1] = 127
		for i := 7; i >= 0; i-- {
			header[2+i] = byte(pLen & 0xFF)
			pLen >>= 8
		}
		headerLen = 10
	}

	if _, err := w.Write(header[:headerLen]); err != nil {
		return fmt.Errorf("ws: write frame header: %w", err)
	}
	if len(payload) > 0 {
		if _, err := w.Write(payload); err != nil {
			return fmt.Errorf("ws: write frame payload: %w", err)
		}
	}
	return w.Flush()
}

// wsWriteServerCloseFrame sends an unmasked RFC 6455 close frame (opcode 0x8).
func wsWriteServerCloseFrame(w *bufio.Writer) error {
	header := []byte{0x88, 0x00}
	if _, err := w.Write(header); err != nil {
		return err
	}
	return w.Flush()
}

// WSConnHandler defines the callback function invoked when a WebSocket connection is established.
type WSConnHandler func(conn *WSConn)

// WSServerOption allows custom configuration when creating a WSServer.
type WSServerOption func(*WSServer)

// WithWSAuthToken configures a required Bearer token for WebSocket upgrades.
func WithWSAuthToken(token string) WSServerOption {
	return func(s *WSServer) {
		s.authToken = token
	}
}

// WSServer is an HTTP handler that terminates incoming WebSocket connections,
// enforcing per-IP token-bucket rate limiting and concurrent connection caps.
type WSServer struct {
	limiter   *WSRateLimiter
	handler   WSConnHandler
	authToken string
	mu        sync.RWMutex
	conns     map[*WSConn]struct{}
}

// NewWSServer creates a new WSServer with the given rate limiter and connection handler.
func NewWSServer(limiter *WSRateLimiter, handler WSConnHandler, opts ...WSServerOption) *WSServer {
	if limiter == nil {
		limiter = NewWSRateLimiter(DefaultWSRateLimiterConfig())
	}

	srv := &WSServer{
		limiter: limiter,
		handler: handler,
		conns:   make(map[*WSConn]struct{}),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(srv)
		}
	}

	return srv
}

// Limiter returns the underlying WSRateLimiter.
func (s *WSServer) Limiter() *WSRateLimiter {
	return s.limiter
}

// ActiveConnections returns the number of active WebSocket connections currently connected to this server.
func (s *WSServer) ActiveConnections() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.conns)
}

// Close closes all active WebSocket connections and cleans up the limiter.
func (s *WSServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for conn := range s.conns {
		_ = conn.Close()
		delete(s.conns, conn)
	}

	s.limiter.Close()
	return nil
}

// isWebSocketUpgrade checks if the request represents a WebSocket upgrade handshake.
func isWebSocketUpgrade(r *http.Request) bool {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}
	connHdr := strings.ToLower(r.Header.Get("Connection"))
	return strings.Contains(connHdr, "upgrade")
}

// ServeHTTP handles incoming HTTP requests, upgrading valid WebSocket requests while
// enforcing per-IP rate limiting and concurrent connection limits.
func (s *WSServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tracer := telemetry.GetTracer()
	_, span := tracer.Start(r.Context(), "rpc_ws_upgrade")
	defer span.End()

	// 1. Verify WebSocket upgrade headers
	if !isWebSocketUpgrade(r) {
		http.Error(w, "websocket: not a websocket upgrade", http.StatusBadRequest)
		return
	}

	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		http.Error(w, "websocket: missing Sec-WebSocket-Key", http.StatusBadRequest)
		return
	}

	// 2. Authentication check if configured
	if s.authToken != "" {
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token != s.authToken {
			// Also check query parameter as fallback for browser clients
			if r.URL.Query().Get("token") != s.authToken {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
	}

	// 3. Extract client IP and apply rate limiting
	clientIP := ExtractIP(r)
	span.SetAttributes(attribute.String("client.ip", clientIP))

	release, err := s.limiter.Acquire(clientIP)
	if err != nil {
		span.RecordError(err)
		w.Header().Set("Retry-After", "1")
		w.Header().Set("Sec-WebSocket-Version", "13")
		http.Error(w, "rate limit exceeded: too many concurrent WebSocket connections", http.StatusTooManyRequests)
		if logger.Logger != nil {
			logger.Logger.Warn("WebSocket connection rejected by rate limiter",
				"ip", clientIP, "error", err)
		}
		return
	}

	// 4. Perform WebSocket upgrade via HTTP hijacking
	hj, ok := w.(http.Hijacker)
	if !ok {
		release()
		http.Error(w, "websocket: hijacking not supported", http.StatusInternalServerError)
		return
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		release()
		if logger.Logger != nil {
			logger.Logger.Error("WebSocket hijack failed", "error", err)
		}
		return
	}

	accept := wsAcceptKey(key)

	// Send RFC 6455 101 Switching Protocols response
	resp := fmt.Sprintf(
		"HTTP/1.1 101 Switching Protocols\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Accept: %s\r\n"+
			"\r\n",
		accept,
	)

	if _, err := io.WriteString(bufrw, resp); err != nil {
		release()
		_ = conn.Close()
		return
	}
	if err := bufrw.Flush(); err != nil {
		release()
		_ = conn.Close()
		return
	}

	wsConn := &WSConn{
		Conn:      conn,
		Reader:    bufrw.Reader,
		Writer:    bufrw.Writer,
		ClientIP:  clientIP,
		releaseFn: release,
	}

	s.mu.Lock()
	s.conns[wsConn] = struct{}{}
	s.mu.Unlock()

	if s.handler != nil {
		go func() {
			defer wsConn.Close()
			defer func() {
				s.mu.Lock()
				delete(s.conns, wsConn)
				s.mu.Unlock()
			}()
			s.handler(wsConn)
		}()
	}
}

// ---------------------------------------------------------------------------
// Rate Limiting HTTP Middleware
// ---------------------------------------------------------------------------

// RateLimitMiddleware creates an HTTP middleware that rate-limits incoming WebSocket
// connections per client IP using the provided WSRateLimiter.
//
// For WebSocket requests, it checks the rate limit and concurrent connection count.
// If the limit is exceeded, HTTP 429 Too Many Requests is returned immediately.
func RateLimitMiddleware(limiter *WSRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isWebSocketUpgrade(r) {
				clientIP := ExtractIP(r)
				release, err := limiter.Acquire(clientIP)
				if err != nil {
					w.Header().Set("Retry-After", "1")
					http.Error(w, "rate limit exceeded: too many concurrent WebSocket connections", http.StatusTooManyRequests)
					if logger.Logger != nil {
						logger.Logger.Warn("WebSocket upgrade blocked by rate limit middleware",
							"ip", clientIP, "error", err)
					}
					return
				}

				trackingWriter := &wsTrackingResponseWriter{
					ResponseWriter: w,
					releaseFn:      release,
				}
				defer trackingWriter.cleanup()

				next.ServeHTTP(trackingWriter, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// wsTrackingResponseWriter wraps http.ResponseWriter to track connection hijacking
// and ensure the rate limiter concurrency slot is released when the connection closes.
type wsTrackingResponseWriter struct {
	http.ResponseWriter
	releaseFn func()
	once      sync.Once
	hijacked  bool
}

func (tw *wsTrackingResponseWriter) cleanup() {
	tw.once.Do(func() {
		if !tw.hijacked && tw.releaseFn != nil {
			tw.releaseFn()
		}
	})
}

func (tw *wsTrackingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := tw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("websocket: hijacking not supported")
	}
	tw.hijacked = true

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		if tw.releaseFn != nil {
			tw.releaseFn()
		}
		return nil, nil, err
	}

	trackedConn := &wsTrackedNetConn{
		Conn:      conn,
		releaseFn: tw.releaseFn,
	}
	return trackedConn, bufrw, nil
}

// wsTrackedNetConn wraps net.Conn to execute releaseFn when the connection is closed.
type wsTrackedNetConn struct {
	net.Conn
	releaseFn func()
	once      sync.Once
}

func (c *wsTrackedNetConn) Close() error {
	c.once.Do(func() {
		if c.releaseFn != nil {
			c.releaseFn()
		}
	})
	return c.Conn.Close()
}
