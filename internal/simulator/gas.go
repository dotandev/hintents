// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"fmt"

	"github.com/dotandev/hintents/internal/errors"
)

// ─── Fee estimation constants ─────────────────────────────────────────────────
// These are conservative heuristics for deriving a fee lower/upper bound from
// observed resource usage.  They will be replaced by exact network pricing once
// the fee configuration is exposed by the public Soroban RPC.

const (
	// BaseFeeStroops is the minimum network base fee per transaction.
	BaseFeeStroops int64 = 100

	// CPUStroopsPerUnit converts CPU instructions to stroops (1 stroop per 10 000 insns).
	CPUStroopsPerUnit uint64 = 10_000

	// MemStroopsPerUnit converts memory bytes to stroops (1 stroop per 64 KiB).
	MemStroopsPerUnit uint64 = 64 * 1024

	// UpperBoundMultiplierPercent applies a safety margin to the lower-bound fee
	// estimate.  A value of 115 means the upper bound is 115 % of the lower bound.
	UpperBoundMultiplierPercent int64 = 115

	// CPUWarningPercent is the threshold above which CPU usage is considered elevated.
	CPUWarningPercent float64 = 80.0

	// CPUCriticalPercent is the threshold above which CPU usage is considered critical.
	CPUCriticalPercent float64 = 95.0

	// MemWarningPercent is the threshold above which memory usage is considered elevated.
	MemWarningPercent float64 = 80.0

	// MemCriticalPercent is the threshold above which memory usage is considered critical.
	MemCriticalPercent float64 = 95.0
)

// GasEstimation provides a clean, focused view of simulated resource costs
// without requiring callers to parse the full SimulationResponse.
//
// Example usage:
//
//	resp, _ := runner.Run(req)
//	gas, _ := ExtractGasEstimation(resp)
//	fmt.Println(gas.CPUCost)                // raw CPU instructions consumed
//	fmt.Println(gas.MemoryCost)             // raw memory bytes consumed
//	fmt.Println(gas.EstimatedFeeLowerBound) // conservative fee in stroops
type GasEstimation struct {
	// CPUCost is the number of CPU instructions consumed during simulation.
	CPUCost uint64 `json:"cpu_cost"`

	// MemoryCost is the number of memory bytes consumed during simulation.
	MemoryCost uint64 `json:"memory_cost"`

	// CPULimit is the maximum allowed CPU instructions for this protocol version.
	CPULimit uint64 `json:"cpu_limit"`

	// MemoryLimit is the maximum allowed memory bytes for this protocol version.
	MemoryLimit uint64 `json:"memory_limit"`

	// CPUUsagePercent is the percentage of the CPU budget consumed (0–100).
	CPUUsagePercent float64 `json:"cpu_usage_percent"`

	// MemoryUsagePercent is the percentage of the memory budget consumed (0–100).
	MemoryUsagePercent float64 `json:"memory_usage_percent"`

	// OperationsCount is the number of host-function operations executed.
	OperationsCount int `json:"operations_count"`

	// EstimatedFeeLowerBound is a conservative lower-bound fee estimate in stroops.
	// Derived from: base fee + (cpu_instructions / 10 000) + (memory_bytes / 64 KiB).
	EstimatedFeeLowerBound int64 `json:"estimated_fee_lower_bound"`

	// EstimatedFeeUpperBound is an upper-bound fee estimate in stroops that includes
	// a safety margin (115 %) over the lower bound.
	EstimatedFeeUpperBound int64 `json:"estimated_fee_upper_bound"`
}

// ─── GasEstimation helper methods ─────────────────────────────────────────────

// IsCPUWarning returns true when CPU usage is at or above the warning threshold (80 %).
func (g *GasEstimation) IsCPUWarning() bool {
	return g.CPUUsagePercent >= CPUWarningPercent
}

// IsCPUCritical returns true when CPU usage is at or above the critical threshold (95 %).
func (g *GasEstimation) IsCPUCritical() bool {
	return g.CPUUsagePercent >= CPUCriticalPercent
}

// IsMemoryWarning returns true when memory usage is at or above the warning threshold (80 %).
func (g *GasEstimation) IsMemoryWarning() bool {
	return g.MemoryUsagePercent >= MemWarningPercent
}

// IsMemoryCritical returns true when memory usage is at or above the critical threshold (95 %).
func (g *GasEstimation) IsMemoryCritical() bool {
	return g.MemoryUsagePercent >= MemCriticalPercent
}

// HasBudgetPressure returns true when either CPU or memory usage is at warning level or above.
func (g *GasEstimation) HasBudgetPressure() bool {
	return g.IsCPUWarning() || g.IsMemoryWarning()
}

// String returns a human-readable one-line summary.
func (g *GasEstimation) String() string {
	cpuInd := ""
	if g.IsCPUCritical() {
		cpuInd = " [CRITICAL]"
	} else if g.IsCPUWarning() {
		cpuInd = " [WARNING]"
	}

	memInd := ""
	if g.IsMemoryCritical() {
		memInd = " [CRITICAL]"
	} else if g.IsMemoryWarning() {
		memInd = " [WARNING]"
	}

	return fmt.Sprintf(
		"GasEstimation{CPU: %d/%d (%.1f%%)%s, Memory: %d/%d (%.1f%%)%s, Ops: %d, Fee: %d–%d stroops}",
		g.CPUCost, g.CPULimit, g.CPUUsagePercent, cpuInd,
		g.MemoryCost, g.MemoryLimit, g.MemoryUsagePercent, memInd,
		g.OperationsCount,
		g.EstimatedFeeLowerBound, g.EstimatedFeeUpperBound,
	)
}

// ─── Extraction helpers ───────────────────────────────────────────────────────

// ExtractGasEstimation extracts a GasEstimation from a SimulationResponse,
// providing independent access to CPU and memory costs without parsing the
// full trace, events, or logs.
//
// Returns an error if the response is nil or does not contain budget usage data.
func ExtractGasEstimation(resp *SimulationResponse) (*GasEstimation, error) {
	if resp == nil {
		return nil, errors.WrapValidationError("simulation response is nil")
	}
	if resp.BudgetUsage == nil {
		return nil, errors.WrapSimulationLogicError("simulation response does not contain budget usage data")
	}
	return budgetToGasEstimation(resp.BudgetUsage)
}

// EstimateGas runs a simulation and returns only the gas estimation, discarding
// the rest of the response.  This is the preferred entry-point when the caller
// only needs cost / fee data.
//
// Example:
//
//	gas, err := simulator.EstimateGas(runner, req)
//	if err != nil { ... }
//	fmt.Printf("CPU: %d  Mem: %d  Fee: %d stroops\n",
//	    gas.CPUCost, gas.MemoryCost, gas.EstimatedFeeLowerBound)
func EstimateGas(runner RunnerInterface, req *SimulationRequest) (*GasEstimation, error) {
	if runner == nil {
		return nil, errors.WrapValidationError("runner is nil")
	}
	if req == nil {
		return nil, errors.WrapValidationError("simulation request is nil")
	}

	resp, err := runner.Run(req)
	if err != nil {
		return nil, fmt.Errorf("simulation failed: %w", err)
	}

	return ExtractGasEstimation(resp)
}

// ─── BudgetUsage conversion ──────────────────────────────────────────────────

// ToGasEstimation converts a BudgetUsage into a GasEstimation with derived fee
// estimates.  This is useful when the caller already has a BudgetUsage value.
func (b *BudgetUsage) ToGasEstimation() (*GasEstimation, error) {
	return budgetToGasEstimation(b)
}

// budgetToGasEstimation is the shared internal conversion.
func budgetToGasEstimation(b *BudgetUsage) (*GasEstimation, error) {
	lower, err := estimateFeeFromUsage(b.CPUInstructions, b.MemoryBytes)
	if err != nil {
		return nil, err
	}
	upper := lower * UpperBoundMultiplierPercent / 100

	return &GasEstimation{
		CPUCost:                b.CPUInstructions,
		MemoryCost:             b.MemoryBytes,
		CPULimit:               b.CPULimit,
		MemoryLimit:            b.MemoryLimit,
		CPUUsagePercent:        b.CPUUsagePercent,
		MemoryUsagePercent:     b.MemoryUsagePercent,
		OperationsCount:        b.OperationsCount,
		EstimatedFeeLowerBound: lower,
		EstimatedFeeUpperBound: upper,
	}, nil
}

// estimateFeeFromUsage computes a conservative fee estimate (stroops) from raw
// CPU and memory consumption.
func estimateFeeFromUsage(cpuInsns, memBytes uint64) (int64, error) {
	cpu := int64(cpuInsns / CPUStroopsPerUnit)
	mem := int64(memBytes / MemStroopsPerUnit)
	if cpu < 0 || mem < 0 {
		return 0, errors.WrapSimulationLogicError("invalid budget usage: negative derived cost")
	}
	return BaseFeeStroops + cpu + mem, nil
}
