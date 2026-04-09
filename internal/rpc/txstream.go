// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package rpc provides access to Stellar Horizon and Soroban RPC endpoints.
// txstream.go implements TxStreamer: an abstraction that streams transaction
// status updates via a WebSocket connection when the provider supports it,
// and falls back transparently to periodic JSON-RPC polling over HTTPS.
package rpc

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// Transaction status values as returned by Soroban RPC getTransaction.
const (
	TxStatusPending  = "PENDING"
	TxStatusSuccess  = "SUCCESS"
	TxStatusFailed   = "FAILED"
	TxStatusNotFound = "NOT_FOUND"
)

// wsGUID is the magic constant defined by RFC 6455 §1.3 for the
// Sec-WebSocket-Accept header derivation.
const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// wsProbeTimeout is the maximum time allowed for probing WebSocket support.
const wsProbeTimeout = 3 * time.Second

// wsStreamInterval is the cadence at which the WebSocket streamer re-queries
// the remote endpoint while waiting for a non-PENDING result.
var wsStreamInterval = 2 * time.Second

// pollStreamInterval is the cadence used by the polling streamer.
var pollStreamInterval = 3 * time.Second

// TxStatus carries a single status snapshot for a Soroban transaction.
type TxStatus struct {
	Hash   string
	Status string // one of TxStatus{Pending,Success,Failed,NotFound}
	Ledger int64
	Error  string

	// XDR payload fields — populated only on SUCCESS or FAILED terminal states.
	EnvelopeXdr      string
	ResultXdr        string
	ResultMetaXdr    string
	ApplicationOrder int64
	LatestLedger     int64
	CreatedAt        int64
}

// IsFinal reports whether this status is a terminal state (no further
// updates will arrive for this transaction).
func (s TxStatus) IsFinal() bool {
	return s.Status == TxStatusSuccess || s.Status == TxStatusFailed
}

// TxStreamer streams per-transaction status updates until a final state is
// reached or the caller's context is cancelled.
type TxStreamer interface {
	// Stream opens a status channel for the transaction identified by hash.
	// The channel is always closed when the stream ends (final state, error,
	// or context cancellation). Callers must drain or discard the channel.
	Stream(ctx context.Context, hash string) (<-chan TxStatus, error)
}

// NewTxStreamer returns a TxStreamer backed by a WebSocket connection when the
// provider at client.SorobanURL supports the WebSocket upgrade, and falls back
// to JSON-RPC polling over HTTPS otherwise.
//
// The WebSocket probe is performed with a short timeout so that construction
// remains fast even when the provider does not support WebSocket.
func NewTxStreamer(c *Client) TxStreamer {
	wsURL := wsURLFor(c.SorobanURL)
	if wsURL != "" {
		probeCtx, cancel := context.WithTimeout(context.Background(), wsProbeTimeout)
		defer cancel()
		if probeWebSocket(probeCtx, wsURL, c.token) {
			logger.Logger.Info("WebSocket streaming enabled", "url", wsURL)
			return &wsStreamer{client: c, wsURL: wsURL}
		}
	}
	logger.Logger.Info("WebSocket not supported, using JSON-RPC polling", "url", c.SorobanURL)
	return &pollingStreamer{client: c}
}

// ---------------------------------------------------------------------------
// WebSocket streamer
// ---------------------------------------------------------------------------

// wsStreamer streams transaction status via a persistent WebSocket connection.
// It issues getTransaction JSON-RPC requests at wsStreamInterval and forwards
// each response to the output channel until a final status is received.
type wsStreamer struct {
	client *Client
	wsURL  string
}

// Stream implements TxStreamer.
func (s *wsStreamer) Stream(ctx context.Context, hash string) (<-chan TxStatus, error) {
	conn, err := wsDialUpgrade(ctx, s.wsURL, s.client.token)
	if err != nil {
		return nil, fmt.Errorf("ws streamer: dial: %w", err)
	}

	ch := make(chan TxStatus, 8)
	go func() {
		defer conn.close()
		defer close(ch)

		tracer := telemetry.GetTracer()
		sCtx, span := tracer.Start(ctx, "rpc_tx_stream_ws")
		span.SetAttributes(
			attribute.String("transaction.hash", hash),
			attribute.String("rpc.url", s.wsURL),
		)
		defer span.End()

		var reqID atomic.Int64
		ticker := time.NewTicker(wsStreamInterval)
		defer ticker.Stop()

		// Issue the first request immediately without waiting for the ticker.
		if done := s.poll(sCtx, conn, hash, reqID.Add(1), ch); done {
			return
		}

		for {
			select {
			case <-sCtx.Done():
				return
			case <-ticker.C:
				if done := s.poll(sCtx, conn, hash, reqID.Add(1), ch); done {
					return
				}
			}
		}
	}()

	return ch, nil
}

// poll sends one getTransaction request and forwards the response to ch.
// It returns true when the stream should terminate (final status or error).
func (s *wsStreamer) poll(ctx context.Context, conn *wsConn, hash string, id int64, ch chan<- TxStatus) bool {
	req := jsonrpcRequest{
		Jsonrpc: "2.0",
		ID:      id,
		Method:  "getTransaction",
		Params:  &rpcGetTxParams{Hash: hash},
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		// json.Marshal on a plain struct should never fail.
		logger.Logger.Error("ws streamer: marshal request", "error", err)
		return true
	}

	conn.raw.SetWriteDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	if err := wsWriteFrame(conn.raw, reqBytes); err != nil {
		logger.Logger.Warn("ws streamer: write frame", "error", err)
		return true
	}

	conn.raw.SetReadDeadline(time.Now().Add(15 * time.Second)) //nolint:errcheck
	data, err := wsReadMessage(conn.br)
	conn.raw.SetDeadline(time.Time{}) //nolint:errcheck
	if err != nil {
		logger.Logger.Warn("ws streamer: read message", "error", err)
		return true
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		logger.Logger.Warn("ws streamer: unmarshal response", "error", err)
		return false // malformed message — try again next tick
	}

	status := decodeTxStatus(hash, &resp)

	select {
	case ch <- status:
	case <-ctx.Done():
		return true
	}

	return status.IsFinal()
}

// ---------------------------------------------------------------------------
// Polling streamer (HTTPS JSON-RPC)
// ---------------------------------------------------------------------------

// pollingStreamer streams transaction status via repeated HTTPS JSON-RPC
// getTransaction requests at pollStreamInterval intervals.
type pollingStreamer struct {
	client *Client
}

// Stream implements TxStreamer.
func (s *pollingStreamer) Stream(ctx context.Context, hash string) (<-chan TxStatus, error) {
	ch := make(chan TxStatus, 8)
	go func() {
		defer close(ch)

		tracer := telemetry.GetTracer()
		sCtx, span := tracer.Start(ctx, "rpc_tx_stream_poll")
		span.SetAttributes(
			attribute.String("transaction.hash", hash),
			attribute.String("rpc.url", s.client.SorobanURL),
		)
		defer span.End()

		ticker := time.NewTicker(pollStreamInterval)
		defer ticker.Stop()

		// First check immediately.
		if done := s.check(sCtx, hash, ch); done {
			return
		}

		for {
			select {
			case <-sCtx.Done():
				return
			case <-ticker.C:
				if done := s.check(sCtx, hash, ch); done {
					return
				}
			}
		}
	}()

	return ch, nil
}

// check issues one getTransaction HTTP call and forwards the result to ch.
// Returns true when the stream should terminate.
func (s *pollingStreamer) check(ctx context.Context, hash string, ch chan<- TxStatus) bool {
	status, err := s.queryTxStatus(ctx, hash)
	if err != nil {
		logger.Logger.Warn("poll streamer: getTransaction failed", "hash", hash, "error", err)
		// Transient error — keep polling.
		return false
	}

	select {
	case ch <- status:
	case <-ctx.Done():
		return true
	}

	return status.IsFinal()
}

// queryTxStatus calls the Soroban RPC getTransaction method over HTTPS.
func (s *pollingStreamer) queryTxStatus(ctx context.Context, hash string) (TxStatus, error) {
	req := jsonrpcRequest{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "getTransaction",
		Params:  &rpcGetTxParams{Hash: hash},
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return TxStatus{}, fmt.Errorf("poll: marshal: %w", err)
	}

	targetURL := s.client.SorobanURL

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewBuffer(reqBytes))
	if err != nil {
		return TxStatus{}, fmt.Errorf("poll: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := s.client.getHTTPClient().Do(httpReq)
	if err != nil {
		return TxStatus{}, fmt.Errorf("poll: http: %w", err)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return TxStatus{}, fmt.Errorf("poll: read body: %w", err)
	}

	var rpcResp jsonrpcResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return TxStatus{}, fmt.Errorf("poll: unmarshal: %w", err)
	}

	return decodeTxStatus(hash, &rpcResp), nil
}

// ---------------------------------------------------------------------------
// WebSocket URL conversion
// ---------------------------------------------------------------------------

// wsURLFor converts an HTTP(S) Soroban RPC URL to the corresponding WebSocket
// URL (ws:// or wss://). It returns an empty string if url is empty.
func wsURLFor(httpURL string) string {
	switch {
	case strings.HasPrefix(httpURL, "https://"):
		return "wss://" + httpURL[len("https://"):]
	case strings.HasPrefix(httpURL, "http://"):
		return "ws://" + httpURL[len("http://"):]
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// WebSocket probe
// ---------------------------------------------------------------------------

// probeWebSocket attempts a WebSocket upgrade to wsURL and immediately closes
// the connection. It returns true only when the upgrade succeeds with a valid
// 101 Switching Protocols response.
func probeWebSocket(ctx context.Context, wsURL, token string) bool {
	conn, err := wsDialUpgrade(ctx, wsURL, token)
	if err != nil {
		return false
	}
	conn.close()
	return true
}

// ---------------------------------------------------------------------------
// WebSocket connection
// ---------------------------------------------------------------------------

// wsConn wraps a raw TCP/TLS connection after a successful HTTP upgrade.
// br buffers reads from raw so that any data already received during the
// upgrade handshake is not lost.
type wsConn struct {
	br  *bufio.Reader
	raw net.Conn
}

func (c *wsConn) close() {
	// Send a close frame before closing the underlying connection.
	c.raw.SetWriteDeadline(time.Now().Add(1 * time.Second)) //nolint:errcheck
	wsWriteFrame(c.raw, nil)                                //nolint:errcheck — best-effort
	c.raw.Close()
}

// ---------------------------------------------------------------------------
// WebSocket dial / HTTP upgrade
// ---------------------------------------------------------------------------

// wsDialUpgrade dials wsURL and performs the HTTP/1.1 → WebSocket upgrade
// handshake (RFC 6455). It returns a *wsConn ready for frame I/O.
func wsDialUpgrade(ctx context.Context, wsURL, token string) (*wsConn, error) {
	scheme, host, port, path, err := parseWSURL(wsURL)
	if err != nil {
		return nil, fmt.Errorf("ws: %w", err)
	}

	addr := net.JoinHostPort(host, port)
	var raw net.Conn

	switch scheme {
	case "wss":
		tlsDialer := &tls.Dialer{
			NetDialer: &net.Dialer{},
			Config:    &tls.Config{ServerName: host},
		}
		raw, err = tlsDialer.DialContext(ctx, "tcp", addr)
	default:
		d := &net.Dialer{}
		raw, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("ws: dial %s: %w", addr, err)
	}

	key := wsGenKey()

	var reqSB strings.Builder
	fmt.Fprintf(&reqSB, "GET %s HTTP/1.1\r\n", path)
	fmt.Fprintf(&reqSB, "Host: %s\r\n", net.JoinHostPort(host, port))
	reqSB.WriteString("Upgrade: websocket\r\n")
	reqSB.WriteString("Connection: Upgrade\r\n")
	fmt.Fprintf(&reqSB, "Sec-WebSocket-Key: %s\r\n", key)
	reqSB.WriteString("Sec-WebSocket-Version: 13\r\n")
	if token != "" {
		fmt.Fprintf(&reqSB, "Authorization: Bearer %s\r\n", token)
	}
	reqSB.WriteString("\r\n")

	if _, err := io.WriteString(raw, reqSB.String()); err != nil {
		raw.Close()
		return nil, fmt.Errorf("ws: send upgrade request: %w", err)
	}

	br := bufio.NewReader(raw)

	// Read status line.
	statusLine, err := br.ReadString('\n')
	if err != nil {
		raw.Close()
		return nil, fmt.Errorf("ws: read status line: %w", err)
	}
	if !strings.Contains(statusLine, "101") {
		raw.Close()
		return nil, fmt.Errorf("ws: expected 101 Switching Protocols, got: %s", strings.TrimSpace(statusLine))
	}

	// Read and validate headers.
	gotAccept := ""
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			raw.Close()
			return nil, fmt.Errorf("ws: read upgrade headers: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // blank line marks end of headers
		}
		if strings.HasPrefix(strings.ToLower(line), "sec-websocket-accept:") {
			gotAccept = strings.TrimSpace(line[len("sec-websocket-accept:"):])
		}
	}

	if want := wsAcceptKey(key); gotAccept != want {
		raw.Close()
		return nil, fmt.Errorf("ws: accept key mismatch: got %q, want %q", gotAccept, want)
	}

	return &wsConn{br: br, raw: raw}, nil
}

// ---------------------------------------------------------------------------
// WebSocket frame I/O (RFC 6455)
// ---------------------------------------------------------------------------

// wsWriteFrame sends a masked text frame to w.
// Passing a nil payload sends a close frame (opcode 0x8).
func wsWriteFrame(w io.Writer, payload []byte) error {
	var header [14]byte
	headerLen := 2

	if payload == nil {
		// Close frame: FIN=1, opcode=0x8, MASK=1, payload_len=0.
		header[0] = 0x88
		header[1] = 0x80
		var mask [4]byte
		if _, err := rand.Read(mask[:]); err != nil {
			return fmt.Errorf("ws: generate mask: %w", err)
		}
		copy(header[2:6], mask[:])
		_, err := w.Write(header[:6])
		return err
	}

	// Text frame: FIN=1, opcode=0x1.
	header[0] = 0x81

	pLen := len(payload)
	switch {
	case pLen < 126:
		header[1] = 0x80 | byte(pLen)
	case pLen <= 0xFFFF:
		header[1] = 0x80 | 126
		binary.BigEndian.PutUint16(header[2:4], uint16(pLen))
		headerLen = 4
	default:
		header[1] = 0x80 | 127
		binary.BigEndian.PutUint64(header[2:10], uint64(pLen))
		headerLen = 10
	}

	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return fmt.Errorf("ws: generate mask: %w", err)
	}
	copy(header[headerLen:headerLen+4], mask[:])
	headerLen += 4

	if _, err := w.Write(header[:headerLen]); err != nil {
		return fmt.Errorf("ws: write header: %w", err)
	}

	masked := make([]byte, pLen)
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}
	if _, err := w.Write(masked); err != nil {
		return fmt.Errorf("ws: write payload: %w", err)
	}
	return nil
}

// wsReadRawFrame reads one RFC 6455 frame from r and returns its components.
// It does not interpret the opcode — callers must dispatch accordingly.
func wsReadRawFrame(r *bufio.Reader) (fin bool, opcode byte, payload []byte, err error) {
	b0, e := r.ReadByte()
	if e != nil {
		return false, 0, nil, fmt.Errorf("ws: read header[0]: %w", e)
	}
	b1, e := r.ReadByte()
	if e != nil {
		return false, 0, nil, fmt.Errorf("ws: read header[1]: %w", e)
	}

	fin = (b0 & 0x80) != 0
	opcode = b0 & 0x0F
	masked := (b1 & 0x80) != 0

	rawLen := int64(b1 & 0x7F)
	switch rawLen {
	case 126:
		var ext [2]byte
		if _, e := io.ReadFull(r, ext[:]); e != nil {
			return false, 0, nil, fmt.Errorf("ws: read 16-bit length: %w", e)
		}
		rawLen = int64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, e := io.ReadFull(r, ext[:]); e != nil {
			return false, 0, nil, fmt.Errorf("ws: read 64-bit length: %w", e)
		}
		rawLen = int64(binary.BigEndian.Uint64(ext[:]))
	}

	var maskKey [4]byte
	if masked {
		if _, e := io.ReadFull(r, maskKey[:]); e != nil {
			return false, 0, nil, fmt.Errorf("ws: read mask key: %w", e)
		}
	}

	payload = make([]byte, rawLen)
	if _, e := io.ReadFull(r, payload); e != nil {
		return false, 0, nil, fmt.Errorf("ws: read payload: %w", e)
	}

	if masked {
		for i, b := range payload {
			payload[i] = b ^ maskKey[i%4]
		}
	}

	return fin, opcode, payload, nil
}

// wsReadFrame reads one complete (non-fragmented) data frame from r.
// Control frames (ping, pong) are discarded silently. A close frame
// returns io.EOF. For fragmented messages use wsReadMessage instead.
func wsReadFrame(r *bufio.Reader) ([]byte, error) {
	for {
		_, opcode, payload, err := wsReadRawFrame(r)
		if err != nil {
			return nil, err
		}
		switch opcode {
		case 0x8: // close
			return nil, io.EOF
		case 0x9, 0xA: // ping or pong — discard
			continue
		case 0x0, 0x1, 0x2: // continuation, text, binary
			return payload, nil
		default:
			return nil, fmt.Errorf("ws: unexpected opcode 0x%x", opcode)
		}
	}
}

// wsReadMessage reassembles a (possibly fragmented) WebSocket message per
// RFC 6455 §5.4. Control frames interleaved with fragments are handled
// silently. A close frame returns io.EOF.
func wsReadMessage(r *bufio.Reader) ([]byte, error) {
	var buf bytes.Buffer
	for {
		fin, opcode, payload, err := wsReadRawFrame(r)
		if err != nil {
			return nil, err
		}
		switch opcode {
		case 0x8: // close
			return nil, io.EOF
		case 0x9, 0xA: // ping/pong interleaved with fragments — discard
			continue
		case 0x0, 0x1, 0x2: // continuation, text, binary
			buf.Write(payload)
			if fin {
				return buf.Bytes(), nil
			}
		default:
			return nil, fmt.Errorf("ws: unexpected opcode 0x%x", opcode)
		}
	}
}

// ---------------------------------------------------------------------------
// WebSocket key helpers (RFC 6455 §4.1)
// ---------------------------------------------------------------------------

// wsGenKey generates a random 16-byte base64-encoded nonce for the
// Sec-WebSocket-Key request header.
func wsGenKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to a deterministic value — upgrade will still work but
		// the key uniqueness guarantee is lost.
		copy(b[:], "fallbackwskey123")
	}
	return base64.StdEncoding.EncodeToString(b[:])
}

// wsAcceptKey derives the expected Sec-WebSocket-Accept value for the given
// client key per RFC 6455 §4.2.2 step 5.4.
func wsAcceptKey(clientKey string) string {
	h := sha1.New()
	io.WriteString(h, clientKey+wsGUID) //nolint:errcheck — sha1.Write never fails
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ---------------------------------------------------------------------------
// URL helpers
// ---------------------------------------------------------------------------

// parseWSURL splits a WebSocket URL (ws:// or wss://) into its components.
func parseWSURL(rawURL string) (scheme, host, port, path string, err error) {
	// Manual parse to avoid importing net/url for a small helper.
	var rest string
	switch {
	case strings.HasPrefix(rawURL, "wss://"):
		scheme = "wss"
		rest = rawURL[len("wss://"):]
	case strings.HasPrefix(rawURL, "ws://"):
		scheme = "ws"
		rest = rawURL[len("ws://"):]
	default:
		return "", "", "", "", fmt.Errorf("unsupported WebSocket scheme in %q", rawURL)
	}

	// Split host[:port] from path.
	idx := strings.IndexByte(rest, '/')
	if idx < 0 {
		host = rest
		path = "/"
	} else {
		host = rest[:idx]
		path = rest[idx:]
	}
	if path == "" {
		path = "/"
	}

	// Split host and port.
	if h, p, e := net.SplitHostPort(host); e == nil {
		host = h
		port = p
	} else {
		// No explicit port.
		if scheme == "wss" {
			port = "443"
		} else {
			port = "80"
		}
	}

	return scheme, host, port, path, nil
}

// ---------------------------------------------------------------------------
// JSON-RPC types for getTransaction
// ---------------------------------------------------------------------------
type jsonrpcRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	ID      int64       `json:"id"`
	Params  interface{} `json:"params"`
}

type rpcGetTxParams struct {
	Hash string `json:"hash"`
}

// sorobanGetTxResult maps the full Soroban RPC getTransaction result object.
type sorobanGetTxResult struct {
	Status           string `json:"status"`
	Ledger           int64  `json:"ledger,omitempty"`
	EnvelopeXdr      string `json:"envelopeXdr,omitempty"`
	ResultXdr        string `json:"resultXdr,omitempty"`
	ResultMetaXdr    string `json:"resultMetaXdr,omitempty"`
	ApplicationOrder int64  `json:"applicationOrder,omitempty"`
	LatestLedger     int64  `json:"latestLedger,omitempty"`
	CreatedAt        int64  `json:"createdAt,omitempty"`
}

type jsonrpcResponse struct {
	Jsonrpc string              `json:"jsonrpc"`
	ID      int64               `json:"id"`
	Result  *sorobanGetTxResult `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// decodeTxStatus converts a JSON-RPC getTransaction response into a TxStatus,
// mapping RPC errors to NOT_FOUND and propagating all XDR fields on success.
func decodeTxStatus(hash string, resp *jsonrpcResponse) TxStatus {
	if resp.Error != nil {
		return TxStatus{Hash: hash, Status: TxStatusNotFound, Error: resp.Error.Message}
	}
	if resp.Result == nil {
		return TxStatus{Hash: hash, Status: TxStatusNotFound}
	}
	return TxStatus{
		Hash:             hash,
		Status:           resp.Result.Status,
		Ledger:           resp.Result.Ledger,
		EnvelopeXdr:      resp.Result.EnvelopeXdr,
		ResultXdr:        resp.Result.ResultXdr,
		ResultMetaXdr:    resp.Result.ResultMetaXdr,
		ApplicationOrder: resp.Result.ApplicationOrder,
		LatestLedger:     resp.Result.LatestLedger,
		CreatedAt:        resp.Result.CreatedAt,
	}
}
