// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/dotandev/hintents/internal/logger"
	stellarrpc "github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/telemetry"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
	"go.opentelemetry.io/otel/attribute"
)

// Server represents the JSON-RPC daemon server
type Server struct {
	rpcClient *stellarrpc.Client
	simulator *simulator.Runner
	authToken string
}

// Config holds daemon configuration
type Config struct {
	Port      string
	Network   string
	RPCURL    string
	AuthToken string
}

// DebugTransactionRequest represents the debug_transaction RPC request
type DebugTransactionRequest struct {
	Hash string `json:"hash"`
}

// DebugTransactionResponse represents the debug_transaction RPC response
type DebugTransactionResponse struct {
	Hash         string `json:"hash"`
	Network      string `json:"network"`
	EnvelopeSize int    `json:"envelope_size"`
	Status       string `json:"status"`
}

// GetTraceRequest represents the get_trace RPC request
type GetTraceRequest struct {
	Hash string `json:"hash"`
}

// GetTraceResponse represents the get_trace RPC response
type GetTraceResponse struct {
	Hash   string                   `json:"hash"`
	Traces []map[string]interface{} `json:"traces"`
}

// NewServer creates a new JSON-RPC server
func NewServer(config Config) (*Server, error) {
	var client *stellarrpc.Client
	if config.RPCURL != "" {
		client = stellarrpc.NewClientWithURL(config.RPCURL, stellarrpc.Network(config.Network), "")
	} else {
		client = stellarrpc.NewClient(stellarrpc.Network(config.Network), "")
	}

	sim, err := simulator.NewRunner("", false)
	if err != nil {
		return nil, fmt.Errorf("failed to create simulator: %w", err)
	}

	return &Server{
		rpcClient: client,
		simulator: sim,
		authToken: config.AuthToken,
	}, nil
}

// authenticate validates the authorization token
func (s *Server) authenticate(r *http.Request) bool {
	if s.authToken == "" {
		return true // No auth required
	}

	auth := r.Header.Get("Authorization")
	if auth == "" {
		return false
	}

	// Support "Bearer <token>" format
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		return token == s.authToken
	}

	return auth == s.authToken
}

// DebugTransaction handles debug_transaction RPC calls
func (s *Server) DebugTransaction(r *http.Request, req *DebugTransactionRequest, resp *DebugTransactionResponse) error {
	if !s.authenticate(r) {
		return fmt.Errorf("unauthorized")
	}

	ctx := r.Context()
	tracer := telemetry.GetTracer()
	ctx, span := tracer.Start(ctx, "rpc_debug_transaction")
	span.SetAttributes(attribute.String("transaction.hash", req.Hash))
	defer span.End()

	logger.Logger.Info("Processing debug_transaction RPC", "hash", req.Hash)

	// Fetch transaction details
	txResp, err := s.rpcClient.GetTransaction(ctx, req.Hash)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to fetch transaction: %w", err)
	}

	*resp = DebugTransactionResponse{
		Hash:         req.Hash,
		Network:      string(s.rpcClient.Network),
		EnvelopeSize: len(txResp.EnvelopeXdr),
		Status:       "success",
	}

	return nil
}

// GetTrace handles get_trace RPC calls
func (s *Server) GetTrace(r *http.Request, req *GetTraceRequest, resp *GetTraceResponse) error {
	if !s.authenticate(r) {
		return fmt.Errorf("unauthorized")
	}

	ctx := r.Context()
	tracer := telemetry.GetTracer()
	_, span := tracer.Start(ctx, "rpc_get_trace")
	span.SetAttributes(attribute.String("transaction.hash", req.Hash))
	defer span.End()

	logger.Logger.Info("Processing get_trace RPC", "hash", req.Hash)

	// For now, return mock trace data
	// In a full implementation, this would integrate with actual tracing
	*resp = GetTraceResponse{
		Hash: req.Hash,
		Traces: []map[string]interface{}{
			{
				"span_id":   "debug_transaction",
				"operation": "fetch_transaction",
				"duration":  "150ms",
				"status":    "success",
			},
		},
	}

	return nil
}

// Start starts the JSON-RPC server
func (s *Server) Start(ctx context.Context, port string) error {
	server := rpc.NewServer()
	server.RegisterCodec(json2.NewCodec(), "application/json")
	server.RegisterCodec(json2.NewCodec(), "application/json;charset=UTF-8")

	if err := server.RegisterService(s, ""); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	http.Handle("/rpc", server)

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	logger.Logger.Info("Starting JSON-RPC server", "port", port)

	srv := &http.Server{
		Addr: ":" + port,
	}

	// Start server in goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Logger.Error("Server failed", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	logger.Logger.Info("Shutting down JSON-RPC server")
	return srv.Shutdown(context.Background())
}
