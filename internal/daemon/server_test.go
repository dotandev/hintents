// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package daemon

import (
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	stellarrpc "github.com/dotandev/hintents/internal/rpc"
)

// getTestSimPath returns a path to a mock simulator for testing.
// On Unix systems, it uses /bin/echo. On Windows, it uses cmd.exe.
// Returns empty string if no suitable mock is available.
func getTestSimPath() string {
	if runtime.GOOS == "windows" {
		// On Windows, use cmd.exe as a mock (it exists on all Windows systems)
		if path, err := exec.LookPath("cmd.exe"); err == nil {
			return path
		}
		return ""
	}
	// On Unix, use /bin/echo
	if _, err := os.Stat("/bin/echo"); err == nil {
		return "/bin/echo"
	}
	return ""
}

// skipIfNoSimulator skips the test if no simulator mock is available
func skipIfNoSimulator(t *testing.T) string {
	t.Helper()
	simPath := getTestSimPath()
	if simPath == "" {
		t.Skip("Skipping test: no simulator mock available")
	}
	return simPath
}

func TestServer_DebugTransaction(t *testing.T) {
	simPath := skipIfNoSimulator(t)
	t.Setenv("ERST_SIM_PATH", simPath)

	server, err := NewServer(Config{
		Network: string(stellarrpc.Testnet),
	})
	if err != nil {
		// Skip if simulator binary not found (expected in CI without erst-sim)
		if strings.Contains(err.Error(), "erst-sim binary not found") {
			t.Skip("Skipping test: erst-sim binary not found")
		}
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
	simPath := skipIfNoSimulator(t)
	t.Setenv("ERST_SIM_PATH", simPath)

	server, err := NewServer(Config{
		Network: string(stellarrpc.Testnet),
	})
	if err != nil {
		if strings.Contains(err.Error(), "erst-sim binary not found") {
			t.Skip("Skipping test: erst-sim binary not found")
		}
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
	simPath := skipIfNoSimulator(t)
	t.Setenv("ERST_SIM_PATH", simPath)

	server, err := NewServer(Config{
		Network:   string(stellarrpc.Testnet),
		AuthToken: "secret123",
	})
	if err != nil {
		if strings.Contains(err.Error(), "erst-sim binary not found") {
			t.Skip("Skipping test: erst-sim binary not found")
		}
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

	// Test with direct token (must be rejected — Bearer prefix required)
	req.Header.Set("Authorization", "secret123")
	if server.authenticate(req) {
		t.Error("Expected authentication to fail without Bearer prefix")
	}

	// Test with wrong token
	req.Header.Set("Authorization", "Bearer wrong-token")
	if server.authenticate(req) {
		t.Error("Expected authentication to fail with wrong token")
	}
}

func TestServer_StartStop(t *testing.T) {
	simPath := skipIfNoSimulator(t)
	t.Setenv("ERST_SIM_PATH", simPath)

	server, err := NewServer(Config{
		Network: string(stellarrpc.Testnet),
	})
	if err != nil {
		if strings.Contains(err.Error(), "erst-sim binary not found") {
			t.Skip("Skipping test: erst-sim binary not found")
		}
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

func TestServer_GetContractCode(t *testing.T) {
	t.Setenv("ERST_SIM_PATH", os.Args[0])

	server, err := NewServer(Config{
		Network: string(stellarrpc.Testnet),
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	req := httptest.NewRequest("POST", "/rpc", nil)
	var resp GetContractCodeResponse
	err = server.GetContractCode(req, &GetContractCodeRequest{
		ContractID: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
		TxHash:     "fake-tx-hash",
	}, &resp)

	if err == nil {
		t.Error("Expected error for fake transaction hash")
	}
}

func TestServer_GetContractCode_Auth(t *testing.T) {
	t.Setenv("ERST_SIM_PATH", os.Args[0])

	server, err := NewServer(Config{
		Network:   string(stellarrpc.Testnet),
		AuthToken: "secret-token",
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	req := httptest.NewRequest("POST", "/rpc", nil)
	var resp GetContractCodeResponse
	err = server.GetContractCode(req, &GetContractCodeRequest{
		ContractID: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
		TxHash:     "test-hash",
	}, &resp)

	if err == nil {
		t.Error("Expected auth error without token")
	}
}
