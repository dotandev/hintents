// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	interrors "github.com/dotandev/hintents/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// TokenBucket Tests
// ---------------------------------------------------------------------------

func TestTokenBucket_InitialCapacity(t *testing.T) {
	tb := NewTokenBucket(5.0, 1.0)
	assert.Equal(t, 5.0, tb.Capacity())
	assert.Equal(t, 1.0, tb.RefillRate())
	assert.InDelta(t, 5.0, tb.Tokens(), 0.01)
}

func TestTokenBucket_AllowAndExhaustion(t *testing.T) {
	tb := NewTokenBucket(3.0, 0.0) // No refill

	assert.True(t, tb.Allow(), "1st token should succeed")
	assert.True(t, tb.Allow(), "2nd token should succeed")
	assert.True(t, tb.Allow(), "3rd token should succeed")
	assert.False(t, tb.Allow(), "4th token should fail (exhausted)")
	assert.InDelta(t, 0.0, tb.Tokens(), 0.01)
}

func TestTokenBucket_RefillOverTime(t *testing.T) {
	tb := NewTokenBucket(2.0, 10.0) // 10 tokens per second

	assert.True(t, tb.Allow())
	assert.True(t, tb.Allow())
	assert.False(t, tb.Allow(), "bucket should be empty immediately after 2 takes")

	// Sleep 150ms -> should refill at least 1 token (10 * 0.15 = 1.5 tokens)
	time.Sleep(150 * time.Millisecond)

	assert.True(t, tb.Allow(), "token should be available after refill sleep")
}

func TestTokenBucket_CapacityCap(t *testing.T) {
	tb := NewTokenBucket(2.0, 100.0)

	// Wait 50ms (would generate 5 tokens without cap)
	time.Sleep(50 * time.Millisecond)

	// Should not exceed capacity of 2.0
	assert.InDelta(t, 2.0, tb.Tokens(), 0.05)
}

func TestTokenBucket_AllowN(t *testing.T) {
	tb := NewTokenBucket(5.0, 0.0)

	assert.True(t, tb.AllowN(3.0))
	assert.InDelta(t, 2.0, tb.Tokens(), 0.01)

	assert.False(t, tb.AllowN(3.0), "not enough tokens for 3.0")
	assert.True(t, tb.AllowN(2.0), "exactly 2.0 left")
	assert.False(t, tb.AllowN(1.0))
}

func TestTokenBucket_Reset(t *testing.T) {
	tb := NewTokenBucket(4.0, 0.0)
	assert.True(t, tb.AllowN(4.0))
	assert.False(t, tb.Allow())

	tb.Reset()
	assert.InDelta(t, 4.0, tb.Tokens(), 0.01)
	assert.True(t, tb.Allow())
}

func TestTokenBucket_ConcurrentAccess(t *testing.T) {
	tb := NewTokenBucket(100.0, 0.0)

	var wg sync.WaitGroup
	successCount := int64(0)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				if tb.Allow() {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int64(100), successCount, "should allow exactly 100 tokens across goroutines")
}

// ---------------------------------------------------------------------------
// WSRateLimiter Tests
// ---------------------------------------------------------------------------

func TestWSRateLimiter_ConcurrentConnectionsLimit(t *testing.T) {
	limiter := NewWSRateLimiter(WSRateLimiterConfig{
		MaxConcurrent: 3,
		Capacity:      10.0,
		RefillRate:    10.0,
	})
	defer limiter.Close()

	ip := "192.168.1.100"

	// Acquire 3 concurrent connections
	r1, err1 := limiter.Acquire(ip)
	require.NoError(t, err1)
	require.NotNil(t, r1)

	r2, err2 := limiter.Acquire(ip)
	require.NoError(t, err2)
	require.NotNil(t, r2)

	r3, err3 := limiter.Acquire(ip)
	require.NoError(t, err3)
	require.NotNil(t, r3)

	assert.Equal(t, 3, limiter.ActiveConnections(ip))

	// 4th connection should be rejected due to MaxConcurrent
	r4, err4 := limiter.Acquire(ip)
	assert.Error(t, err4)
	assert.Nil(t, r4)
	assert.True(t, errors.Is(err4, ErrMaxConcurrentConnectionsExceeded))
	assert.True(t, errors.Is(err4, interrors.ErrRateLimitExceeded))
	assert.True(t, IsRateLimitError(err4))

	// Release one connection
	r1()
	assert.Equal(t, 2, limiter.ActiveConnections(ip))

	// Now another acquire should succeed
	r5, err5 := limiter.Acquire(ip)
	require.NoError(t, err5)
	require.NotNil(t, r5)
	assert.Equal(t, 3, limiter.ActiveConnections(ip))

	// Clean up
	r2()
	r3()
	r5()
	assert.Equal(t, 0, limiter.ActiveConnections(ip))
}

func TestWSRateLimiter_TokenBucketExhaustion(t *testing.T) {
	limiter := NewWSRateLimiter(WSRateLimiterConfig{
		MaxConcurrent: 10,
		Capacity:      2.0,
		RefillRate:    0.0, // No refill
	})
	defer limiter.Close()

	ip := "10.0.0.5"

	// Acquire and immediately release 2 connections (active connections = 0, but tokens used)
	r1, err1 := limiter.Acquire(ip)
	require.NoError(t, err1)
	r1()

	r2, err2 := limiter.Acquire(ip)
	require.NoError(t, err2)
	r2()

	// 3rd attempt should fail because token bucket is exhausted
	r3, err3 := limiter.Acquire(ip)
	assert.Error(t, err3)
	assert.Nil(t, r3)
	assert.True(t, errors.Is(err3, ErrRateLimitExceeded))
	assert.True(t, IsRateLimitError(err3))
}

func TestWSRateLimiter_PerIPIsolation(t *testing.T) {
	limiter := NewWSRateLimiter(WSRateLimiterConfig{
		MaxConcurrent: 2,
		Capacity:      2.0,
		RefillRate:    0.0,
	})
	defer limiter.Close()

	ipA := "192.168.1.1"
	ipB := "192.168.1.2"

	// Exhaust IP A
	rA1, errA1 := limiter.Acquire(ipA)
	require.NoError(t, errA1)
	defer rA1()

	rA2, errA2 := limiter.Acquire(ipA)
	require.NoError(t, errA2)
	defer rA2()

	_, errA3 := limiter.Acquire(ipA)
	assert.Error(t, errA3, "IP A should be blocked")

	// IP B should be unaffected
	rB1, errB1 := limiter.Acquire(ipB)
	require.NoError(t, errB1, "IP B should still be allowed")
	defer rB1()

	assert.Equal(t, 2, limiter.ActiveConnections(ipA))
	assert.Equal(t, 1, limiter.ActiveConnections(ipB))
	assert.Equal(t, 3, limiter.TotalActiveConnections())
	assert.Equal(t, 2, limiter.TotalTrackedIPs())
}

func TestWSRateLimiter_IdempotentRelease(t *testing.T) {
	limiter := NewWSRateLimiter(DefaultWSRateLimiterConfig())
	defer limiter.Close()

	ip := "172.16.0.1"

	release, err := limiter.Acquire(ip)
	require.NoError(t, err)
	assert.Equal(t, 1, limiter.ActiveConnections(ip))

	// Calling release multiple times should only decrement once
	release()
	release()
	release()
	assert.Equal(t, 0, limiter.ActiveConnections(ip))
}

func TestWSRateLimiter_TryAcquire(t *testing.T) {
	limiter := NewWSRateLimiter(WSRateLimiterConfig{
		MaxConcurrent: 1,
		Capacity:      1.0,
		RefillRate:    0.0,
	})
	defer limiter.Close()

	ip := "1.1.1.1"

	rel, ok := limiter.TryAcquire(ip)
	assert.True(t, ok)
	require.NotNil(t, rel)

	_, ok2 := limiter.TryAcquire(ip)
	assert.False(t, ok2, "second acquire should fail")

	rel()
	// Still fails because token bucket is exhausted
	_, ok3 := limiter.TryAcquire(ip)
	assert.False(t, ok3)
}

func TestWSRateLimiter_Allow(t *testing.T) {
	limiter := NewWSRateLimiter(WSRateLimiterConfig{
		MaxConcurrent: 1,
		Capacity:      1.0,
		RefillRate:    0.0,
	})
	defer limiter.Close()

	ip := "8.8.8.8"

	assert.True(t, limiter.Allow(ip))

	rel, err := limiter.Acquire(ip)
	require.NoError(t, err)
	defer rel()

	// Now under limit, Allow should be false
	assert.False(t, limiter.Allow(ip))
}

func TestWSRateLimiter_Reset(t *testing.T) {
	limiter := NewWSRateLimiter(DefaultWSRateLimiterConfig())
	defer limiter.Close()

	_, _ = limiter.Acquire("1.1.1.1")
	_, _ = limiter.Acquire("2.2.2.2")
	assert.Equal(t, 2, limiter.TotalTrackedIPs())

	limiter.Reset()
	assert.Equal(t, 0, limiter.TotalTrackedIPs())
	assert.Equal(t, 0, limiter.TotalActiveConnections())
}

func TestWSRateLimiter_CleanupStaleEntries(t *testing.T) {
	limiter := NewWSRateLimiter(WSRateLimiterConfig{
		MaxConcurrent:   5,
		Capacity:        5.0,
		RefillRate:      1.0,
		CleanupInterval: 10 * time.Millisecond,
		EntryTTL:        20 * time.Millisecond,
	})
	defer limiter.Close()

	ipIdle := "192.168.1.50"
	ipActive := "192.168.1.51"

	// ipIdle acquires and releases immediately (0 active connections)
	relIdle, err := limiter.Acquire(ipIdle)
	require.NoError(t, err)
	relIdle()

	// ipActive acquires and keeps open (1 active connection)
	relActive, err := limiter.Acquire(ipActive)
	require.NoError(t, err)
	defer relActive()

	assert.Equal(t, 2, limiter.TotalTrackedIPs())

	// Sleep longer than TTL + CleanupInterval
	time.Sleep(60 * time.Millisecond)

	limiter.mu.RLock()
	tracked := len(limiter.entries)
	_, idleExists := limiter.entries[ipIdle]
	_, activeExists := limiter.entries[ipActive]
	limiter.mu.RUnlock()

	assert.False(t, idleExists, "idle entry should have been evicted")
	assert.True(t, activeExists, "active entry must NOT be evicted")
	assert.Equal(t, 1, tracked)
}

// ---------------------------------------------------------------------------
// IP Extraction and Normalization Tests
// ---------------------------------------------------------------------------

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expected   string
	}{
		{
			name:       "RemoteAddr IPv4 with port",
			remoteAddr: "192.168.1.5:43210",
			expected:   "192.168.1.5",
		},
		{
			name:       "RemoteAddr IPv6 with port",
			remoteAddr: "[2001:db8::1]:8080",
			expected:   "2001:db8::1",
		},
		{
			name:       "X-Forwarded-For single IP",
			remoteAddr: "127.0.0.1:1234",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.195"},
			expected:   "203.0.113.195",
		},
		{
			name:       "X-Forwarded-For multiple IPs (select leftmost)",
			remoteAddr: "127.0.0.1:1234",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.195, 70.41.3.18, 150.172.238.178"},
			expected:   "203.0.113.195",
		},
		{
			name:       "X-Real-IP takes precedence over RemoteAddr",
			remoteAddr: "127.0.0.1:1234",
			headers:    map[string]string{"X-Real-IP": "198.51.100.42"},
			expected:   "198.51.100.42",
		},
		{
			name:       "X-Forwarded-For takes precedence over X-Real-IP",
			remoteAddr: "127.0.0.1:1234",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.5",
				"X-Real-IP":       "198.51.100.42",
			},
			expected: "203.0.113.5",
		},
		{
			name:       "Empty RemoteAddr",
			remoteAddr: "",
			expected:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/ws", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			assert.Equal(t, tt.expected, ExtractIP(req))
			assert.Equal(t, tt.expected, GetClientIP(req))
		})
	}
}

func TestNormalizeIP(t *testing.T) {
	assert.Equal(t, "127.0.0.1", NormalizeIP("127.0.0.1:8080"))
	assert.Equal(t, "127.0.0.1", NormalizeIP("  127.0.0.1  "))
	assert.Equal(t, "::1", NormalizeIP("[::1]:9000"))
	assert.Equal(t, "::1", NormalizeIP("::1"))
	assert.Equal(t, "unknown", NormalizeIP(""))
	assert.Equal(t, "unknown", NormalizeIP("   "))
}

// ---------------------------------------------------------------------------
// WSServer & Handshake Tests
// ---------------------------------------------------------------------------

func TestWSServer_NonWebSocketUpgrade(t *testing.T) {
	srv := NewWSServer(nil, nil)
	defer func() { _ = srv.Close() }()

	req := httptest.NewRequest("GET", "/ws", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "not a websocket upgrade")
}

func TestWSServer_MissingSecWebSocketKey(t *testing.T) {
	srv := NewWSServer(nil, nil)
	defer func() { _ = srv.Close() }()

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing Sec-WebSocket-Key")
}

func TestWSServer_AuthTokenValidation(t *testing.T) {
	srv := NewWSServer(nil, nil, WithWSAuthToken("secret-token-123"))
	defer func() { _ = srv.Close() }()

	// 1. Missing token
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// 2. Invalid token
	req = httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec = httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// 3. Valid token in query param
	req = httptest.NewRequest("GET", "/ws?token=secret-token-123", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rec = httptest.NewRecorder()

	// Response recorder doesn't support Hijacker so it reaches the hijack check
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "hijacking not supported")
}

func TestWSServer_RateLimitRejection_Returns429(t *testing.T) {
	limiter := NewWSRateLimiter(WSRateLimiterConfig{
		MaxConcurrent: 1,
		Capacity:      1.0,
		RefillRate:    0.0,
	})
	defer limiter.Close()

	srv := NewWSServer(limiter, nil)
	defer func() { _ = srv.Close() }()

	clientIP := "192.168.1.200"

	// Exhaust the single connection allowed
	rel, err := limiter.Acquire(clientIP)
	require.NoError(t, err)
	defer rel()

	// Incoming request from same IP should get HTTP 429
	req := httptest.NewRequest("GET", "/ws", nil)
	req.RemoteAddr = clientIP + ":54321"
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "1", rec.Header().Get("Retry-After"))
	assert.Contains(t, rec.Body.String(), "rate limit exceeded")
}

// ---------------------------------------------------------------------------
// Mock Hijackable ResponseWriter for in-memory integration testing
// ---------------------------------------------------------------------------

type mockHijackableWriter struct {
	*httptest.ResponseRecorder
	clientConn net.Conn
	serverConn net.Conn
	hijacked   bool
}

func newMockHijackableWriter() *mockHijackableWriter {
	c1, c2 := net.Pipe()
	return &mockHijackableWriter{
		ResponseRecorder: httptest.NewRecorder(),
		clientConn:       c1,
		serverConn:       c2,
	}
}

func (m *mockHijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if m.hijacked {
		return nil, nil, errors.New("already hijacked")
	}
	m.hijacked = true
	brw := bufio.NewReadWriter(bufio.NewReader(m.serverConn), bufio.NewWriter(m.serverConn))
	return m.serverConn, brw, nil
}

func (m *mockHijackableWriter) Close() {
	_ = m.clientConn.Close()
	_ = m.serverConn.Close()
}

func TestWSServer_SuccessfulUpgradeAndMessageExchange(t *testing.T) {
	limiter := NewWSRateLimiter(WSRateLimiterConfig{
		MaxConcurrent: 2,
		Capacity:      5.0,
		RefillRate:    5.0,
	})
	defer limiter.Close()

	msgReceived := make(chan string, 1)

	handler := func(conn *WSConn) {
		msg, err := conn.ReadMessage()
		if err == nil {
			msgReceived <- string(msg)
			_ = conn.WriteMessage([]byte("echo: " + string(msg)))
		}
		for {
			if _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}

	srv := NewWSServer(limiter, handler)
	defer func() { _ = srv.Close() }()

	rec := newMockHijackableWriter()
	defer rec.Close()

	req := httptest.NewRequest("GET", "/ws", nil)
	req.RemoteAddr = "10.10.10.10:5555"
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	key := wsGenKey()
	req.Header.Set("Sec-WebSocket-Key", key)

	// Serve in background as ServeHTTP sets up the connection
	go srv.ServeHTTP(rec, req)

	// Client reads handshake response from clientConn
	br := bufio.NewReader(rec.clientConn)
	statusLine, err := br.ReadString('\n')
	require.NoError(t, err)
	assert.Contains(t, statusLine, "101 Switching Protocols")

	// Read handshake headers until blank line
	gotAccept := ""
	for {
		line, err := br.ReadString('\n')
		require.NoError(t, err)
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "sec-websocket-accept:") {
			gotAccept = strings.TrimSpace(line[len("sec-websocket-accept:"):])
		}
	}
	assert.Equal(t, wsAcceptKey(key), gotAccept)

	// Verify limiter tracks active connection
	assert.Eventually(t, func() bool {
		return limiter.ActiveConnections("10.10.10.10") == 1 && srv.ActiveConnections() == 1
	}, 1*time.Second, 10*time.Millisecond)

	// Client sends a masked WebSocket frame to server
	err = wsWriteFrame(rec.clientConn, []byte("hello server"))
	require.NoError(t, err)

	// Verify server received message
	select {
	case msg := <-msgReceived:
		assert.Equal(t, "hello server", msg)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message handler")
	}

	// Client reads server unmasked reply
	reply, err := wsReadFrame(br)
	require.NoError(t, err)
	assert.Equal(t, "echo: hello server", string(reply))

	// Close client conn
	_ = rec.clientConn.Close()

	// Wait briefly for server connection cleanup
	assert.Eventually(t, func() bool {
		return limiter.ActiveConnections("10.10.10.10") == 0
	}, 1*time.Second, 20*time.Millisecond, "active connections should return to 0 after close")
}

func TestWSServer_ConnectionExhaustionAttackBlocked(t *testing.T) {
	// Setup limiter with max 2 concurrent connections per IP
	limiter := NewWSRateLimiter(WSRateLimiterConfig{
		MaxConcurrent: 2,
		Capacity:      10.0,
		RefillRate:    10.0,
	})
	defer limiter.Close()

	srv := NewWSServer(limiter, func(conn *WSConn) {
		// Keep connection open until closed by client
		_, _ = conn.ReadMessage()
	})
	defer func() { _ = srv.Close() }()

	attackerIP := "203.0.113.99"

	// Establish connection 1
	rec1 := newMockHijackableWriter()
	defer rec1.Close()
	req1 := httptest.NewRequest("GET", "/ws", nil)
	req1.RemoteAddr = attackerIP + ":1001"
	req1.Header.Set("Upgrade", "websocket")
	req1.Header.Set("Connection", "Upgrade")
	req1.Header.Set("Sec-WebSocket-Key", wsGenKey())
	go srv.ServeHTTP(rec1, req1)

	br1 := bufio.NewReader(rec1.clientConn)
	status1, err := br1.ReadString('\n')
	require.NoError(t, err)
	assert.Contains(t, status1, "101")

	// Establish connection 2
	rec2 := newMockHijackableWriter()
	defer rec2.Close()
	req2 := httptest.NewRequest("GET", "/ws", nil)
	req2.RemoteAddr = attackerIP + ":1002"
	req2.Header.Set("Upgrade", "websocket")
	req2.Header.Set("Connection", "Upgrade")
	req2.Header.Set("Sec-WebSocket-Key", wsGenKey())
	go srv.ServeHTTP(rec2, req2)

	br2 := bufio.NewReader(rec2.clientConn)
	status2, err := br2.ReadString('\n')
	require.NoError(t, err)
	assert.Contains(t, status2, "101")

	assert.Equal(t, 2, limiter.ActiveConnections(attackerIP))

	// Connection 3 from same IP MUST be rejected with HTTP 429
	rec3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("GET", "/ws", nil)
	req3.RemoteAddr = attackerIP + ":1003"
	req3.Header.Set("Upgrade", "websocket")
	req3.Header.Set("Connection", "Upgrade")
	req3.Header.Set("Sec-WebSocket-Key", wsGenKey())

	srv.ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusTooManyRequests, rec3.Code)
	assert.Contains(t, rec3.Body.String(), "rate limit exceeded")

	// But a DIFFERENT IP address can still connect!
	victimIP := "198.51.100.1"
	recLegit := newMockHijackableWriter()
	defer recLegit.Close()
	reqLegit := httptest.NewRequest("GET", "/ws", nil)
	reqLegit.RemoteAddr = victimIP + ":5000"
	reqLegit.Header.Set("Upgrade", "websocket")
	reqLegit.Header.Set("Connection", "Upgrade")
	reqLegit.Header.Set("Sec-WebSocket-Key", wsGenKey())
	go srv.ServeHTTP(recLegit, reqLegit)

	brLegit := bufio.NewReader(recLegit.clientConn)
	statusLegit, err := brLegit.ReadString('\n')
	require.NoError(t, err)
	assert.Contains(t, statusLegit, "101")
	assert.Equal(t, 1, limiter.ActiveConnections(victimIP))
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := NewWSRateLimiter(WSRateLimiterConfig{
		MaxConcurrent: 1,
		Capacity:      1.0,
		RefillRate:    0.0,
	})
	defer limiter.Close()

	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mw := RateLimitMiddleware(limiter)(nextHandler)

	// 1. Regular non-WebSocket request passes through
	req1 := httptest.NewRequest("GET", "/api", nil)
	rec1 := httptest.NewRecorder()
	mw.ServeHTTP(rec1, req1)
	assert.True(t, nextHandlerCalled)
	assert.Equal(t, http.StatusOK, rec1.Code)

	// 2. WebSocket request succeeds once
	nextHandlerCalled = false
	req2 := httptest.NewRequest("GET", "/ws", nil)
	req2.RemoteAddr = "1.2.3.4:1234"
	req2.Header.Set("Upgrade", "websocket")
	req2.Header.Set("Connection", "Upgrade")
	rec2 := httptest.NewRecorder()
	mw.ServeHTTP(rec2, req2)
	assert.True(t, nextHandlerCalled)
	assert.Equal(t, http.StatusOK, rec2.Code)

	// 3. Second WebSocket request from same IP is blocked (token bucket exhausted)
	nextHandlerCalled = false
	req3 := httptest.NewRequest("GET", "/ws", nil)
	req3.RemoteAddr = "1.2.3.4:5678"
	req3.Header.Set("Upgrade", "websocket")
	req3.Header.Set("Connection", "Upgrade")
	rec3 := httptest.NewRecorder()
	mw.ServeHTTP(rec3, req3)
	assert.False(t, nextHandlerCalled, "handler must not be called when rate limited")
	assert.Equal(t, http.StatusTooManyRequests, rec3.Code)
}
