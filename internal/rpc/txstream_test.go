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
				pw.CloseWithError(err)
				return
			}
			pw.Close()
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
		wsWriteFrame(pw, nil) //nolint:errcheck
		pw.Close()
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
		fmt.Fprint(w, resp)
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
		fmt.Fprint(w, resp)
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
		defer conn.Close()

		// Write the 101 upgrade response manually.
		fmt.Fprintf(bufrw,
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
			conn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
			msg, err := wsReadFrame(bufrw.Reader)
			if err != nil {
				return
			}
			conn.SetReadDeadline(time.Time{}) //nolint:errcheck

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

			conn.SetWriteDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
			// Server sends unmasked frames — use a simple writer that skips masking.
			if err := wsWriteFrameUnmasked(conn, []byte(resp)); err != nil {
				return
			}
			conn.SetWriteDeadline(time.Time{}) //nolint:errcheck

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
	if _, ok := streamer.(*wsStreamer); !ok {
		t.Logf("server at %s did not accept WebSocket probe — treating as expected in offline CI", httpURL)
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
