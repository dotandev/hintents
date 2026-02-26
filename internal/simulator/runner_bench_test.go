// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/authtrace"
)

// ==================== Compute-Heavy Benchmarks ====================
// These benchmarks measure CPU and memory overhead for simulation processing

// BenchmarkSimulationRequestMarshal benchmarks JSON marshaling of simulation requests
func BenchmarkSimulationRequestMarshal(b *testing.B) {
	tests := []struct {
		name          string
		numLedgerKeys int
		hasAuthTrace  bool
	}{
		{"Small", 1, false},
		{"Medium", 10, false},
		{"Large", 50, false},
		{"WithAuthTrace", 10, true},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			req := &SimulationRequest{
				EnvelopeXdr:    strings.Repeat("e", 512),
				ResultMetaXdr:  strings.Repeat("m", 1024),
				LedgerEntries:  make(map[string]string, tt.numLedgerKeys),
				Timestamp:      1234567890,
				LedgerSequence: 12345,
				Profile:        false,
			}

			// Add ledger entries
			for i := 0; i < tt.numLedgerKeys; i++ {
				key := strings.Repeat("k", 64)
				value := strings.Repeat("v", 128)
				req.LedgerEntries[key] = value
			}

			// Add auth trace options if requested
			if tt.hasAuthTrace {
				req.AuthTraceOpts = &AuthTraceOptions{
					Enabled:              true,
					TraceCustomContracts: true,
					CaptureSigDetails:    true,
					MaxEventDepth:        10,
				}
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := json.Marshal(req)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkSimulationResponseUnmarshal benchmarks JSON unmarshaling of simulation responses
func BenchmarkSimulationResponseUnmarshal(b *testing.B) {
	tests := []struct {
		name         string
		numEvents    int
		hasBudget    bool
		hasAuthTrace bool
	}{
		{"Small", 5, false, false},
		{"Medium", 20, true, false},
		{"Large", 100, true, false},
		{"WithAuthTrace", 20, true, true},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			resp := SimulationResponse{
				Status: "success",
				Events: make([]string, tt.numEvents),
				Logs:   make([]string, tt.numEvents/2),
			}

			// Add events
			for i := 0; i < tt.numEvents; i++ {
				resp.Events[i] = strings.Repeat("event-data-", 10)
			}

			// Add logs
			for i := 0; i < len(resp.Logs); i++ {
				resp.Logs[i] = "log message " + strings.Repeat("x", 50)
			}

			data, _ := json.Marshal(resp)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var r SimulationResponse
				if err := json.Unmarshal(data, &r); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkStreamingVsBuffering compares memory allocation between buffering and streaming
func BenchmarkStreamingVsBuffering(b *testing.B) {
	req := &SimulationRequest{
		EnvelopeXdr:   strings.Repeat("e", 1024*1024), // 1MB
		ResultMetaXdr: strings.Repeat("m", 1024*1024), // 1MB
	}

	b.Run("Buffering", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Simulate buffering: Marshal -> Buffer -> Unmarshal
			data, _ := json.Marshal(req)
			var r SimulationRequest
			_ = json.Unmarshal(data, &r)
		}
	})

	b.Run("Streaming", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Simulate streaming: Encode -> Pipe -> Decode
			pr, pw := io.Pipe()
			go func() {
				_ = json.NewEncoder(pw).Encode(req)
				pw.Close()
			}()
			var r SimulationRequest
			_ = json.NewDecoder(pr).Decode(&r)
		}
	})
}
