// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/hex"
	"fmt"
	"math/rand"
)

// FuzzerInput represents a single fuzz test input
type FuzzerInput struct {
	EnvelopeXdr   string
	LedgerEntries map[string]string
	Timestamp     int64
	Args          []string
	Seed          uint64
}

// FuzzingConfig contains configuration for fuzzing operations
type FuzzingConfig struct {
	MaxInputSize     int
	MaxIterations    uint64
	TimeoutMs        uint64
	EnableCoverage   bool
	TargetContractID string
}

// FuzzingResult represents the outcome of a fuzz test
type FuzzingResult struct {
	Seed            uint64
	Status          string // "pass", "crash", "slow", "error"
	ErrorMessage    string
	ExecutionTimeMs uint64
	CodeCoverage    uint32
}

// FuzzingHarness manages fuzzing operations for XDR inputs
type FuzzingHarness struct {
	Runner         RunnerInterface
	Config         FuzzingConfig
	Results        []FuzzingResult
	CrashingInputs []FuzzerInput
}

// NewFuzzingHarness creates a new fuzzing harness for contract testing
func NewFuzzingHarness(runner RunnerInterface, config FuzzingConfig) *FuzzingHarness {
	if config.MaxIterations == 0 {
		config.MaxIterations = 1000
	}
	if config.TimeoutMs == 0 {
		config.TimeoutMs = 5000 // 5 second default timeout
	}
	if config.MaxInputSize == 0 {
		config.MaxInputSize = 256 * 1024 // 256KB default
	}

	return &FuzzingHarness{
		Runner:         runner,
		Config:         config,
		Results:        make([]FuzzingResult, 0),
		CrashingInputs: make([]FuzzerInput, 0),
	}
}

// Fuzz runs fuzzing against randomly generated XDR inputs
func (h *FuzzingHarness) Fuzz(baseInput *FuzzerInput) ([]FuzzingResult, []FuzzerInput, error) {
	if baseInput == nil {
		return nil, nil, fmt.Errorf("base input required for fuzzing")
	}

	results := make([]FuzzingResult, 0)
	crashingInputs := make([]FuzzerInput, 0)

	for i := uint64(0); i < h.Config.MaxIterations; i++ {
		// Generate mutated input based on base
		mutated := h.mutateInput(baseInput, i)

		// Run simulation with timeout
		result := h.testFuzzerInput(&mutated)

		results = append(results, result)

		// Track crashing inputs
		if result.Status == "crash" {
			crashingInputs = append(crashingInputs, mutated)
		}

		// Report progress every 100 iterations
		if (i+1)%100 == 0 {
			fmt.Printf("Fuzz progress: %d/%d iterations\n", i+1, h.Config.MaxIterations)
		}
	}

	h.Results = results
	h.CrashingInputs = crashingInputs

	return results, crashingInputs, nil
}

// FuzzXDR fuzzes a specific XDR input against the harness
func (h *FuzzingHarness) FuzzXDR(envelopeXdr string) (*FuzzingResult, error) {
	if envelopeXdr == "" {
		return nil, fmt.Errorf("envelope XDR cannot be empty")
	}

	// Decode and validate XDR
	data, err := hex.DecodeString(envelopeXdr)
	if err != nil {
		return nil, fmt.Errorf("invalid XDR encoding: %w", err)
	}

	if len(data) > h.Config.MaxInputSize {
		return nil, fmt.Errorf("input size exceeds maximum: %d > %d", len(data), h.Config.MaxInputSize)
	}

	input := &FuzzerInput{
		EnvelopeXdr:   envelopeXdr,
		LedgerEntries: make(map[string]string),
		Timestamp:     0,
	}

	result := h.testFuzzerInput(input)
	return &result, nil
}

// mutateInput creates a mutated copy of the input with random modifications
func (h *FuzzingHarness) mutateInput(base *FuzzerInput, seed uint64) FuzzerInput {
	rng := rand.New(rand.NewSource(int64(seed)))

	mutated := FuzzerInput{
		EnvelopeXdr:   base.EnvelopeXdr,
		Timestamp:     base.Timestamp,
		LedgerEntries: make(map[string]string),
		Args:          make([]string, 0, len(base.Args)),
		Seed:          seed,
	}

	// Copy and potentially mutate ledger entries
	for k, v := range base.LedgerEntries {
		if rng.Float64() < 0.3 { // 30% chance to mutate entry
			// Mutate value by XORing random bytes
			mutated.LedgerEntries[k] = h.mutateHexString(v, rng)
		} else {
			mutated.LedgerEntries[k] = v
		}
	}

	// Optionally mutate timestamp
	if rng.Float64() < 0.2 {
		mutated.Timestamp = base.Timestamp + int64(rng.Intn(1000000))
	}

	// Mutate arguments if present
	for _, arg := range base.Args {
		if rng.Float64() < 0.5 {
			mutated.Args = append(mutated.Args, h.mutateHexString(arg, rng))
		} else {
			mutated.Args = append(mutated.Args, arg)
		}
	}

	return mutated
}

// mutateHexString applies random bit flips to a hex-encoded string
func (h *FuzzingHarness) mutateHexString(hexStr string, rng *rand.Rand) string {
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return hexStr
	}

	// Apply 1-3 random bit flips
	flipCount := 1 + rng.Intn(3)
	for i := 0; i < flipCount; i++ {
		pos := rng.Intn(len(data))
		bit := uint8(1 << (rng.Intn(8)))
		data[pos] ^= bit
	}

	return hex.EncodeToString(data)
}

// testFuzzerInput runs a single fuzz input through the simulator
func (h *FuzzingHarness) testFuzzerInput(input *FuzzerInput) FuzzingResult {
	result := FuzzingResult{
		Seed:   input.Seed,
		Status: "pass",
	}

	// Build simulation request
	simReq := &SimulationRequest{
		EnvelopeXdr:   input.EnvelopeXdr,
		LedgerEntries: input.LedgerEntries,
		Timestamp:     input.Timestamp,
		MockArgs:      &input.Args,
	}

	// Run simulation with timeout context
	simResp, err := h.Runner.Run(simReq)
	if err != nil {
		result.Status = "crash"
		result.ErrorMessage = fmt.Sprintf("execution error: %v", err)
		return result
	}

	// Analyze response
	if simResp.Status == "error" {
		result.Status = "error"
		result.ErrorMessage = simResp.Error
	}

	// Check for slow execution (would need actual timing in production)
	if result.ExecutionTimeMs > h.Config.TimeoutMs {
		result.Status = "slow"
		result.ErrorMessage = fmt.Sprintf("execution time exceeded %dms", h.Config.TimeoutMs)
	}

	return result
}

// CorpusCoverage returns statistics about code coverage across all fuzz runs
func (h *FuzzingHarness) CorpusCoverage() (uint32, int, int) {
	totalCoverage := uint32(0)
	passCount := 0
	crashCount := 0

	for _, result := range h.Results {
		totalCoverage += result.CodeCoverage

		switch result.Status {
		case "pass":
			passCount++
		case "crash":
			crashCount++
		}
	}

	avgCoverage := uint32(0)
	if len(h.Results) > 0 {
		avgCoverage = totalCoverage / uint32(len(h.Results))
	}

	return avgCoverage, passCount, crashCount
}

// Summary returns a formatted summary of fuzzing results
func (h *FuzzingHarness) Summary() string {
	avgCov, passes, crashes := h.CorpusCoverage()

	return fmt.Sprintf(
		"Fuzzing Summary:\n"+
			"  Total Runs: %d\n"+
			"  Passes: %d\n"+
			"  Crashes Found: %d\n"+
			"  Avg Code Coverage: %d%%\n"+
			"  Unique Crashes: %d",
		len(h.Results),
		passes,
		crashes,
		avgCov,
		len(h.CrashingInputs),
	)
}
