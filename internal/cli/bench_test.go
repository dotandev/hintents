// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"io"
	"testing"
)

func BenchmarkCommandStartup(b *testing.B) {
	// Benchmark the initialization of the command registry and add command
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cmd := NewAddCommand()
		_ = cmd.CreateCobraCommand()
	}
}

func BenchmarkAddNetworkExecution(b *testing.B) {
	// Benchmark the execution of the add network command
	// Mock config save to avoid disk I/O if possible, but here we test full flow
	// Use temp dir for each iteration or reset
	
	tmpDir := b.TempDir()
	b.Setenv("HOME", tmpDir)
	
	cmd := NewAddNetworkCommand()
	cmd.Name = "bench-net"
	cmd.RPCURL = "https://localhost:8000"
	
	ctx := context.Background()
	
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// We might be overwriting the same file repeatedly, which is fine for measuring overhead
		if err := cmd.Execute(ctx, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtocolValidation(b *testing.B) {
	pm := NewProtocolManager()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := pm.Validate(22); err != nil {
			b.Fatal(err)
		}
	}
}
