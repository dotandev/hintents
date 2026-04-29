// Package simulator provides the core simulation harness for Soroban smart contracts.
package simulator

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Ledger key / value types (stubs — replace with real XDR types)
// ---------------------------------------------------------------------------

// LedgerKey is a canonical string representation of a Soroban ledger key.
// In a full implementation this wraps xdr.LedgerKey serialised to a
// deterministic string (e.g. base64 XDR or a human-readable form).
type LedgerKey = string

// LedgerValue holds the raw serialised bytes of a ledger entry value.
type LedgerValue = []byte

// ---------------------------------------------------------------------------
// Runner configuration
// ---------------------------------------------------------------------------

// RunnerConfig holds all options for a single simulation run.
type RunnerConfig struct {
	// TransactionEnvelope is the raw XDR of the transaction to simulate.
	TransactionEnvelope []byte

	// LedgerFootprint is the read/write set for the transaction.
	LedgerFootprint map[LedgerKey]LedgerValue

	// Network identifies the Stellar network ("testnet", "mainnet", …).
	Network string

	// Watchpoints is the manager that receives ledger mutation notifications.
	// May be nil, in which case watchpoint reporting is disabled.
	Watchpoints *WatchpointManager

	// ContractID is the hex-encoded contract being simulated.
	ContractID string
}

// ---------------------------------------------------------------------------
// Simulation result types
// ---------------------------------------------------------------------------

// SimulationResult carries the outcome of a single simulation run.
type SimulationResult struct {
	// Success indicates whether the transaction completed without trapping.
	Success bool

	// DiagnosticEvents contains all events emitted by soroban-env-host.
	DiagnosticEvents []DiagnosticEvent

	// WatchpointEvents contains all watchpoint events that fired during the run.
	// Populated only when RunnerConfig.Watchpoints is non-nil.
	WatchpointEvents []WatchpointEvent

	// ErrorMessage holds the human-readable failure reason, if any.
	ErrorMessage string
}

// DiagnosticEvent is a host-level diagnostic emitted during execution.
type DiagnosticEvent struct {
	// Message is the raw diagnostic string from the host.
	Message string
	// ContractID is the emitting contract's identifier.
	ContractID string
}

// ---------------------------------------------------------------------------
// Runner
// ---------------------------------------------------------------------------

// Runner orchestrates a single simulation run of a Soroban transaction.
//
// Lifecycle:
//  1. Create via NewRunner.
//  2. Optionally register watchpoints via config.Watchpoints.
//  3. Call Run to execute the simulation.
type Runner struct {
	cfg RunnerConfig

	// collectedWatchpointEvents accumulates events from the watchpoint handler
	// installed by Run so they can be returned in SimulationResult.
	collectedWatchpointEvents []WatchpointEvent
}

// NewRunner creates a Runner from the provided configuration.
// Returns an error if the configuration is invalid.
func NewRunner(cfg RunnerConfig) (*Runner, error) {
	if err := validateRunnerConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid runner config: %w", err)
	}
	return &Runner{cfg: cfg}, nil
}

// validateRunnerConfig performs basic sanity checks on RunnerConfig.
func validateRunnerConfig(cfg RunnerConfig) error {
	if len(cfg.TransactionEnvelope) == 0 {
		return fmt.Errorf("TransactionEnvelope must not be empty")
	}
	if cfg.Network == "" {
		return fmt.Errorf("Network must not be empty")
	}
	return nil
}

// Run executes the simulation and returns a SimulationResult.
//
// When cfg.Watchpoints is non-nil, Run registers a local handler on the
// manager that accumulates all fired events into the returned result. This
// handler is installed for the duration of this call only — it does not
// persist across runs.
//
// Callers are responsible for resetting or re-using the WatchpointManager
// between runs.
func (r *Runner) Run() (SimulationResult, error) {
	r.collectedWatchpointEvents = nil

	// Install a scoped watchpoint collector if the caller has provided a manager.
	if r.cfg.Watchpoints != nil {
		r.cfg.Watchpoints.OnFire(func(evt WatchpointEvent) {
			r.collectedWatchpointEvents = append(r.collectedWatchpointEvents, evt)
		})
	}

	// Execute the simulated transaction. In a real implementation this calls
	// into the Rust erst-sim binary via IPC/FFI and receives execution events.
	diagnostics, ledgerMutations, err := r.executeTransaction()
	if err != nil {
		return SimulationResult{}, fmt.Errorf("simulation execution failed: %w", err)
	}

	// Process each ledger mutation: fire watchpoints for writes and deletes.
	for _, mut := range ledgerMutations {
		r.applyMutation(mut)
	}

	return SimulationResult{
		Success:          true,
		DiagnosticEvents: diagnostics,
		WatchpointEvents: r.collectedWatchpointEvents,
	}, nil
}

// ---------------------------------------------------------------------------
// Internal simulation helpers
// ---------------------------------------------------------------------------

// ledgerMutation represents a single write or delete that occurred during
// WASM execution. Populated by executeTransaction from IPC output.
type ledgerMutation struct {
	Key         LedgerKey
	Kind        WatchpointEventKind
	NewValue    LedgerValue
	OldValue    LedgerValue
	CallStack   []WASMFrame
	HostFn      string
	Instruction string
}

// executeTransaction is the boundary between the Go runner and the Rust
// simulator binary. In the real implementation this marshals the config to
// JSON (or protobuf), invokes erst-sim via os/exec or a Unix socket, and
// unmarshals the structured response.
//
// For the purposes of this stub it returns a synthetic result that exercises
// the watchpoint path so integration tests can verify the pipeline end-to-end.
func (r *Runner) executeTransaction() ([]DiagnosticEvent, []ledgerMutation, error) {
	// Stub diagnostics — in production these come from diagnostic_events in the
	// soroban-env-host XDR result.
	diagnostics := []DiagnosticEvent{
		{
			Message:    fmt.Sprintf("simulating contract %s on %s", r.cfg.ContractID, r.cfg.Network),
			ContractID: r.cfg.ContractID,
		},
	}

	// Stub mutations — in production the Rust simulator emits these as part of
	// its structured trace output whenever a storage put/remove is executed.
	mutations := []ledgerMutation{
		{
			Key:  "ContractData/AAAAAA==/balance",
			Kind: WatchpointWrite,
			NewValue: []byte(`{"amount":1000}`),
			OldValue: []byte(`{"amount":0}`),
			CallStack: []WASMFrame{
				{
					Index:           0,
					FunctionName:    "token::write_balance",
					InstructionOffset: 0x3c,
					ModuleOffset:    0x1abc,
					SourceFile:      "src/token.rs",
					SourceLine:      142,
					SourceColumn:    5,
					GitHubURL:       "https://github.com/stellar/soroban-examples/blob/main/token/src/token.rs#L142",
				},
				{
					Index:           1,
					FunctionName:    "token::transfer",
					InstructionOffset: 0x10,
					ModuleOffset:    0x1a00,
					SourceFile:      "src/token.rs",
					SourceLine:      98,
					SourceColumn:    9,
					GitHubURL:       "https://github.com/stellar/soroban-examples/blob/main/token/src/token.rs#L98",
				},
				{
					Index:           2,
					FunctionName:    "invoke_contract_fn[0]",
					InstructionOffset: 0x04,
					ModuleOffset:    0x0800,
					SourceFile:      "",
					SourceLine:      0,
				},
			},
			HostFn:      "call",
			Instruction: "i64.store offset=0x8",
		},
		{
			Key:  "ContractData/AAAAAA==/allowance",
			Kind: WatchpointDelete,
			OldValue: []byte(`{"spender":"GBXYZ","amount":500}`),
			CallStack: []WASMFrame{
				{
					Index:           0,
					FunctionName:    "token::remove_allowance",
					InstructionOffset: 0x22,
					ModuleOffset:    0x2200,
					SourceFile:      "src/allowance.rs",
					SourceLine:      67,
					SourceColumn:    5,
					GitHubURL:       "https://github.com/stellar/soroban-examples/blob/main/token/src/allowance.rs#L67",
				},
				{
					Index:           1,
					FunctionName:    "token::transfer_from",
					InstructionOffset: 0x44,
					ModuleOffset:    0x1f00,
					SourceFile:      "src/token.rs",
					SourceLine:      115,
					SourceColumn:    9,
					GitHubURL:       "https://github.com/stellar/soroban-examples/blob/main/token/src/token.rs#L115",
				},
			},
			HostFn:      "call",
			Instruction: "call $storage_remove",
		},
	}

	return diagnostics, mutations, nil
}

// applyMutation applies a single ledger mutation to the local footprint and
// notifies the watchpoint manager if one is configured.
func (r *Runner) applyMutation(mut ledgerMutation) {
	// Update the in-memory footprint to reflect the mutation.
	switch mut.Kind {
	case WatchpointWrite:
		if r.cfg.LedgerFootprint != nil {
			r.cfg.LedgerFootprint[mut.Key] = mut.NewValue
		}
	case WatchpointDelete:
		if r.cfg.LedgerFootprint != nil {
			delete(r.cfg.LedgerFootprint, mut.Key)
		}
	}

	// Fire watchpoints (no-op when manager is nil or key is not watched).
	if r.cfg.Watchpoints == nil {
		return
	}

	evt := buildWatchpointEvent(
		mut.Key,
		mut.Kind,
		mut.NewValue,
		mut.OldValue,
		mut.CallStack,
		r.cfg.ContractID,
		mut.HostFn,
		mut.Instruction,
	)
	r.cfg.Watchpoints.Notify(evt)
}

// ---------------------------------------------------------------------------
// Pretty-print helpers (used by the CLI trace viewer)
// ---------------------------------------------------------------------------

// FormatWatchpointReport returns a human-readable report of all watchpoint
// events in a SimulationResult. Returns an empty string when no events fired.
func FormatWatchpointReport(result SimulationResult) string {
	if len(result.WatchpointEvents) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("=== Watchpoint Report ===\n")
	sb.WriteString(fmt.Sprintf("%d event(s) fired\n\n", len(result.WatchpointEvents)))

	for i, evt := range result.WatchpointEvents {
		sb.WriteString(fmt.Sprintf("--- Event %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("  Key:         %s\n", evt.WatchedKey))
		sb.WriteString(fmt.Sprintf("  Kind:        %s\n", evt.Kind))
		sb.WriteString(fmt.Sprintf("  Contract:    %s\n", evt.ContractID))
		sb.WriteString(fmt.Sprintf("  Host fn:     %s\n", evt.FunctionName))
		sb.WriteString(fmt.Sprintf("  Instruction: %s\n", evt.WASMInstruction))
		sb.WriteString(fmt.Sprintf("  Timestamp:   %s\n", evt.Timestamp.Format("2006-01-02T15:04:05.999Z")))

		if len(evt.PreviousValue) > 0 {
			sb.WriteString(fmt.Sprintf("  Old value:   %s\n", string(evt.PreviousValue)))
		}
		if len(evt.Value) > 0 {
			sb.WriteString(fmt.Sprintf("  New value:   %s\n", string(evt.Value)))
		}

		sb.WriteString("  Call stack:\n")
		sb.WriteString(evt.FormatCallStack())
		sb.WriteByte('\n')
	}

	return sb.String()
}