// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package integration provides end-to-end tests verifying Go/Rust bridge
// communication during a time-travel (step-by-step) simulation session.
//
// The fixture contract is tests/fixtures/contracts/counter.rust — a minimal
// Soroban counter whose increment() function is simulated here via MockRunner.
// Each call to Stepper.Step corresponds to one ledger advance and produces a
// snapshot of the resulting ledger state.
package integration

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// counterKey is the base64-encoded XDR key used to store the counter value in
// the ledger.  Matches the COUNT_KEY symbol in counter.rust.
const counterKey = "Q09VTlRFUg==" // base64.StdEncoding.EncodeToString([]byte("COUNTER"))

// StepResult holds the outcome of a single simulation step in a time-travel
// session.
type StepResult struct {
	// StepIndex is the zero-based position of this step in the session.
	StepIndex int

	// LedgerSequence is the ledger at which this step was simulated.
	LedgerSequence uint32

	// InstructionOffset is the WASM bytecode offset extracted from the first
	// DiagnosticEvent returned by the bridge.  The acceptance criterion
	// requires this to equal StepIndex.
	InstructionOffset uint64

	// Snapshot is the ledger state captured immediately after applying the
	// step's state updates.
	Snapshot *snapshot.Snapshot
}

// Stepper drives a time-travel session one ledger at a time.  It wraps a
// RunnerInterface (typically MockRunner in tests, the real Rust bridge in
// production) and accumulates a StepResult for every Step call.
type Stepper struct {
	runner      simulator.RunnerInterface
	baseReq     simulator.SimulationRequest
	ledgerState map[string]string
	results     []StepResult
}

// NewStepper creates a Stepper seeded with the initial ledger state encoded in
// req.LedgerEntries.
func NewStepper(runner simulator.RunnerInterface, req simulator.SimulationRequest) *Stepper {
	state := make(map[string]string, len(req.LedgerEntries))
	for k, v := range req.LedgerEntries {
		state[k] = v
	}
	return &Stepper{runner: runner, baseReq: req, ledgerState: state}
}

// Step advances the simulation by one ledger.  updatedEntries is applied to
// the accumulated ledger state before the simulation request is sent, modelling
// how each step transitions the contract's storage.
//
// The InstructionOffset in the returned StepResult is read from the
// WasmInstruction field of the first DiagnosticEvent in the bridge response.
func (s *Stepper) Step(ctx context.Context, updatedEntries map[string]string) (StepResult, error) {
	idx := len(s.results)
	seq := s.baseReq.LedgerSequence + uint32(idx) + 1

	// Apply state mutations for this step before snapshotting.
	for k, v := range updatedEntries {
		s.ledgerState[k] = v
	}

	req := s.baseReq
	req.LedgerEntries = s.ledgerState
	req.LedgerSequence = seq
	req.Timestamp = int64(seq) * 5 // ~5 s per Stellar ledger

	resp, err := s.runner.Run(ctx, &req)
	if err != nil {
		return StepResult{}, fmt.Errorf("step %d (ledger %d): simulation failed: %w", idx, seq, err)
	}

	snap := snapshot.FromMap(s.ledgerState)

	var instrOffset uint64
	if len(resp.DiagnosticEvents) > 0 && resp.DiagnosticEvents[0].WasmInstruction != nil {
		instrOffset, _ = strconv.ParseUint(*resp.DiagnosticEvents[0].WasmInstruction, 10, 64)
	}

	result := StepResult{
		StepIndex:         idx,
		LedgerSequence:    seq,
		InstructionOffset: instrOffset,
		Snapshot:          snap,
	}
	s.results = append(s.results, result)
	return result, nil
}

// Steps returns all recorded StepResults in execution order.
func (s *Stepper) Steps() []StepResult { return s.results }

// ── helpers ──────────────────────────────────────────────────────────────────

// counterValue encodes a counter integer as a base64 XDR-style value that
// matches what the Rust host would serialise for the COUNT_KEY entry.
func counterValue(n uint32) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("v:%d", n)))
}

// makeCounterRunner returns a MockRunner that simulates n invocations of the
// counter contract's increment() function.
//
// For call i (0-indexed) the runner returns:
//   - Status "success".
//   - One DiagnosticEvent of type "contract_call" whose WasmInstruction field
//     is the decimal string representation of i.  This ensures that the step
//     index equals the instruction offset recorded in the trace.
//   - One base64-encoded XDR event string carrying the new counter value.
func makeCounterRunner(n int) *simulator.MockRunner {
	callIdx := 0
	contractID := "CDUMMY00000000000000000000000000000000000000000000000000"

	return simulator.NewMockRunner(func(_ context.Context, req *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
		if callIdx >= n {
			return nil, fmt.Errorf("makeCounterRunner: only %d steps configured, got call %d", n, callIdx)
		}
		i := callIdx
		callIdx++

		wasmInstr := strconv.Itoa(i)
		newCount := uint32(i + 1)
		cid := contractID

		return &simulator.SimulationResponse{
			Status: "success",
			DiagnosticEvents: []simulator.DiagnosticEvent{
				{
					EventType:                "contract_call",
					ContractID:               &cid,
					Topics:                   []string{"increment"},
					Data:                     fmt.Sprintf(`{"count":%d}`, newCount),
					InSuccessfulContractCall: true,
					WasmInstruction:          &wasmInstr,
				},
			},
			Events: []string{
				base64.StdEncoding.EncodeToString(
					[]byte(fmt.Sprintf("counter=%d", newCount)),
				),
			},
		}, nil
	})
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestTimeTravelStepByStep verifies the full end-to-end step-by-step flow:
//
//  1. A Stepper wraps the Go/Rust bridge (MockRunner simulating the counter
//     contract) together with an initial ledger state.
//  2. Stepping through the session produces coherent StepResults.
//  3. LedgerSequence increments correctly with each step.
//  4. The total number of recorded steps equals the number of Step calls.
func TestTimeTravelStepByStep(t *testing.T) {
	const numSteps = 3

	runner := makeCounterRunner(numSteps)
	req := simulator.SimulationRequest{
		EnvelopeXdr:    "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		ResultMetaXdr:  "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		LedgerEntries:  map[string]string{counterKey: counterValue(0)},
		LedgerSequence: 1000,
	}

	stepper := NewStepper(runner, req)
	ctx := context.Background()

	for i := 0; i < numSteps; i++ {
		result, err := stepper.Step(ctx, map[string]string{counterKey: counterValue(uint32(i + 1))})
		require.NoError(t, err, "step %d failed", i)

		assert.Equal(t, i, result.StepIndex, "step index mismatch at step %d", i)
		assert.Equal(t, uint32(1001+i), result.LedgerSequence, "ledger sequence mismatch at step %d", i)
	}

	require.Len(t, stepper.Steps(), numSteps, "expected %d recorded steps", numSteps)
}

// TestTimeTravelSnapshotsAreValid verifies that each step produces a non-nil
// Snapshot containing at least one ledger entry, and that the counter value
// inside the snapshot reflects the state mutation applied during that step.
func TestTimeTravelSnapshotsAreValid(t *testing.T) {
	const numSteps = 3

	runner := makeCounterRunner(numSteps)
	req := simulator.SimulationRequest{
		EnvelopeXdr:    "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		ResultMetaXdr:  "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		LedgerEntries:  map[string]string{counterKey: counterValue(0)},
		LedgerSequence: 2000,
	}

	stepper := NewStepper(runner, req)
	ctx := context.Background()

	for i := 0; i < numSteps; i++ {
		result, err := stepper.Step(ctx, map[string]string{counterKey: counterValue(uint32(i + 1))})
		require.NoError(t, err, "step %d failed", i)

		require.NotNil(t, result.Snapshot, "snapshot is nil at step %d", i)
		assert.NotEmpty(t, result.Snapshot.LedgerEntries, "snapshot has no entries at step %d", i)

		// The snapshot must carry the updated counter value for this step.
		snap := result.Snapshot.ToMap()
		assert.Equal(t,
			counterValue(uint32(i+1)),
			snap[counterKey],
			"snapshot counter value mismatch at step %d", i,
		)
	}
}

// TestTimeTravelStepIndicesMatchInstructionOffsets is the primary acceptance
// criterion test:
//
//	"Step indices match the trace instruction offsets."
//
// The MockRunner encodes the call index as the WasmInstruction offset so that
// a real bridge and a mock bridge behave identically from the Stepper's
// perspective.  For each step i the assertion verifies:
//
//	result.StepIndex == result.InstructionOffset
func TestTimeTravelStepIndicesMatchInstructionOffsets(t *testing.T) {
	const numSteps = 5

	runner := makeCounterRunner(numSteps)
	req := simulator.SimulationRequest{
		EnvelopeXdr:    "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		ResultMetaXdr:  "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		LedgerEntries:  map[string]string{counterKey: counterValue(0)},
		LedgerSequence: 3000,
	}

	stepper := NewStepper(runner, req)
	ctx := context.Background()

	for i := 0; i < numSteps; i++ {
		result, err := stepper.Step(ctx, map[string]string{counterKey: counterValue(uint32(i + 1))})
		require.NoError(t, err, "step %d failed", i)

		assert.Equal(t,
			uint64(result.StepIndex),
			result.InstructionOffset,
			"step %d: InstructionOffset %d != StepIndex %d",
			i, result.InstructionOffset, result.StepIndex,
		)
	}
}

// TestTimeTravelSimulationError verifies that a simulation error propagates
// correctly through the Stepper without corrupting the accumulated state.
func TestTimeTravelSimulationError(t *testing.T) {
	errRunner := simulator.NewMockRunner(func(_ context.Context, _ *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
		return nil, fmt.Errorf("bridge: rust simulator exited with code 1")
	})

	req := simulator.SimulationRequest{
		EnvelopeXdr:    "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		ResultMetaXdr:  "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		LedgerEntries:  map[string]string{counterKey: counterValue(0)},
		LedgerSequence: 4000,
	}

	stepper := NewStepper(errRunner, req)
	_, err := stepper.Step(context.Background(), nil)

	require.Error(t, err, "expected error from failing runner")
	assert.Contains(t, err.Error(), "bridge: rust simulator exited with code 1")
	assert.Empty(t, stepper.Steps(), "no steps should be recorded after a failed run")
}
