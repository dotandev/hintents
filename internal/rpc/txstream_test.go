// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Unit tests — TxStatus
// ---------------------------------------------------------------------------

func TestTxStatus_IsFinal(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{TxStatusSuccess, true},
		{TxStatusFailed, true},
		{TxStatusPending, false},
		{TxStatusNotFound, false},
		{"", false},
	}
	for _, c := range cases {
		got := (TxStatus{Status: c.status}).IsFinal()
		if got != c.want {
			t.Errorf("TxStatus{%q}.IsFinal() = %v, want %v", c.status, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Unit tests — wsURLFor
// ---------------------------------------------------------------------------

func TestWsURLFor(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://soroban-testnet.stellar.org", "wss://soroban-testnet.stellar.org"},
		{"http://localhost:8000", "ws://localhost:8000"},
		{"", ""},
		{"ftp://example.com", ""},
	}
	for _, c := range cases {
		got := wsURLFor(c.in)
		if got != c.want {
			t.Errorf("wsURLFor(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Unit tests — wsAcceptKey (RFC 6455 §4.2.2)
// ---------------------------------------------------------------------------

func TestWsAcceptKey_KnownVector(t *testing.T) {
	// RFC 6455 §4.2.2 example: client key "dGhlIHNhbXBsZSBub25jZQ=="
	// expected accept: "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	const clientKey = "dGhlIHNhbXBsZSBub25jZQ=="
	const want = "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	got := wsAcceptKey(clientKey)
	if got != want {
		t.Errorf("wsAcceptKey(%q) = %q, want %q", clientKey, got, want)
	}
}

// ---------------------------------------------------------------------------
// Unit tests — parseWSURL
// ---------------------------------------------------------------------------

func TestParseWSURL(t *testing.T) {
	cases := []struct {
		in         string
		wantScheme string
		wantHost   string
		wantPort   string
		wantPath   string
		wantErr    bool
	}{
		{
			in:         "wss://example.com/rpc",
			wantScheme: "wss",
			wantHost:   "example.com",
			wantPort:   "443",
			wantPath:   "/rpc",
		},
		{
			in:         "ws://localhost:8000",
			wantScheme: "ws",
			wantHost:   "localhost",
			wantPort:   "8000",
			wantPath:   "/",
		},
		{
			in:         "wss://node.example.com:4321/v1/rpc",
			wantScheme: "wss",
			wantHost:   "node.example.com",
			wantPort:   "4321",
			wantPath:   "/v1/rpc",
		},
		{
			in:      "https://bad.scheme",
			wantErr: true,
		},
	}

	for _, c := range cases {
		scheme, host, port, path, err := parseWSURL(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseWSURL(%q): expected error, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseWSURL(%q): unexpected error: %v", c.in, err)
			continue
		}
		if scheme != c.wantScheme || host != c.wantHost || port != c.wantPort || path != c.wantPath {
			t.Errorf("parseWSURL(%q) = (%q,%q,%q,%q), want (%q,%q,%q,%q)",
				c.in, scheme, host, port, path,
				c.wantScheme, c.wantHost, c.wantPort, c.wantPath)
		}
	}
}

// ---------------------------------------------------------------------------
// Unit tests — WebSocket frame encoding / decoding
// ---------------------------------------------------------------------------

func TestWsFrameRoundTrip(t *testing.T) {
	payloads := [][]byte{
		[]byte(`{"jsonrpc":"2.0"}`),
		[]byte(strings.Repeat("a", 200)),   // > 125 bytes triggers 2-byte length
		[]byte(strings.Repeat("b", 70000)), // > 65535 bytes triggers 8-byte length
	}

	for _, want := range payloads {
		// Write a masked client frame into a pipe.
		pr, pw := io.Pipe()
		go func() {
			if err := wsWriteFrame(pw, want); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			_ = pw.Close()
		}()

		// Unmask and read server-side (server reads client frames, which are masked).
		// For this test we re-use wsReadFrame which handles masked frames too.
		br := bufio.NewReader(pr)
		got, err := wsReadFrame(br)
		if err != nil {
			t.Errorf("wsReadFrame: %v (payload len %d)", err, len(want))
			continue
		}
		if string(got) != string(want) {
			t.Errorf("frame round-trip mismatch: got len %d, want len %d", len(got), len(want))
		}
	}
}

func TestWsWriteFrame_CloseFrame(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		_ = wsWriteFrame(pw, nil) //nolint:errcheck // Test code, pipe write failure is not critical
		_ = pw.Close()
	}()

	br := bufio.NewReader(pr)
	// A nil payload produces a close frame; wsReadFrame should return io.EOF.
	_, err := wsReadFrame(br)
	if err != io.EOF {
		t.Errorf("close frame: expected io.EOF, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Integration test — pollingStreamer with a mock Soroban RPC HTTP server
// ---------------------------------------------------------------------------

// serveGetTransaction handles JSON-RPC getTransaction requests and sequences
// through the provided statuses.
func serveGetTransaction(statuses []string) http.HandlerFunc {
	idx := 0
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "expected POST", http.StatusMethodNotAllowed)
			return
		}

		if idx >= len(statuses) {
			idx = len(statuses) - 1
		}
		status := statuses[idx]
		idx++

		resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{"status":%q,"ledger":100}}`, status)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, resp)
	}
}

func TestPollingStreamer_StreamsFinalStatus(t *testing.T) {
	// Serve: PENDING → PENDING → SUCCESS
	srv := httptest.NewServer(serveGetTransaction([]string{
		TxStatusPending,
		TxStatusPending,
		TxStatusSuccess,
	}))
	defer srv.Close()

	client := &Client{
		SorobanURL: srv.URL,
		httpClient: srv.Client(),
	}

	streamer := &pollingStreamer{client: client}

	// Use a short interval so the test runs quickly.
	origInterval := pollStreamInterval
	pollStreamInterval = 20 * time.Millisecond
	defer func() { pollStreamInterval = origInterval }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := streamer.Stream(ctx, "deadbeef01")
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var statuses []string
	for s := range ch {
		statuses = append(statuses, s.Status)
	}

	if len(statuses) == 0 {
		t.Fatal("received no status updates")
	}
	last := statuses[len(statuses)-1]
	if last != TxStatusSuccess {
		t.Errorf("last status = %q, want %q", last, TxStatusSuccess)
	}
}

func TestPollingStreamer_ContextCancellation(t *testing.T) {
	// Server always returns PENDING — context cancellation should stop streaming.
	srv := httptest.NewServer(serveGetTransaction([]string{
		TxStatusPending, TxStatusPending, TxStatusPending,
		TxStatusPending, TxStatusPending, TxStatusPending,
	}))
	defer srv.Close()

	client := &Client{
		SorobanURL: srv.URL,
		httpClient: srv.Client(),
	}
	streamer := &pollingStreamer{client: client}

	origInterval := pollStreamInterval
	pollStreamInterval = 20 * time.Millisecond
	defer func() { pollStreamInterval = origInterval }()

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := streamer.Stream(ctx, "aabbcc")
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	// Cancel after receiving a few statuses.
	go func() {
		<-ch
		cancel()
	}()

	deadline := time.After(3 * time.Second)
	select {
	case <-deadline:
		t.Error("channel not closed after context cancellation")
	case _, ok := <-ch:
		if ok {
			// drain remaining
			for range ch {
			}
		}
		// channel closed — success
	}
}

func TestPollingStreamer_RPCError_Retries(t *testing.T) {
	// First request returns 500, subsequent requests return SUCCESS.
	callN := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callN++
		if callN == 1 {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{"status":%q,"ledger":101}}`, TxStatusSuccess)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, resp)
	}))
	defer srv.Close()

	client := &Client{
		SorobanURL: srv.URL,
		httpClient: srv.Client(),
	}
	streamer := &pollingStreamer{client: client}

	origInterval := pollStreamInterval
	pollStreamInterval = 20 * time.Millisecond
	defer func() { pollStreamInterval = origInterval }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := streamer.Stream(ctx, "abc123")
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var last TxStatus
	for s := range ch {
		last = s
	}
	if last.Status != TxStatusSuccess {
		t.Errorf("expected final SUCCESS after retry; got %q", last.Status)
	}
}

// ---------------------------------------------------------------------------
// Integration test — WebSocket streamer with a mock WS server
// ---------------------------------------------------------------------------

// newMockWSServer starts a minimal WebSocket server that responds to
// getTransaction requests and sequences through statuses.
func newMockWSServer(t *testing.T, statuses []string) *httptest.Server {
	t.Helper()
	idx := 0

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Perform the WebSocket handshake.
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "not a websocket upgrade", http.StatusBadRequest)
			return
		}

		key := r.Header.Get("Sec-Websocket-Key")
		accept := wsAcceptKey(key)

		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijacking not supported", http.StatusInternalServerError)
			return
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		// Write the 101 upgrade response manually.
		_, _ = fmt.Fprintf(bufrw,
			"HTTP/1.1 101 Switching Protocols\r\n"+
				"Upgrade: websocket\r\n"+
				"Connection: Upgrade\r\n"+
				"Sec-WebSocket-Accept: %s\r\n"+
				"\r\n",
			accept,
		)
		if err := bufrw.Flush(); err != nil {
			return
		}

		// Service JSON-RPC requests until the client closes.
		for {
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck // Mock WebSocket server, deadline set failure is not critical
			msg, err := wsReadFrame(bufrw.Reader)
			if err != nil {
				return
			}
			_ = conn.SetReadDeadline(time.Time{}) //nolint:errcheck // Mock WebSocket server, deadline clear failure is not critical

			var req jsonrpcRequest
			if err := json.Unmarshal(msg, &req); err != nil {
				return
			}

			if idx >= len(statuses) {
				idx = len(statuses) - 1
			}
			status := statuses[idx]
			idx++

			resp := fmt.Sprintf(
				`{"jsonrpc":"2.0","id":%d,"result":{"status":%q,"ledger":200}}`,
				req.ID, status,
			)

			_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck // Mock WebSocket server, deadline set failure is not critical
			// Server sends unmasked frames — use a simple writer that skips masking.
			if err := wsWriteFrameUnmasked(conn, []byte(resp)); err != nil {
				return
			}
			_ = conn.SetWriteDeadline(time.Time{}) //nolint:errcheck // Mock WebSocket server, deadline clear failure is not critical

			if status == TxStatusSuccess || status == TxStatusFailed {
				return
			}
		}
	})

	return httptest.NewServer(handler)
}

// wsWriteFrameUnmasked writes an unmasked text frame (as a server would send).
// This is only used in tests; production client code always sends masked frames.
func wsWriteFrameUnmasked(w io.Writer, payload []byte) error {
	header := make([]byte, 2)
	header[0] = 0x81 // FIN=1, opcode=text
	pLen := len(payload)
	switch {
	case pLen < 126:
		header[1] = byte(pLen)
	case pLen <= 0xFFFF:
		header[1] = 126
		ext := make([]byte, 2)
		ext[0] = byte(pLen >> 8)
		ext[1] = byte(pLen)
		header = append(header, ext...)
	default:
		header[1] = 127
		ext := make([]byte, 8)
		for i := 7; i >= 0; i-- {
			ext[i] = byte(pLen & 0xFF)
			pLen >>= 8
		}
		header = append(header, ext...)
	}
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func TestWsStreamer_StreamsFinalStatus(t *testing.T) {
	srv := newMockWSServer(t, []string{TxStatusPending, TxStatusPending, TxStatusSuccess})
	defer srv.Close()

	wsURL := "ws://" + srv.Listener.Addr().String()
	client := &Client{
		SorobanURL: "http://" + srv.Listener.Addr().String(),
		token:      "",
	}
	streamer := &wsStreamer{client: client, wsURL: wsURL}

	origInterval := wsStreamInterval
	wsStreamInterval = 20 * time.Millisecond
	defer func() { wsStreamInterval = origInterval }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := streamer.Stream(ctx, "cafebabe")
	if err != nil {
		t.Fatalf("wsStreamer.Stream: %v", err)
	}

	var statuses []string
	for s := range ch {
		statuses = append(statuses, s.Status)
	}
	if len(statuses) == 0 {
		t.Fatal("no statuses received")
	}
	if last := statuses[len(statuses)-1]; last != TxStatusSuccess {
		t.Errorf("last status = %q, want SUCCESS", last)
	}
}

// ---------------------------------------------------------------------------
// Integration test — NewTxStreamer factory
// ---------------------------------------------------------------------------

func TestNewTxStreamer_FallsBackToPolling_WhenNoWSServer(t *testing.T) {
	// Point at a plain HTTP server that does not support WebSocket upgrade.
	srv := httptest.NewServer(serveGetTransaction([]string{TxStatusSuccess}))
	defer srv.Close()

	client, err := NewClient(
		WithNetwork(Testnet),
		WithSorobanURL(srv.URL),
		WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	streamer := NewTxStreamer(client)
	if _, ok := streamer.(*wsStreamer); ok {
		t.Error("expected pollingStreamer fallback, got wsStreamer")
	}
	if _, ok := streamer.(*pollingStreamer); !ok {
		t.Error("expected *pollingStreamer, got different type")
	}
}

func TestNewTxStreamer_PrefersWebSocket_WhenSupportedByServer(t *testing.T) {
	srv := newMockWSServer(t, []string{TxStatusSuccess})
	defer srv.Close()

	// Supply the HTTP URL; NewTxStreamer will convert it to ws://.
	httpURL := "http://" + srv.Listener.Addr().String()

	client, err := NewClient(
		WithNetwork(Testnet),
		WithSorobanURL(httpURL),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	streamer := NewTxStreamer(client)
	if _, ok := streamer.(*autoFallbackStreamer); !ok {
		t.Fatalf("expected *autoFallbackStreamer when WebSocket is supported, got %T", streamer)
	}
}

func TestClientWatchTransaction_UsesWebSocketStreaming(t *testing.T) {
	srv := newMockWSServer(t, []string{TxStatusPending, TxStatusSuccess})
	defer srv.Close()

	client, err := NewClient(
		WithNetwork(Testnet),
		WithSorobanURL("http://"+srv.Listener.Addr().String()),
		WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	origInterval := wsStreamInterval
	wsStreamInterval = 20 * time.Millisecond
	defer func() { wsStreamInterval = origInterval }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := client.WatchTransaction(ctx, "feedface")
	if err != nil {
		t.Fatalf("WatchTransaction: %v", err)
	}

	var statuses []string
	for status := range ch {
		statuses = append(statuses, status.Status)
	}

	if len(statuses) < 2 {
		t.Fatalf("expected multiple statuses from WatchTransaction, got %v", statuses)
	}
	if got := statuses[len(statuses)-1]; got != TxStatusSuccess {
		t.Fatalf("final status = %q, want %q", got, TxStatusSuccess)
	}
}

func TestClientWatchTransaction_FallsBackWhenWebSocketStreamDrops(t *testing.T) {
	var mu sync.Mutex
	upgradeCount := 0
	httpStatuses := []string{TxStatusPending, TxStatusSuccess}
	httpIdx := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			mu.Lock()
			upgradeCount++
			currentUpgrade := upgradeCount
			mu.Unlock()

			key := r.Header.Get("Sec-Websocket-Key")
			accept := wsAcceptKey(key)

			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "hijacking not supported", http.StatusInternalServerError)
				return
			}

			conn, bufrw, err := hj.Hijack()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()

			_, _ = fmt.Fprintf(bufrw,
				"HTTP/1.1 101 Switching Protocols\r\n"+
					"Upgrade: websocket\r\n"+
					"Connection: Upgrade\r\n"+
					"Sec-WebSocket-Accept: %s\r\n"+
					"\r\n",
				accept,
			)
			if err := bufrw.Flush(); err != nil {
				return
			}

			// First upgrade is the probe. On the actual stream, return one
			// PENDING update and then drop the connection so polling can resume.
			if currentUpgrade == 1 {
				return
			}

			msg, err := wsReadFrame(bufrw.Reader)
			if err != nil {
				return
			}

			var req jsonrpcRequest
			if err := json.Unmarshal(msg, &req); err != nil {
				return
			}

			resp := fmt.Sprintf(
				`{"jsonrpc":"2.0","id":%d,"result":{"status":%q,"ledger":123}}`,
				req.ID, TxStatusPending,
			)
			if err := wsWriteFrameUnmasked(conn, []byte(resp)); err != nil {
				t.Errorf("wsWriteFrameUnmasked: %v", err)
			}
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "expected POST", http.StatusMethodNotAllowed)
			return
		}

		mu.Lock()
		if httpIdx >= len(httpStatuses) {
			httpIdx = len(httpStatuses) - 1
		}
		status := httpStatuses[httpIdx]
		httpIdx++
		mu.Unlock()

		resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{"status":%q,"ledger":456}}`, status)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, resp)
	}))
	defer srv.Close()

	client, err := NewClient(
		WithNetwork(Testnet),
		WithSorobanURL(srv.URL),
		WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	origWSInterval := wsStreamInterval
	origPollInterval := pollStreamInterval
	wsStreamInterval = 20 * time.Millisecond
	pollStreamInterval = 20 * time.Millisecond
	defer func() {
		wsStreamInterval = origWSInterval
		pollStreamInterval = origPollInterval
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := client.WatchTransaction(ctx, "decafbad")
	if err != nil {
		t.Fatalf("WatchTransaction: %v", err)
	}

	var statuses []string
	for status := range ch {
		statuses = append(statuses, status.Status)
	}

	if len(statuses) < 2 {
		t.Fatalf("expected fallback statuses, got %v", statuses)
	}
	if got := statuses[len(statuses)-1]; got != TxStatusSuccess {
		t.Fatalf("final status = %q, want %q", got, TxStatusSuccess)
	}
	mu.Lock()
	count := upgradeCount
	mu.Unlock()
	if count < 2 {
		t.Fatalf("expected probe and watch WebSocket upgrades, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Probe test
// ---------------------------------------------------------------------------

func TestProbeWebSocket_ReturnsFalseForHTTPOnlyServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not a websocket server", http.StatusOK)
	}))
	defer srv.Close()

	// Strip the http:// prefix if present — Listener.Addr() returns host:port.
	wsURL := "ws://" + strings.TrimPrefix(srv.URL, "http://")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if probeWebSocket(ctx, wsURL, "") {
		t.Error("expected probeWebSocket to return false for non-WebSocket server")
	}
}

func TestProbeWebSocket_ReturnsTrueForWSServer(t *testing.T) {
	srv := newMockWSServer(t, []string{TxStatusSuccess})
	defer srv.Close()

	wsURL := "ws://" + strings.TrimPrefix(srv.URL, "http://")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if !probeWebSocket(ctx, wsURL, "") {
		t.Error("expected probeWebSocket to return true for WebSocket server")
	}
}

// ---------------------------------------------------------------------------
// Dial failure test
// ---------------------------------------------------------------------------

func TestWsDialUpgrade_FailsForUnreachableHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Port 1 is conventionally unused and will refuse immediately on most systems.
	_, err := wsDialUpgrade(ctx, "ws://127.0.0.1:1/rpc", "")
	if err == nil {
		t.Error("expected error dialing unreachable host, got nil")
	}
}

// ---------------------------------------------------------------------------
// wsGenKey uniqueness
// ---------------------------------------------------------------------------

func TestWsGenKey_AreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		k := wsGenKey()
		if seen[k] {
			t.Errorf("duplicate key generated: %q", k)
		}
		seen[k] = true
		if len(k) == 0 {
			t.Error("empty key generated")
		}
	}
}

// Verify that net.Conn is assignable to io.Writer for wsConn.raw usage.
var _ io.Writer = (net.Conn)(nil)

// ---------------------------------------------------------------------------
// Unit tests — decodeTxStatus
// ---------------------------------------------------------------------------

func TestDecodeTxStatus_FullResponse(t *testing.T) {
	resp := &jsonrpcResponse{
		Result: &sorobanGetTxResult{
			Status:           TxStatusSuccess,
			Ledger:           1234,
			EnvelopeXdr:      "ENVXDR==",
			ResultXdr:        "RESXDR==",
			ResultMetaXdr:    "METAXDR==",
			ApplicationOrder: 2,
			LatestLedger:     1240,
			CreatedAt:        1700000000,
		},
	}
	got := decodeTxStatus("abc123", resp)
	if got.Hash != "abc123" {
		t.Errorf("Hash = %q, want %q", got.Hash, "abc123")
	}
	if got.Status != TxStatusSuccess {
		t.Errorf("Status = %q, want SUCCESS", got.Status)
	}
	if got.Ledger != 1234 {
		t.Errorf("Ledger = %d, want 1234", got.Ledger)
	}
	if got.EnvelopeXdr != "ENVXDR==" {
		t.Errorf("EnvelopeXdr = %q, want %q", got.EnvelopeXdr, "ENVXDR==")
	}
	if got.ResultXdr != "RESXDR==" {
		t.Errorf("ResultXdr = %q, want %q", got.ResultXdr, "RESXDR==")
	}
	if got.ResultMetaXdr != "METAXDR==" {
		t.Errorf("ResultMetaXdr = %q, want %q", got.ResultMetaXdr, "METAXDR==")
	}
	if got.ApplicationOrder != 2 {
		t.Errorf("ApplicationOrder = %d, want 2", got.ApplicationOrder)
	}
	if got.LatestLedger != 1240 {
		t.Errorf("LatestLedger = %d, want 1240", got.LatestLedger)
	}
	if got.CreatedAt != 1700000000 {
		t.Errorf("CreatedAt = %d, want 1700000000", got.CreatedAt)
	}
}

func TestDecodeTxStatus_RPCError(t *testing.T) {
	resp := &jsonrpcResponse{
		Error: &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{Code: -32600, Message: "transaction not found"},
	}
	got := decodeTxStatus("deadbeef", resp)
	if got.Status != TxStatusNotFound {
		t.Errorf("Status = %q, want NOT_FOUND", got.Status)
	}
	if got.Error != "transaction not found" {
		t.Errorf("Error = %q, want %q", got.Error, "transaction not found")
	}
}

func TestDecodeTxStatus_NilResult(t *testing.T) {
	resp := &jsonrpcResponse{Result: nil}
	got := decodeTxStatus("xyz", resp)
	if got.Status != TxStatusNotFound {
		t.Errorf("nil result: Status = %q, want NOT_FOUND", got.Status)
	}
}

// ---------------------------------------------------------------------------
// Unit tests — wsReadMessage (fragmented frame reassembly)
// ---------------------------------------------------------------------------

// wsWriteFragmented splits payload into n unmasked frames.
// The first frame uses opcode 0x1 (text) with FIN=0; subsequent frames use
// opcode 0x0 (continuation); the last fragment has FIN=1.
func wsWriteFragmented(w io.Writer, payload []byte, n int) error {
	if n <= 0 {
		n = 1
	}
	chunkSize := (len(payload) + n - 1) / n
	for i := 0; i < n; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(payload) {
			end = len(payload)
		}
		chunk := payload[start:end]

		fin := byte(0x00)
		if i == n-1 {
			fin = 0x80
		}
		var opcode byte
		if i == 0 {
			opcode = 0x01 // text
		} else {
			opcode = 0x00 // continuation
		}
		header := []byte{fin | opcode, byte(len(chunk))}
		if _, err := w.Write(header); err != nil {
			return err
		}
		if _, err := w.Write(chunk); err != nil {
			return err
		}
	}
	return nil
}

func TestWsReadMessage_ReassemblesFragments(t *testing.T) {
	want := []byte(`{"jsonrpc":"2.0","id":1,"result":{"status":"SUCCESS","ledger":42}}`)

	pr, pw := io.Pipe()
	go func() {
		if err := wsWriteFragmented(pw, want, 3); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()

	br := bufio.NewReader(pr)
	got, err := wsReadMessage(br)
	if err != nil {
		t.Fatalf("wsReadMessage: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("reassembled = %q, want %q", got, want)
	}
}

func TestWsReadMessage_SingleFrame(t *testing.T) {
	want := []byte(`{"status":"PENDING"}`)
	pr, pw := io.Pipe()
	go func() {
		_ = wsWriteFrameUnmasked(pw, want) //nolint:errcheck // Test code, pipe write failure is not critical
		_ = pw.Close()
	}()

	br := bufio.NewReader(pr)
	got, err := wsReadMessage(br)
	if err != nil {
		t.Fatalf("wsReadMessage single frame: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Integration tests — XDR field propagation through pollingStreamer
// ---------------------------------------------------------------------------

// serveGetTransactionFull returns a handler that serves a full getTransaction
// response containing all XDR fields on the first SUCCESS response.
func serveGetTransactionFull() http.HandlerFunc {
	called := false
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "expected POST", http.StatusMethodNotAllowed)
			return
		}
		var status string
		if !called {
			called = true
			status = TxStatusPending
			resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{"status":%q,"ledger":0}}`, status)
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, resp)
			return
		}
		resp := `{"jsonrpc":"2.0","id":1,"result":{` +
			`"status":"SUCCESS",` +
			`"ledger":500,` +
			`"envelopeXdr":"ENVXDR==",` +
			`"resultXdr":"RESXDR==",` +
			`"resultMetaXdr":"METAXDR==",` +
			`"applicationOrder":3,` +
			`"latestLedger":510,` +
			`"createdAt":1700000001` +
			`}}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, resp)
	}
}

func TestPollingStreamer_DecodesFullResponse(t *testing.T) {
	srv := httptest.NewServer(serveGetTransactionFull())
	defer srv.Close()

	client := &Client{
		SorobanURL: srv.URL,
		httpClient: srv.Client(),
	}
	streamer := &pollingStreamer{client: client}

	origInterval := pollStreamInterval
	pollStreamInterval = 20 * time.Millisecond
	defer func() { pollStreamInterval = origInterval }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := streamer.Stream(ctx, "fullhash")
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var final TxStatus
	for s := range ch {
		final = s
	}

	if final.Status != TxStatusSuccess {
		t.Errorf("Status = %q, want SUCCESS", final.Status)
	}
	if final.EnvelopeXdr != "ENVXDR==" {
		t.Errorf("EnvelopeXdr = %q, want ENVXDR==", final.EnvelopeXdr)
	}
	if final.ResultXdr != "RESXDR==" {
		t.Errorf("ResultXdr = %q, want RESXDR==", final.ResultXdr)
	}
	if final.ResultMetaXdr != "METAXDR==" {
		t.Errorf("ResultMetaXdr = %q, want METAXDR==", final.ResultMetaXdr)
	}
	if final.ApplicationOrder != 3 {
		t.Errorf("ApplicationOrder = %d, want 3", final.ApplicationOrder)
	}
	if final.LatestLedger != 510 {
		t.Errorf("LatestLedger = %d, want 510", final.LatestLedger)
	}
	if final.CreatedAt != 1700000001 {
		t.Errorf("CreatedAt = %d, want 1700000001", final.CreatedAt)
	}
}

// ---------------------------------------------------------------------------
// Integration test — XDR field propagation through wsStreamer
// ---------------------------------------------------------------------------

// newMockWSServerFull is like newMockWSServer but the SUCCESS response
// includes all XDR fields.
func newMockWSServerFull(t *testing.T) *httptest.Server {
	t.Helper()
	callN := 0

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "not a websocket upgrade", http.StatusBadRequest)
			return
		}

		key := r.Header.Get("Sec-Websocket-Key")
		accept := wsAcceptKey(key)

		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijacking not supported", http.StatusInternalServerError)
			return
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		_, _ = fmt.Fprintf(bufrw,
			"HTTP/1.1 101 Switching Protocols\r\n"+
				"Upgrade: websocket\r\n"+
				"Connection: Upgrade\r\n"+
				"Sec-WebSocket-Accept: %s\r\n"+
				"\r\n",
			accept,
		)
		if err := bufrw.Flush(); err != nil {
			return
		}

		for {
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck // Mock WebSocket server, deadline set failure is not critical
			msg, err := wsReadFrame(bufrw.Reader)
			if err != nil {
				return
			}
			_ = conn.SetReadDeadline(time.Time{}) //nolint:errcheck // Mock WebSocket server, deadline clear failure is not critical

			var req jsonrpcRequest
			if err := json.Unmarshal(msg, &req); err != nil {
				return
			}

			callN++
			var resp string
			if callN < 2 {
				resp = fmt.Sprintf(
					`{"jsonrpc":"2.0","id":%d,"result":{"status":%q,"ledger":0}}`,
					req.ID, TxStatusPending,
				)
			} else {
				resp = fmt.Sprintf(
					`{"jsonrpc":"2.0","id":%d,"result":{`+
						`"status":"SUCCESS",`+
						`"ledger":600,`+
						`"envelopeXdr":"WSENV==",`+
						`"resultXdr":"WSRES==",`+
						`"resultMetaXdr":"WSMETA==",`+
						`"applicationOrder":5,`+
						`"latestLedger":605,`+
						`"createdAt":1700000002`+
						`}}`,
					req.ID,
				)
			}

			_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck // Mock WebSocket server, deadline set failure is not critical
			if err := wsWriteFrameUnmasked(conn, []byte(resp)); err != nil {
				return
			}
			_ = conn.SetWriteDeadline(time.Time{}) //nolint:errcheck // Mock WebSocket server, deadline clear failure is not critical

			if callN >= 2 {
				return
			}
		}
	})

	return httptest.NewServer(handler)
}

func TestWsStreamer_DecodesFullResponse(t *testing.T) {
	srv := newMockWSServerFull(t)
	defer srv.Close()

	wsURL := "ws://" + srv.Listener.Addr().String()
	client := &Client{
		SorobanURL: "http://" + srv.Listener.Addr().String(),
	}
	streamer := &wsStreamer{client: client, wsURL: wsURL}

	origInterval := wsStreamInterval
	wsStreamInterval = 20 * time.Millisecond
	defer func() { wsStreamInterval = origInterval }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := streamer.Stream(ctx, "wshash")
	if err != nil {
		t.Fatalf("wsStreamer.Stream: %v", err)
	}

	var final TxStatus
	for s := range ch {
		final = s
	}

	if final.Status != TxStatusSuccess {
		t.Errorf("Status = %q, want SUCCESS", final.Status)
	}
	if final.EnvelopeXdr != "WSENV==" {
		t.Errorf("EnvelopeXdr = %q, want WSENV==", final.EnvelopeXdr)
	}
	if final.ResultXdr != "WSRES==" {
		t.Errorf("ResultXdr = %q, want WSRES==", final.ResultXdr)
	}
	if final.ResultMetaXdr != "WSMETA==" {
		t.Errorf("ResultMetaXdr = %q, want WSMETA==", final.ResultMetaXdr)
	}
	if final.ApplicationOrder != 5 {
		t.Errorf("ApplicationOrder = %d, want 5", final.ApplicationOrder)
	}
	if final.LatestLedger != 605 {
		t.Errorf("LatestLedger = %d, want 605", final.LatestLedger)
	}
	if final.CreatedAt != 1700000002 {
		t.Errorf("CreatedAt = %d, want 1700000002", final.CreatedAt)
	}
}
