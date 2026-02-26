// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ─── ExtractGasEstimation ────────────────────────────────────────────────────

func TestExtractGasEstimation_Success(t *testing.T) {
	resp := &SimulationResponse{
		Status: "success",
		BudgetUsage: &BudgetUsage{
			CPUInstructions:    50_000_000,
			MemoryBytes:        25_000_000,
			OperationsCount:    10,
			CPULimit:           100_000_000,
			MemoryLimit:        50_000_000,
			CPUUsagePercent:    50.0,
			MemoryUsagePercent: 50.0,
		},
	}

	gas, err := ExtractGasEstimation(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gas.CPUCost != 50_000_000 {
		t.Errorf("CPUCost: want 50000000, got %d", gas.CPUCost)
	}
	if gas.MemoryCost != 25_000_000 {
		t.Errorf("MemoryCost: want 25000000, got %d", gas.MemoryCost)
	}
	if gas.CPULimit != 100_000_000 {
		t.Errorf("CPULimit: want 100000000, got %d", gas.CPULimit)
	}
	if gas.MemoryLimit != 50_000_000 {
		t.Errorf("MemoryLimit: want 50000000, got %d", gas.MemoryLimit)
	}
	if gas.CPUUsagePercent != 50.0 {
		t.Errorf("CPUUsagePercent: want 50.0, got %.2f", gas.CPUUsagePercent)
	}
	if gas.MemoryUsagePercent != 50.0 {
		t.Errorf("MemoryUsagePercent: want 50.0, got %.2f", gas.MemoryUsagePercent)
	}
	if gas.OperationsCount != 10 {
		t.Errorf("OperationsCount: want 10, got %d", gas.OperationsCount)
	}
}

func TestExtractGasEstimation_NilResponse(t *testing.T) {
	_, err := ExtractGasEstimation(nil)
	if err == nil {
		t.Fatal("expected error for nil response")
	}
}

func TestExtractGasEstimation_NoBudget(t *testing.T) {
	resp := &SimulationResponse{Status: "success"}
	_, err := ExtractGasEstimation(resp)
	if err == nil {
		t.Fatal("expected error when BudgetUsage is nil")
	}
}

// ─── Fee estimation arithmetic ───────────────────────────────────────────────

func TestFeeEstimation_Arithmetic(t *testing.T) {
	tests := []struct {
		name         string
		cpuInsns     uint64
		memBytes     uint64
		wantLower    int64
		wantUpperGeq int64 // upper must be >= lower
	}{
		{
			name:      "zero usage",
			cpuInsns:  0,
			memBytes:  0,
			wantLower: BaseFeeStroops, // 100
		},
		{
			name:      "only CPU",
			cpuInsns:  100_000,
			memBytes:  0,
			wantLower: BaseFeeStroops + 10, // 100_000 / 10_000 = 10
		},
		{
			name:      "only memory",
			cpuInsns:  0,
			memBytes:  128 * 1024,
			wantLower: BaseFeeStroops + 2, // 128 KiB / 64 KiB = 2
		},
		{
			name:      "mixed realistic workload",
			cpuInsns:  50_000_000,
			memBytes:  25_000_000,
			wantLower: BaseFeeStroops + 5000 + 381, // 50M/10k + 25M/65536
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bu := &BudgetUsage{
				CPUInstructions:    tt.cpuInsns,
				MemoryBytes:        tt.memBytes,
				CPULimit:           100_000_000,
				MemoryLimit:        50_000_000,
				CPUUsagePercent:    float64(tt.cpuInsns) / 100_000_000 * 100,
				MemoryUsagePercent: float64(tt.memBytes) / 50_000_000 * 100,
				OperationsCount:    1,
			}

			gas, err := bu.ToGasEstimation()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gas.EstimatedFeeLowerBound != tt.wantLower {
				t.Errorf("lower bound: want %d, got %d", tt.wantLower, gas.EstimatedFeeLowerBound)
			}

			if gas.EstimatedFeeUpperBound < gas.EstimatedFeeLowerBound {
				t.Errorf("upper bound (%d) < lower bound (%d)", gas.EstimatedFeeUpperBound, gas.EstimatedFeeLowerBound)
			}

			expectedUpper := gas.EstimatedFeeLowerBound * UpperBoundMultiplierPercent / 100
			if gas.EstimatedFeeUpperBound != expectedUpper {
				t.Errorf("upper bound: want %d, got %d", expectedUpper, gas.EstimatedFeeUpperBound)
			}
		})
	}
}

// ─── Warning / critical thresholds ───────────────────────────────────────────

func TestGasEstimation_ThresholdHelpers(t *testing.T) {
	tests := []struct {
		name               string
		cpuPct             float64
		memPct             float64
		wantCPUWarning     bool
		wantCPUCritical    bool
		wantMemWarning     bool
		wantMemCritical    bool
		wantBudgetPressure bool
	}{
		{"low usage", 30.0, 20.0, false, false, false, false, false},
		{"CPU warning", 82.0, 20.0, true, false, false, false, true},
		{"mem warning", 20.0, 85.0, false, false, true, false, true},
		{"CPU critical", 96.0, 20.0, true, true, false, false, true},
		{"mem critical", 20.0, 97.0, false, false, true, true, true},
		{"both critical", 99.0, 99.0, true, true, true, true, true},
		{"exact warning boundary", 80.0, 80.0, true, false, true, false, true},
		{"exact critical boundary", 95.0, 95.0, true, true, true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gas := &GasEstimation{
				CPUUsagePercent:    tt.cpuPct,
				MemoryUsagePercent: tt.memPct,
			}

			if gas.IsCPUWarning() != tt.wantCPUWarning {
				t.Errorf("IsCPUWarning: want %v, got %v", tt.wantCPUWarning, gas.IsCPUWarning())
			}
			if gas.IsCPUCritical() != tt.wantCPUCritical {
				t.Errorf("IsCPUCritical: want %v, got %v", tt.wantCPUCritical, gas.IsCPUCritical())
			}
			if gas.IsMemoryWarning() != tt.wantMemWarning {
				t.Errorf("IsMemoryWarning: want %v, got %v", tt.wantMemWarning, gas.IsMemoryWarning())
			}
			if gas.IsMemoryCritical() != tt.wantMemCritical {
				t.Errorf("IsMemoryCritical: want %v, got %v", tt.wantMemCritical, gas.IsMemoryCritical())
			}
			if gas.HasBudgetPressure() != tt.wantBudgetPressure {
				t.Errorf("HasBudgetPressure: want %v, got %v", tt.wantBudgetPressure, gas.HasBudgetPressure())
			}
		})
	}
}

// ─── EstimateGas (with MockRunner) ───────────────────────────────────────────

func TestEstimateGas_WithMockRunner(t *testing.T) {
	mock := NewMockRunner(func(req *SimulationRequest) (*SimulationResponse, error) {
		return &SimulationResponse{
			Status: "success",
			BudgetUsage: &BudgetUsage{
				CPUInstructions:    80_000_000,
				MemoryBytes:        30_000_000,
				OperationsCount:    5,
				CPULimit:           100_000_000,
				MemoryLimit:        50_000_000,
				CPUUsagePercent:    80.0,
				MemoryUsagePercent: 60.0,
			},
		}, nil
	})

	req := &SimulationRequest{
		EnvelopeXdr:   "AAAA",
		ResultMetaXdr: "AAAA",
	}

	gas, err := EstimateGas(mock, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gas.CPUCost != 80_000_000 {
		t.Errorf("CPUCost: want 80000000, got %d", gas.CPUCost)
	}
	if gas.MemoryCost != 30_000_000 {
		t.Errorf("MemoryCost: want 30000000, got %d", gas.MemoryCost)
	}
	if !gas.IsCPUWarning() {
		t.Error("expected CPU warning at 80%")
	}
	if gas.EstimatedFeeLowerBound <= 0 {
		t.Errorf("expected positive fee, got %d", gas.EstimatedFeeLowerBound)
	}
}

func TestEstimateGas_NilRunner(t *testing.T) {
	_, err := EstimateGas(nil, &SimulationRequest{})
	if err == nil {
		t.Fatal("expected error for nil runner")
	}
}

func TestEstimateGas_NilRequest(t *testing.T) {
	mock := NewDefaultMockRunner()
	_, err := EstimateGas(mock, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestEstimateGas_RunnerError(t *testing.T) {
	mock := NewMockRunner(func(req *SimulationRequest) (*SimulationResponse, error) {
		return nil, fmt.Errorf("binary not found")
	})

	_, err := EstimateGas(mock, &SimulationRequest{EnvelopeXdr: "X", ResultMetaXdr: "Y"})
	if err == nil {
		t.Fatal("expected error when runner fails")
	}
}

func TestEstimateGas_NoBudgetInResponse(t *testing.T) {
	mock := NewDefaultMockRunner() // returns response without BudgetUsage

	_, err := EstimateGas(mock, &SimulationRequest{EnvelopeXdr: "X", ResultMetaXdr: "Y"})
	if err == nil {
		t.Fatal("expected error when budget usage is nil")
	}
}

// ─── String() ────────────────────────────────────────────────────────────────

func TestGasEstimation_String(t *testing.T) {
	gas := &GasEstimation{
		CPUCost:                50_000_000,
		MemoryCost:             25_000_000,
		CPULimit:               100_000_000,
		MemoryLimit:            50_000_000,
		CPUUsagePercent:        50.0,
		MemoryUsagePercent:     50.0,
		OperationsCount:        10,
		EstimatedFeeLowerBound: 5481,
		EstimatedFeeUpperBound: 6303,
	}

	s := gas.String()
	if !strings.Contains(s, "GasEstimation{") {
		t.Errorf("String() missing prefix: %s", s)
	}
	if !strings.Contains(s, "CPU:") {
		t.Errorf("String() missing CPU: %s", s)
	}
	if !strings.Contains(s, "Memory:") {
		t.Errorf("String() missing Memory: %s", s)
	}
	if !strings.Contains(s, "Fee:") {
		t.Errorf("String() missing Fee: %s", s)
	}
}

func TestGasEstimation_String_WithWarning(t *testing.T) {
	gas := &GasEstimation{
		CPUUsagePercent:    85.0,
		MemoryUsagePercent: 40.0,
	}
	s := gas.String()
	if !strings.Contains(s, "[WARNING]") {
		t.Errorf("expected [WARNING] in output: %s", s)
	}
}

func TestGasEstimation_String_WithCritical(t *testing.T) {
	gas := &GasEstimation{
		CPUUsagePercent:    96.0,
		MemoryUsagePercent: 97.0,
	}
	s := gas.String()
	count := strings.Count(s, "[CRITICAL]")
	if count != 2 {
		t.Errorf("expected 2x [CRITICAL], got %d in: %s", count, s)
	}
}

// ─── JSON round-trip ─────────────────────────────────────────────────────────

func TestGasEstimation_JSONRoundTrip(t *testing.T) {
	original := &GasEstimation{
		CPUCost:                42_000_000,
		MemoryCost:             18_000_000,
		CPULimit:               100_000_000,
		MemoryLimit:            50_000_000,
		CPUUsagePercent:        42.0,
		MemoryUsagePercent:     36.0,
		OperationsCount:        7,
		EstimatedFeeLowerBound: 4474,
		EstimatedFeeUpperBound: 5145,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded GasEstimation
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.CPUCost != original.CPUCost {
		t.Errorf("CPUCost mismatch: %d vs %d", decoded.CPUCost, original.CPUCost)
	}
	if decoded.MemoryCost != original.MemoryCost {
		t.Errorf("MemoryCost mismatch: %d vs %d", decoded.MemoryCost, original.MemoryCost)
	}
	if decoded.EstimatedFeeLowerBound != original.EstimatedFeeLowerBound {
		t.Errorf("EstimatedFeeLowerBound mismatch: %d vs %d", decoded.EstimatedFeeLowerBound, original.EstimatedFeeLowerBound)
	}
	if decoded.EstimatedFeeUpperBound != original.EstimatedFeeUpperBound {
		t.Errorf("EstimatedFeeUpperBound mismatch: %d vs %d", decoded.EstimatedFeeUpperBound, original.EstimatedFeeUpperBound)
	}
}

// ─── BudgetUsage.ToGasEstimation ─────────────────────────────────────────────

func TestBudgetUsage_ToGasEstimation(t *testing.T) {
	bu := &BudgetUsage{
		CPUInstructions:    75_000_000,
		MemoryBytes:        40_000_000,
		OperationsCount:    20,
		CPULimit:           100_000_000,
		MemoryLimit:        50_000_000,
		CPUUsagePercent:    75.0,
		MemoryUsagePercent: 80.0,
	}

	gas, err := bu.ToGasEstimation()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gas.CPUCost != bu.CPUInstructions {
		t.Errorf("CPUCost should equal CPUInstructions")
	}
	if gas.MemoryCost != bu.MemoryBytes {
		t.Errorf("MemoryCost should equal MemoryBytes")
	}
	if gas.OperationsCount != bu.OperationsCount {
		t.Errorf("OperationsCount should match")
	}
	if gas.EstimatedFeeLowerBound <= 0 {
		t.Errorf("fee lower bound should be positive")
	}
	if gas.EstimatedFeeUpperBound < gas.EstimatedFeeLowerBound {
		t.Errorf("fee upper bound should be >= lower bound")
	}
}
