// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"testing"

	"github.com/dotandev/hintents/internal/config"
	"github.com/dotandev/hintents/internal/rpc"
)

func TestAddNetworkCommand_Execute(t *testing.T) {
	// Setup temporary config directory
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Test cases
	tests := []struct {
		name       string
		netName    string
		rpcURL     string
		passphrase string
		wantErr    bool
	}{
		{
			name:       "valid network",
			netName:    "test-net",
			rpcURL:     "https://localhost:8000",
			passphrase: "secret",
			wantErr:    false,
		},
		{
			name:       "valid network no passphrase",
			netName:    "public-net",
			rpcURL:     "https://horizon.stellar.org",
			passphrase: "",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewAddNetworkCommand()
			cmd.Name = tt.netName
			cmd.RPCURL = tt.rpcURL
			cmd.Passphrase = tt.passphrase

			err := cmd.Execute(context.Background(), []string{})
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify it was saved
			if !tt.wantErr {
				saved, err := config.GetCustomNetwork(tt.netName)
				if err != nil {
					t.Fatalf("failed to get saved network: %v", err)
				}
				if saved.HorizonURL != tt.rpcURL {
					t.Errorf("saved RPC URL = %v, want %v", saved.HorizonURL, tt.rpcURL)
				}
			}
		})
	}
}

func TestAddCommand_ProtocolVersion(t *testing.T) {
	cmd := NewAddCommand()
	if cmd.GetProtocolVersion() != 2 {
		t.Errorf("expected ProtocolVersion 2, got %d", cmd.GetProtocolVersion())
	}
}

func TestProtocolManager_Validate(t *testing.T) {
	pm := NewProtocolManager()
	
	tests := []struct {
		version uint32
		wantErr bool
	}{
		{20, false},
		{21, false},
		{22, false},
		{19, true},
		{23, true},
	}
	
	for _, tt := range tests {
		if err := pm.Validate(tt.version); (err != nil) != tt.wantErr {
			t.Errorf("Validate(%d) error = %v, wantErr %v", tt.version, err, tt.wantErr)
		}
	}
}
