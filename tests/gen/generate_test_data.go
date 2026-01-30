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

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TraceEvent represents a single trace event
type TraceEvent struct {
	Type       string                 `json:"type"`
	Timestamp  int64                  `json:"timestamp"`
	ContractID string                 `json:"contract_id"`
	Function   string                 `json:"function"`
	Args       []interface{}          `json:"args"`
	Result     interface{}            `json:"result,omitempty"`
	Error      *string                `json:"error,omitempty"`
	Gas        int64                  `json:"gas"`
	Topics     []string               `json:"topics"`
	Data       map[string]interface{} `json:"data"`
}

// DiagnosticEvent represents a diagnostic event
type DiagnosticEvent struct {
	Level   string                 `json:"level"`
	Message string                 `json:"message"`
	Context map[string]interface{} `json:"context"`
}

// TransactionTrace represents a complete transaction trace
type TransactionTrace struct {
	Hash           string            `json:"hash"`
	LedgerNumber   int64             `json:"ledger_number"`
	SourceAccount  string            `json:"source_account"`
	Fee            int64             `json:"fee"`
	SequenceNumber int64             `json:"sequence_number"`
	TimeBounds     map[string]int64  `json:"time_bounds"`
	Memo           string            `json:"memo"`
	Operations     []Operation       `json:"operations"`
	Events         []TraceEvent      `json:"events"`
	Diagnostics    []DiagnosticEvent `json:"diagnostics"`
	ExecutionTime  int64             `json:"execution_time_ms"`
	Success        bool              `json:"success"`
	ResultXDR      string            `json:"result_xdr"`
}

// Operation represents a transaction operation
type Operation struct {
	Type          string                 `json:"type"`
	SourceAccount string                 `json:"source_account,omitempty"`
	Body          map[string]interface{} `json:"body"`
}

func generateSmallTrace() *TransactionTrace {
	return &TransactionTrace{
		Hash:           "abc123def456" + "0000000000",
		LedgerNumber:   12345,
		SourceAccount:  "GABC123...",
		Fee:            100,
		SequenceNumber: 1,
		TimeBounds:     map[string]int64{"min": 0, "max": 999999999},
		Memo:           "Small test trace",
		Operations:     make([]Operation, 5),
		Events:         generateEvents(10, 2),
		Diagnostics:    generateDiagnostics(5),
		ExecutionTime:  15,
		Success:        false,
		ResultXDR:      "AAAAAAAAAAE=",
	}
}

func generateMediumTrace() *TransactionTrace {
	return &TransactionTrace{
		Hash:           "xyz789ghi012" + "0000000000",
		LedgerNumber:   54321,
		SourceAccount:  "GXYZ789...",
		Fee:            500,
		SequenceNumber: 100,
		TimeBounds:     map[string]int64{"min": 0, "max": 999999999},
		Memo:           "Medium test trace",
		Operations:     generateOperations(25),
		Events:         generateEvents(100, 5),
		Diagnostics:    generateDiagnostics(50),
		ExecutionTime:  250,
		Success:        false,
		ResultXDR:      "AAAAAAAAAAE=",
	}
}

func generateLargeTrace() *TransactionTrace {
	return &TransactionTrace{
		Hash:           "large123hash456" + "0000000000",
		LedgerNumber:   98765,
		SourceAccount:  "GLARGE123...",
		Fee:            1000,
		SequenceNumber: 500,
		TimeBounds:     map[string]int64{"min": 0, "max": 999999999},
		Memo:           "Large test trace - 1MB+",
		Operations:     generateOperations(100),
		Events:         generateEvents(1000, 10),
		Diagnostics:    generateDiagnostics(500),
		ExecutionTime:  5000,
		Success:        false,
		ResultXDR:      "AAAAAAAAAAE=",
	}
}

func generateDeeplyNestedTrace() *TransactionTrace {
	trace := &TransactionTrace{
		Hash:           "nested123deep456" + "0000000000",
		LedgerNumber:   11111,
		SourceAccount:  "GNESTED...",
		Fee:            750,
		SequenceNumber: 250,
		TimeBounds:     map[string]int64{"min": 0, "max": 999999999},
		Memo:           "Deeply nested test trace",
		Operations:     generateOperations(50),
		Events:         generateDeeplyNestedEvents(200, 15),
		Diagnostics:    generateDiagnostics(100),
		ExecutionTime:  3000,
		Success:        false,
		ResultXDR:      "AAAAAAAAAAE=",
	}
	return trace
}

func generateOperations(count int) []Operation {
	ops := make([]Operation, count)
	for i := 0; i < count; i++ {
		ops[i] = Operation{
			Type:          "invoke_contract",
			SourceAccount: fmt.Sprintf("GACCOUNT%d...", i),
			Body: map[string]interface{}{
				"contract_id": fmt.Sprintf("CONTRACT%d", i),
				"function":    fmt.Sprintf("function_%d", i),
				"args": []interface{}{
					fmt.Sprintf("arg_%d_1", i),
					fmt.Sprintf("arg_%d_2", i),
					i * 100,
				},
			},
		}
	}
	return ops
}

func generateEvents(count, dataSize int) []TraceEvent {
	events := make([]TraceEvent, count)
	for i := 0; i < count; i++ {
		data := make(map[string]interface{})
		for j := 0; j < dataSize; j++ {
			data[fmt.Sprintf("key_%d", j)] = fmt.Sprintf("value_%d_%d", i, j)
		}

		events[i] = TraceEvent{
			Type:       "contract_event",
			Timestamp:  int64(1000000 + i*1000),
			ContractID: fmt.Sprintf("CONTRACT_%d", i%10),
			Function:   fmt.Sprintf("function_%d", i%20),
			Args: []interface{}{
				fmt.Sprintf("arg_%d", i),
				i * 10,
				true,
			},
			Gas:    int64(1000 + i*10),
			Topics: []string{fmt.Sprintf("topic_%d", i%5), fmt.Sprintf("topic_%d", i%3)},
			Data:   data,
		}
	}
	return events
}

func generateDeeplyNestedEvents(count, nestingDepth int) []TraceEvent {
	events := make([]TraceEvent, count)
	for i := 0; i < count; i++ {
		data := generateNestedMap(nestingDepth, i)

		events[i] = TraceEvent{
			Type:       "nested_contract_event",
			Timestamp:  int64(1000000 + i*1000),
			ContractID: fmt.Sprintf("NESTED_CONTRACT_%d", i%10),
			Function:   fmt.Sprintf("nested_function_%d", i%20),
			Args:       []interface{}{generateNestedMap(nestingDepth/2, i)},
			Gas:        int64(5000 + i*50),
			Topics:     []string{fmt.Sprintf("nested_topic_%d", i%5)},
			Data:       data,
		}
	}
	return events
}

func generateNestedMap(depth, seed int) map[string]interface{} {
	if depth == 0 {
		return map[string]interface{}{
			"value":     fmt.Sprintf("leaf_value_%d", seed),
			"number":    seed * 100,
			"timestamp": int64(1000000 + seed),
		}
	}

	return map[string]interface{}{
		"level":  depth,
		"nested": generateNestedMap(depth-1, seed),
		"data": map[string]interface{}{
			"key1": fmt.Sprintf("value_%d_%d", depth, seed),
			"key2": depth * seed,
		},
		"array": []interface{}{
			fmt.Sprintf("item_%d_1", depth),
			fmt.Sprintf("item_%d_2", depth),
			generateNestedMap(depth-1, seed+1),
		},
	}
}

func generateDiagnostics(count int) []DiagnosticEvent {
	diagnostics := make([]DiagnosticEvent, count)
	levels := []string{"DEBUG", "INFO", "WARNING", "ERROR"}

	for i := 0; i < count; i++ {
		diagnostics[i] = DiagnosticEvent{
			Level:   levels[i%len(levels)],
			Message: fmt.Sprintf("Diagnostic message %d: Operation executed with result", i),
			Context: map[string]interface{}{
				"step":      i,
				"gas_used":  i * 100,
				"timestamp": int64(1000000 + i*500),
				"contract":  fmt.Sprintf("CONTRACT_%d", i%10),
				"details": map[string]interface{}{
					"function":   fmt.Sprintf("func_%d", i%15),
					"args_count": i % 5,
				},
			},
		}
	}
	return diagnostics
}

func writeTraceToFile(trace *TransactionTrace, filename string) error {
	data, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal trace: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Generated %s (%.2f KB)\n", filename, float64(len(data))/1024)
	return nil
}

func main() {
	// Create testdata directory if it doesn't exist
	if err := os.MkdirAll("testdata", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create testdata directory: %v\n", err)
		os.Exit(1)
	}

	traces := map[string]*TransactionTrace{
		"small_trace.json":         generateSmallTrace(),
		"medium_trace.json":        generateMediumTrace(),
		"large_trace.json":         generateLargeTrace(),
		"deeply_nested_trace.json": generateDeeplyNestedTrace(),
	}

	for filename, trace := range traces {
		fullPath := filepath.Join("testdata", filename)
		if err := writeTraceToFile(trace, fullPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating %s: %v\n", filename, err)
			os.Exit(1)
		}
	}

	fmt.Println("\nAll test traces generated successfully!")
}
