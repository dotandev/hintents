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
	"net/http/httptest"
	"testing"
	"time"

	stellarrpc "github.com/dotandev/hintents/internal/rpc"
)

func TestServer_DebugTransaction(t *testing.T) {
	// Set mock simulator path for testing
	t.Setenv("ERST_SIM_PATH", "/bin/echo")

	server, err := NewServer(Config{
		Network: string(stellarrpc.Testnet),
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	req := httptest.NewRequest("POST", "/rpc", nil)

	// Test the method directly
	var resp DebugTransactionResponse
	err = server.DebugTransaction(req, &DebugTransactionRequest{Hash: "test-hash"}, &resp)

	// We expect this to fail since it's a fake hash, but the method should handle it gracefully
	if err == nil {
		t.Error("Expected error for fake transaction hash")
	}
}

func TestServer_GetTrace(t *testing.T) {
	// Set mock simulator path for testing
	t.Setenv("ERST_SIM_PATH", "/bin/echo")

	server, err := NewServer(Config{
		Network: string(stellarrpc.Testnet),
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	req := httptest.NewRequest("POST", "/rpc", nil)
	var resp GetTraceResponse
	err = server.GetTrace(req, &GetTraceRequest{Hash: "test-hash"}, &resp)

	if err != nil {
		t.Fatalf("GetTrace failed: %v", err)
	}

	if resp.Hash != "test-hash" {
		t.Errorf("Expected hash 'test-hash', got '%s'", resp.Hash)
	}

	if len(resp.Traces) == 0 {
		t.Error("Expected traces to be returned")
	}
}

func TestServer_Authentication(t *testing.T) {
	// Set mock simulator path for testing
	t.Setenv("ERST_SIM_PATH", "/bin/echo")

	server, err := NewServer(Config{
		Network:   string(stellarrpc.Testnet),
		AuthToken: "secret123",
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test without auth token
	req := httptest.NewRequest("POST", "/rpc", nil)
	if server.authenticate(req) {
		t.Error("Expected authentication to fail without token")
	}

	// Test with correct Bearer token
	req.Header.Set("Authorization", "Bearer secret123")
	if !server.authenticate(req) {
		t.Error("Expected authentication to succeed with correct Bearer token")
	}

	// Test with correct direct token
	req.Header.Set("Authorization", "secret123")
	if !server.authenticate(req) {
		t.Error("Expected authentication to succeed with correct direct token")
	}

	// Test with wrong token
	req.Header.Set("Authorization", "wrong-token")
	if server.authenticate(req) {
		t.Error("Expected authentication to fail with wrong token")
	}
}

func TestServer_StartStop(t *testing.T) {
	// Set mock simulator path for testing
	t.Setenv("ERST_SIM_PATH", "/bin/echo")

	server, err := NewServer(Config{
		Network: string(stellarrpc.Testnet),
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start server (should stop after timeout)
	err = server.Start(ctx, "0") // Port 0 for random available port
	if err != nil {
		t.Fatalf("Server start failed: %v", err)
	}
}
