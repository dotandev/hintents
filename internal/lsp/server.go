// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0
package lsp

import (
"bytes"
"context"
"encoding/json"
"errors"
"io"
"sync"
"time"

"go.lsp.dev/jsonrpc2"
"go.lsp.dev/protocol"
"go.opentelemetry.io/otel"
"go.opentelemetry.io/otel/metric"
)

var (
meter = otel.Meter("hintents.lsp")
latencyHistogram, _ = meter.Float64Histogram("lsp.request.latency", metric.WithUnit("ms"))
)

// Server provides a minimal LSP backend for Soroban hinting.
type Server struct {
mu sync.RWMutex
documents map[protocol.DocumentURI]string
connMu sync.Mutex
conn jsonrpc2.Conn
}

// NewServer creates a new LSP backend server.
func NewServer() *Server {
return &Server{
documents: make(map[protocol.DocumentURI]string),
}
}

// Run serves LSP requests over the provided JSON-RPC stream until the context
// is cancelled or the client disconnects.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
stream := jsonrpc2.NewStream(&readWriteCloser{Reader: r, Writer: w})
conn := jsonrpc2.NewConn(stream)
s.connMu.Lock()
s.conn = conn
s.connMu.Unlock()
conn.Go(ctx, s.handler())
for {
select {
case <-ctx.Done():
s.closeConnection()
<-conn.Done()
if err := conn.Err(); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
return err
}
return ctx.Err()
case <-conn.Done():
if err := conn.Err(); err != nil && !errors.Is(err, io.EOF) {
return err
}
return nil
}
}
}

func (s *Server) handler() jsonrpc2.Handler {
return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
start := time.Now()

if ctx.Err() != nil {
return reply(ctx, nil, protocol.ErrRequestCancelled)
}

// Record latency
duration := time.Since(start).Milliseconds()
latencyHistogram.Record(ctx, float64(duration), metric.WithAttributes(
metric.KeyValue{
Key:   "method",
Value: metric.StringValue(req.Method()),
},
))

return nil // full switch placeholder
}
}

// TODO: implement closeConnection and readWriteCloser
func (s *Server) closeConnection() {
// implementation
}

type readWriteCloser struct {
io.Reader
io.Writer
}

func (rwc *readWriteCloser) Close() error {
return nil
}