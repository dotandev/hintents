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

package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"runtime"
	"testing"
)

// BenchmarkDecodeSmallTrace benchmarks decoding a small trace (baseline ~10KB)
func BenchmarkDecodeSmallTrace(b *testing.B) {
	traceData := loadTestTrace(b, "testdata/small_trace.json")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var trace map[string]interface{}
		if err := json.Unmarshal(traceData, &trace); err != nil {
			b.Fatalf("Failed to decode trace: %v", err)
		}
	}
}

// BenchmarkDecodeMediumTrace benchmarks decoding a medium-sized trace (~100KB)
func BenchmarkDecodeMediumTrace(b *testing.B) {
	traceData := loadTestTrace(b, "testdata/medium_trace.json")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var trace map[string]interface{}
		if err := json.Unmarshal(traceData, &trace); err != nil {
			b.Fatalf("Failed to decode trace: %v", err)
		}
	}
}

// BenchmarkDecodeLargeTrace benchmarks decoding a large trace (~1MB+)
func BenchmarkDecodeLargeTrace(b *testing.B) {
	traceData := loadTestTrace(b, "testdata/large_trace.json")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var trace map[string]interface{}
		if err := json.Unmarshal(traceData, &trace); err != nil {
			b.Fatalf("Failed to decode trace: %v", err)
		}
	}
}

// BenchmarkDecodeDeeplyNestedTrace benchmarks decoding a trace with deep nesting
func BenchmarkDecodeDeeplyNestedTrace(b *testing.B) {
	traceData := loadTestTrace(b, "testdata/deeply_nested_trace.json")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var trace map[string]interface{}
		if err := json.Unmarshal(traceData, &trace); err != nil {
			b.Fatalf("Failed to decode trace: %v", err)
		}
	}
}

// BenchmarkDecodeParallel benchmarks parallel decoding of large traces
func BenchmarkDecodeParallel(b *testing.B) {
	traceData := loadTestTrace(b, "testdata/large_trace.json")

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var trace map[string]interface{}
			if err := json.Unmarshal(traceData, &trace); err != nil {
				b.Fatalf("Failed to decode trace: %v", err)
			}
		}
	})
}

// BenchmarkDecodeWithMemoryProfile benchmarks with memory profiling
func BenchmarkDecodeWithMemoryProfile(b *testing.B) {
	traceData := loadTestTrace(b, "testdata/large_trace.json")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	startAlloc := m.Alloc

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var trace map[string]interface{}
		if err := json.Unmarshal(traceData, &trace); err != nil {
			b.Fatalf("Failed to decode trace: %v", err)
		}
	}

	b.StopTimer()
	runtime.ReadMemStats(&m)
	b.ReportMetric(float64(m.Alloc-startAlloc)/float64(b.N), "B/decode")
}

// BenchmarkDecodeStreamingParser benchmarks using a streaming JSON parser
func BenchmarkDecodeStreamingParser(b *testing.B) {
	traceData := loadTestTrace(b, "testdata/large_trace.json")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		decoder := json.NewDecoder(bytes.NewReader(traceData))
		var trace map[string]interface{}
		if err := decoder.Decode(&trace); err != nil {
			b.Fatalf("Failed to decode trace: %v", err)
		}
	}
}

// BenchmarkDecodeWithStructuredOutput benchmarks decoding to a structured type
func BenchmarkDecodeWithStructuredOutput(b *testing.B) {
	traceData := loadTestTrace(b, "testdata/large_trace.json")

	type TransactionTrace struct {
		Hash         string                   `json:"hash"`
		Events       []map[string]interface{} `json:"events"`
		Diagnostics  []map[string]interface{} `json:"diagnostics"`
		LedgerNumber int64                    `json:"ledger_number"`
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var trace TransactionTrace
		if err := json.Unmarshal(traceData, &trace); err != nil {
			b.Fatalf("Failed to decode trace: %v", err)
		}
	}
}

// Helper function to load test trace data
func loadTestTrace(b *testing.B, filename string) []byte {
	b.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		b.Fatalf("Failed to load test trace from %s: %v", filename, err)
	}
	return data
}
