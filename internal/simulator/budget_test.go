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
	"testing"
)

func TestBudgetUsageTracking(t *testing.T) {
	// Test that BudgetUsage fields are properly populated from JSON
	jsonResponse := `{
		"status": "success",
		"error": "",
		"events": [],
		"diagnostic_events": [],
		"categorized_events": [],
		"logs": ["Test log"],
		"budget_usage": {
			"cpu_instructions": 50000000,
			"memory_bytes": 25000000,
			"operations_count": 10,
			"cpu_limit": 100000000,
			"memory_limit": 50000000,
			"cpu_usage_percent": 50.0,
			"memory_usage_percent": 50.0
		}
	}`

	var resp SimulationResponse
	err := json.Unmarshal([]byte(jsonResponse), &resp)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify budget usage is present
	if resp.BudgetUsage == nil {
		t.Fatal("BudgetUsage should not be nil")
	}

	// Verify all fields are correctly populated
	bu := resp.BudgetUsage

	if bu.CPUInstructions != 50000000 {
		t.Errorf("Expected CPUInstructions=50000000, got %d", bu.CPUInstructions)
	}

	if bu.MemoryBytes != 25000000 {
		t.Errorf("Expected MemoryBytes=25000000, got %d", bu.MemoryBytes)
	}

	if bu.OperationsCount != 10 {
		t.Errorf("Expected OperationsCount=10, got %d", bu.OperationsCount)
	}

	if bu.CPULimit != 100000000 {
		t.Errorf("Expected CPULimit=100000000, got %d", bu.CPULimit)
	}

	if bu.MemoryLimit != 50000000 {
		t.Errorf("Expected MemoryLimit=50000000, got %d", bu.MemoryLimit)
	}

	if bu.CPUUsagePercent != 50.0 {
		t.Errorf("Expected CPUUsagePercent=50.0, got %.2f", bu.CPUUsagePercent)
	}

	if bu.MemoryUsagePercent != 50.0 {
		t.Errorf("Expected MemoryUsagePercent=50.0, got %.2f", bu.MemoryUsagePercent)
	}
}

func TestBudgetUsagePercentageCalculations(t *testing.T) {
	tests := []struct {
		name           string
		cpuUsed        uint64
		memUsed        uint64
		cpuLimit       uint64
		memLimit       uint64
		expectedCPUPct float64
		expectedMemPct float64
	}{
		{
			name:           "50% usage",
			cpuUsed:        50000000,
			memUsed:        25000000,
			cpuLimit:       100000000,
			memLimit:       50000000,
			expectedCPUPct: 50.0,
			expectedMemPct: 50.0,
		},
		{
			name:           "90% CPU usage (warning threshold)",
			cpuUsed:        90000000,
			memUsed:        10000000,
			cpuLimit:       100000000,
			memLimit:       50000000,
			expectedCPUPct: 90.0,
			expectedMemPct: 20.0,
		},
		{
			name:           "95% usage (critical threshold)",
			cpuUsed:        95000000,
			memUsed:        47500000,
			cpuLimit:       100000000,
			memLimit:       50000000,
			expectedCPUPct: 95.0,
			expectedMemPct: 95.0,
		},
		{
			name:           "Low usage",
			cpuUsed:        5000000,
			memUsed:        2500000,
			cpuLimit:       100000000,
			memLimit:       50000000,
			expectedCPUPct: 5.0,
			expectedMemPct: 5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bu := &BudgetUsage{
				CPUInstructions:    tt.cpuUsed,
				MemoryBytes:        tt.memUsed,
				CPULimit:           tt.cpuLimit,
				MemoryLimit:        tt.memLimit,
				CPUUsagePercent:    float64(tt.cpuUsed) / float64(tt.cpuLimit) * 100.0,
				MemoryUsagePercent: float64(tt.memUsed) / float64(tt.memLimit) * 100.0,
			}

			// Allow for small floating point differences (0.01%)
			if diff := bu.CPUUsagePercent - tt.expectedCPUPct; diff < -0.01 || diff > 0.01 {
				t.Errorf("CPU percentage mismatch: expected %.2f, got %.2f", tt.expectedCPUPct, bu.CPUUsagePercent)
			}

			if diff := bu.MemoryUsagePercent - tt.expectedMemPct; diff < -0.01 || diff > 0.01 {
				t.Errorf("Memory percentage mismatch: expected %.2f, got %.2f", tt.expectedMemPct, bu.MemoryUsagePercent)
			}
		})
	}
}

func TestBudgetUsageJSONMarshaling(t *testing.T) {
	// Create a BudgetUsage struct
	bu := &BudgetUsage{
		CPUInstructions:    75000000,
		MemoryBytes:        40000000,
		OperationsCount:    20,
		CPULimit:           100000000,
		MemoryLimit:        50000000,
		CPUUsagePercent:    75.0,
		MemoryUsagePercent: 80.0,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(bu)
	if err != nil {
		t.Fatalf("Failed to marshal BudgetUsage: %v", err)
	}

	// Unmarshal back
	var unmarshaled BudgetUsage
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal BudgetUsage: %v", err)
	}

	// Verify all fields match
	if unmarshaled.CPUInstructions != bu.CPUInstructions {
		t.Errorf("CPUInstructions mismatch after marshal/unmarshal")
	}
	if unmarshaled.MemoryBytes != bu.MemoryBytes {
		t.Errorf("MemoryBytes mismatch after marshal/unmarshal")
	}
	if unmarshaled.OperationsCount != bu.OperationsCount {
		t.Errorf("OperationsCount mismatch after marshal/unmarshal")
	}
	if unmarshaled.CPULimit != bu.CPULimit {
		t.Errorf("CPULimit mismatch after marshal/unmarshal")
	}
	if unmarshaled.MemoryLimit != bu.MemoryLimit {
		t.Errorf("MemoryLimit mismatch after marshal/unmarshal")
	}
	if unmarshaled.CPUUsagePercent != bu.CPUUsagePercent {
		t.Errorf("CPUUsagePercent mismatch after marshal/unmarshal")
	}
	if unmarshaled.MemoryUsagePercent != bu.MemoryUsagePercent {
		t.Errorf("MemoryUsagePercent mismatch after marshal/unmarshal")
	}
}
