// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
)

type telemetryCall struct {
	method     string
	attributes map[string]string
	err        error
}

type recordingMethodTelemetry struct {
	mu    sync.Mutex
	start []telemetryCall
	stop  []telemetryCall
}

func (r *recordingMethodTelemetry) StartMethodTimer(_ context.Context, method string, attributes map[string]string) MethodTimer {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.start = append(r.start, telemetryCall{
		method:     method,
		attributes: cloneAttributes(attributes),
	})

	return &recordingMethodTimer{
		parent:     r,
		method:     method,
		attributes: cloneAttributes(attributes),
	}
}

type recordingMethodTimer struct {
	parent     *recordingMethodTelemetry
	method     string
	attributes map[string]string
}

func (t *recordingMethodTimer) Stop(err error) {
	t.parent.mu.Lock()
	defer t.parent.mu.Unlock()
	t.parent.stop = append(t.parent.stop, telemetryCall{
		method:     t.method,
		attributes: cloneAttributes(t.attributes),
		err:        err,
	})
}

func cloneAttributes(attrs map[string]string) map[string]string {
	if attrs == nil {
		return nil
	}
	out := make(map[string]string, len(attrs))
	for k, v := range attrs {
		out[k] = v
	}
	return out
}

func TestWithMethodTelemetry(t *testing.T) {
	rec := &recordingMethodTelemetry{}
	client, err := NewClient(WithMethodTelemetry(rec))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.methodTelemetry != rec {
		t.Fatal("expected injected telemetry on client")
	}
}

func TestGetTransaction_ReportsMethodTelemetry(t *testing.T) {
	rec := &recordingMethodTelemetry{}
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (tx hProtocol.Transaction, err error) {
			return hProtocol.Transaction{
				EnvelopeXdr:   "env",
				ResultXdr:     "res",
				ResultMetaXdr: "meta",
			}, nil
		},
	}
	c := newTestClient(mock)
	c.methodTelemetry = rec

	_, err := c.GetTransaction(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(rec.start) != 1 || len(rec.stop) != 1 {
		t.Fatalf("expected 1 start/stop call, got starts=%d stops=%d", len(rec.start), len(rec.stop))
	}
	if rec.start[0].method != "rpc.get_transaction" {
		t.Fatalf("unexpected method: %s", rec.start[0].method)
	}
	if rec.stop[0].err != nil {
		t.Fatalf("expected nil error on successful stop, got %v", rec.stop[0].err)
	}
}

func TestSimulateTransaction_ReportsMethodTelemetryOnError(t *testing.T) {
	rec := &recordingMethodTelemetry{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		w.Write([]byte("too large"))
	}))
	defer server.Close()

	client, err := NewClient(
		WithNetwork(Testnet),
		WithHorizonURL(server.URL),
		WithSorobanURL(server.URL),
		WithMethodTelemetry(rec),
	)
	if err != nil {
		t.Fatalf("failed to build client: %v", err)
	}

	_, err = client.SimulateTransaction(context.Background(), "AAAA")
	if err == nil {
		t.Fatal("expected error")
	}

	if len(rec.start) != 1 || len(rec.stop) != 1 {
		t.Fatalf("expected 1 start/stop call, got starts=%d stops=%d", len(rec.start), len(rec.stop))
	}
	if rec.start[0].method != "rpc.simulate_transaction" {
		t.Fatalf("unexpected method: %s", rec.start[0].method)
	}
	if rec.stop[0].err == nil {
		t.Fatal("expected error captured in telemetry stop call")
	}
}

func TestSimulateTransaction_ReportsMethodTelemetryOnSuccess(t *testing.T) {
	rec := &recordingMethodTelemetry{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SimulateTransactionResponse{
			Jsonrpc: "2.0",
			ID:      1,
		}
		resp.Result.MinResourceFee = "1"
		resp.Result.TransactionData = "AAAA"
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(
		WithNetwork(Testnet),
		WithHorizonURL(server.URL),
		WithSorobanURL(server.URL),
		WithMethodTelemetry(rec),
	)
	if err != nil {
		t.Fatalf("failed to build client: %v", err)
	}

	_, err = client.SimulateTransaction(context.Background(), "AAAA")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(rec.start) != 1 || len(rec.stop) != 1 {
		t.Fatalf("expected 1 start/stop call, got starts=%d stops=%d", len(rec.start), len(rec.stop))
	}
	if rec.stop[0].err != nil {
		t.Fatalf("expected nil error on successful stop, got %v", rec.stop[0].err)
	}
}
