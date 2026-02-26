// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dotandev/hintents/internal/offline"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPassphraseForNetwork(t *testing.T) {
	tests := []struct {
		network  rpc.Network
		wantErr  bool
		contains string
	}{
		{rpc.Testnet, false, "Test SDF Network"},
		{rpc.Mainnet, false, "Public Global Stellar"},
		{rpc.Futurenet, false, "Future Network"},
		{"invalid", true, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.network), func(t *testing.T) {
			pp, err := passphraseForNetwork(tt.network)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Contains(t, pp, tt.contains)
			}
		})
	}
}

func TestBytesTrimSpaceOffline(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  hello  ", "hello"},
		{"\n\thello\r\n", "hello"},
		{"", ""},
		{"no-space", "no-space"},
	}

	for _, tt := range tests {
		got := string(bytesTrimSpaceOffline([]byte(tt.input)))
		assert.Equal(t, tt.expected, got)
	}
}

func TestRunOfflineGenerate(t *testing.T) {
	dir := t.TempDir()

	// Create a fake XDR file.
	xdrPath := filepath.Join(dir, "tx.xdr")
	require.NoError(t, os.WriteFile(xdrPath, []byte("AAAAAgAAAA=="), 0600))

	outputPath := filepath.Join(dir, "output.erst.json")

	// Set flags.
	offlineNetworkFlag = "testnet"
	offlineOutputFlag = outputPath
	offlineDescFlag = "test generate"
	offlineSourceFlag = "GABCDEF"

	err := runOfflineGenerate(nil, []string{xdrPath})
	require.NoError(t, err)

	// Verify the output file.
	ef, err := offline.LoadEnvelopeFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, "testnet", ef.Network)
	assert.Equal(t, "AAAAAgAAAA==", ef.EnvelopeXDR)
	assert.Equal(t, "test generate", ef.Metadata.Description)
	assert.False(t, ef.IsSigned())
}

func TestRunOfflineGenerate_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	xdrPath := filepath.Join(dir, "empty.xdr")
	require.NoError(t, os.WriteFile(xdrPath, []byte(""), 0600))

	offlineOutputFlag = filepath.Join(dir, "out.json")
	offlineNetworkFlag = "testnet"

	err := runOfflineGenerate(nil, []string{xdrPath})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestRunOfflineGenerate_FileNotFound(t *testing.T) {
	offlineNetworkFlag = "testnet"
	offlineOutputFlag = "/tmp/out.json"

	err := runOfflineGenerate(nil, []string{"/nonexistent/file.xdr"})
	assert.Error(t, err)
}
