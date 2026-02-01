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

package simulator

import (
	"encoding/json"
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

			// Add budget usage if requested
			if tt.hasBudget {
				resp.BudgetUsage = &BudgetUsage{
					CPUInstructions: 1000000,
					MemoryBytes:     5000000,
					OperationsCount: 50,
				}
			}

			// Add auth trace if requested
			if tt.hasAuthTrace {
				resp.AuthTrace = &authtrace.AuthTrace{
					Success:    true,
					AuthEvents: make([]authtrace.AuthEvent, 10),
				}
				for i := 0; i < 10; i++ {
					resp.AuthTrace.AuthEvents[i] = authtrace.AuthEvent{
						AccountID: "GA" + strings.Repeat("A", 54),
						SignerKey: "GA" + strings.Repeat("B", 54),
					}
				}
			}

			respBytes, _ := json.Marshal(resp)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var r SimulationResponse
				err := json.Unmarshal(respBytes, &r)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkStateInjection benchmarks ledger state injection into simulation request
func BenchmarkStateInjection(b *testing.B) {
	tests := []struct {
		name      string
		numStates int
	}{
		{"Single", 1},
		{"Small", 10},
		{"Medium", 50},
		{"Large", 100},
		{"VeryLarge", 1000},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			ledgerEntries := make(map[string]string, tt.numStates)
			for i := 0; i < tt.numStates; i++ {
				key := strings.Repeat("k", 64)
				value := strings.Repeat("v", 256) // Larger state values
				ledgerEntries[key] = value
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				req := &SimulationRequest{
					EnvelopeXdr:    "envelope",
					ResultMetaXdr:  "meta",
					LedgerEntries:  ledgerEntries,
					LedgerSequence: 12345,
				}
				if req.LedgerEntries == nil {
					b.Fatal("nil ledger entries")
				}
			}
		})
	}
}

// BenchmarkLargeEventParsing benchmarks parsing large diagnostic event arrays
func BenchmarkLargeEventParsing(b *testing.B) {
	tests := []struct {
		name      string
		numEvents int
	}{
		{"Small", 10},
		{"Medium", 100},
		{"Large", 500},
		{"VeryLarge", 1000},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Create response with many diagnostic events
			resp := SimulationResponse{
				Status:           "success",
				DiagnosticEvents: make([]DiagnosticEvent, tt.numEvents),
			}

			contractID := strings.Repeat("c", 56)
			for i := 0; i < tt.numEvents; i++ {
				resp.DiagnosticEvents[i] = DiagnosticEvent{
					EventType:                "contract",
					ContractID:               &contractID,
					Topics:                   []string{"topic1", "topic2", "topic3"},
					Data:                     strings.Repeat("d", 100),
					InSuccessfulContractCall: i%2 == 0,
				}
			}

			respBytes, _ := json.Marshal(resp)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var r SimulationResponse
				err := json.Unmarshal(respBytes, &r)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkAuthTraceProcessing benchmarks auth trace data processing
func BenchmarkAuthTraceProcessing(b *testing.B) {
	tests := []struct {
		name      string
		numEvents int
		maxDepth  int
	}{
		{"Small", 10, 3},
		{"Medium", 50, 5},
		{"Large", 200, 10},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			trace := &authtrace.AuthTrace{
				Success:    true,
				AuthEvents: make([]authtrace.AuthEvent, tt.numEvents),
			}

			for i := 0; i < tt.numEvents; i++ {
				trace.AuthEvents[i] = authtrace.AuthEvent{
					AccountID: "GA" + strings.Repeat("A", 54),
					SignerKey: "GA" + strings.Repeat("B", 54),
					Status:    "success",
					Details:   "some details about auth event",
				}
			}

			traceBytes, _ := json.Marshal(trace)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var t authtrace.AuthTrace
				err := json.Unmarshal(traceBytes, &t)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkBudgetUsageCalculation benchmarks budget usage metrics calculation
func BenchmarkBudgetUsageCalculation(b *testing.B) {
	// Simulate budget tracking overhead
	b.Run("Simple", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			budget := &BudgetUsage{
				CPUInstructions: uint64((i + 1) * 1000),
				MemoryBytes:     uint64((i + 1) * 5000),
				OperationsCount: (i % 100) + 1,
			}
			if budget.CPUInstructions == 0 {
				b.Fatal("zero cpu")
			}
		}
	})

	b.Run("WithJSON", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			budget := &BudgetUsage{
				CPUInstructions: uint64((i + 1) * 1000),
				MemoryBytes:     uint64((i + 1) * 5000),
				OperationsCount: (i % 100) + 1,
			}
			_, err := json.Marshal(budget)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkProtocolConfigApplication benchmarks protocol configuration application
func BenchmarkProtocolConfigApplication(b *testing.B) {
	runner := &Runner{
		BinaryPath: "/path/to/simulator",
		Debug:      false,
	}

	req := &SimulationRequest{
		EnvelopeXdr:   "envelope",
		ResultMetaXdr: "meta",
	}

	protocolVersion := uint32(20)
	proto := GetOrDefault(&protocolVersion)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err := runner.applyProtocolConfig(req, proto)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLedgerEntriesMapping benchmarks large ledger entry map creation
func BenchmarkLedgerEntriesMapping(b *testing.B) {
	tests := []struct {
		name       string
		numEntries int
	}{
		{"Small", 10},
		{"Medium", 100},
		{"Large", 500},
		{"VeryLarge", 1000},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Simulate converting RPC response to ledger entries map
			entries := make([]struct {
				Key string
				Xdr string
			}, tt.numEntries)

			for i := 0; i < tt.numEntries; i++ {
				entries[i].Key = "key-" + strings.Repeat("k", 32) + string(rune('a'+(i%26))) + string(rune('0'+(i/10)))
				entries[i].Xdr = strings.Repeat("x", 256)
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ledgerMap := make(map[string]string, len(entries))
				for _, entry := range entries {
					ledgerMap[entry.Key] = entry.Xdr
				}
				if len(ledgerMap) != tt.numEntries {
					b.Fatal("wrong size")
				}
			}
		})
	}
}

// BenchmarkCategorizedEventProcessing benchmarks categorized event processing
func BenchmarkCategorizedEventProcessing(b *testing.B) {
	tests := []struct {
		name      string
		numEvents int
	}{
		{"Small", 10},
		{"Medium", 50},
		{"Large", 200},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			events := make([]CategorizedEvent, tt.numEvents)
			contractID := strings.Repeat("c", 56)

			for i := 0; i < tt.numEvents; i++ {
				events[i] = CategorizedEvent{
					EventType:  "contract",
					ContractID: &contractID,
					Topics:     []string{"topic1", "topic2"},
					Data:       strings.Repeat("d", 100),
				}
			}

			eventsBytes, _ := json.Marshal(events)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var e []CategorizedEvent
				err := json.Unmarshal(eventsBytes, &e)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkSecurityViolationProcessing benchmarks security violation data processing
func BenchmarkSecurityViolationProcessing(b *testing.B) {
	violations := []SecurityViolation{
		{
			Type:        "unauthorized_access",
			Severity:    "high",
			Description: "Attempted unauthorized contract invocation",
			Contract:    strings.Repeat("c", 56),
			Details: map[string]interface{}{
				"function": "transfer",
				"caller":   strings.Repeat("a", 56),
				"attempt":  "direct_call",
			},
		},
		{
			Type:        "excessive_resources",
			Severity:    "medium",
			Description: "CPU limit exceeded",
			Contract:    strings.Repeat("c", 56),
			Details: map[string]interface{}{
				"cpu_used":  1000000,
				"cpu_limit": 500000,
			},
		},
	}

	violationsBytes, _ := json.Marshal(violations)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var v []SecurityViolation
		err := json.Unmarshal(violationsBytes, &v)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompleteSimulationWorkflow benchmarks the complete simulation data flow
func BenchmarkCompleteSimulationWorkflow(b *testing.B) {
	// Simulate a realistic end-to-end workflow
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Step 1: Create request
		req := &SimulationRequest{
			EnvelopeXdr:    strings.Repeat("e", 512),
			ResultMetaXdr:  strings.Repeat("m", 1024),
			LedgerEntries:  make(map[string]string, 10),
			LedgerSequence: 12345,
			AuthTraceOpts: &AuthTraceOptions{
				Enabled:              true,
				TraceCustomContracts: true,
			},
		}

		for j := 0; j < 10; j++ {
			req.LedgerEntries[strings.Repeat("k", 64)] = strings.Repeat("v", 128)
		}

		// Step 2: Marshal request
		reqBytes, err := json.Marshal(req)
		if err != nil {
			b.Fatal(err)
		}

		// Step 3: Simulate response (in real scenario this would be from Rust binary)
		resp := SimulationResponse{
			Status: "success",
			Events: make([]string, 20),
			BudgetUsage: &BudgetUsage{
				CPUInstructions: 1000000,
				MemoryBytes:     5000000,
				OperationsCount: 50,
			},
		}

		for j := 0; j < 20; j++ {
			resp.Events[j] = "event-" + strings.Repeat("e", 50)
		}

		// Step 4: Marshal response
		respBytes, err := json.Marshal(resp)
		if err != nil {
			b.Fatal(err)
		}

		// Step 5: Unmarshal response
		var finalResp SimulationResponse
		err = json.Unmarshal(respBytes, &finalResp)
		if err != nil {
			b.Fatal(err)
		}

		// Verify workflow
		if len(reqBytes) == 0 || len(respBytes) == 0 || finalResp.Status != "success" {
			b.Fatal("workflow failed")
		}
	}
}
