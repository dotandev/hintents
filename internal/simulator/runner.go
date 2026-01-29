// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/telemetry"
	"github.com/dotandev/hintents/internal/trace"
	"go.opentelemetry.io/otel/attribute"
)

// Runner handles the execution of the Rust simulator binary
type Runner struct {
	BinaryPath string
}

// Compile-time check to ensure Runner implements RunnerInterface
var _ RunnerInterface = (*Runner)(nil)

// NewRunner creates a new simulator runner.
// It checks for the binary in common locations.
func NewRunner() (*Runner, error) {
	// 1. Check environment variable
	if envPath := os.Getenv("ERST_SIMULATOR_PATH"); envPath != "" {
		return &Runner{BinaryPath: envPath}, nil
	}

	// 2. Check current directory (for Docker/Production)
	cwd, err := os.Getwd()
	if err == nil {
		localPath := filepath.Join(cwd, "erst-sim")
		if _, err := os.Stat(localPath); err == nil {
			return &Runner{BinaryPath: localPath}, nil
		}
	}

	// 3. Check development path (assuming running from sdk root)
	devPath := filepath.Join("simulator", "target", "release", "erst-sim")
	if _, err := os.Stat(devPath); err == nil {
		return &Runner{BinaryPath: devPath}, nil
	}

	// 4. Check global PATH
	if path, err := exec.LookPath("erst-sim"); err == nil {
		return &Runner{BinaryPath: path}, nil
	}

	return nil, errors.WrapSimulatorNotFound("simulator binary 'erst-sim' not found: Please build it or set ERST_SIMULATOR_PATH")
}

// Run executes the simulation with the given request
func (r *Runner) Run(req *SimulationRequest) (*SimulationResponse, error) {
	ctx := context.Background()
	tracer := telemetry.GetTracer()
	ctx, span := tracer.Start(ctx, "simulate_transaction")
	span.SetAttributes(attribute.String("simulator.binary_path", r.BinaryPath))
	defer span.End()

	logger.Logger.Debug("Starting simulation", "binary", r.BinaryPath)

	// Serialize Request
	_, marshalSpan := tracer.Start(ctx, "marshal_request")
	inputBytes, err := json.Marshal(req)
	marshalSpan.End()
	if err != nil {
		span.RecordError(err)
		logger.Logger.Error("Failed to marshal simulation request", "error", err)
		return nil, errors.WrapMarshalFailed(err)
	}

	span.SetAttributes(attribute.Int("request.size_bytes", len(inputBytes)))
	logger.Logger.Debug("Simulation request marshaled", "input_size", len(inputBytes))

	// Prepare Command
	cmd := exec.Command(r.BinaryPath)
	cmd.Stdin = bytes.NewReader(inputBytes)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	_, execSpan := tracer.Start(ctx, "execute_simulator")
	logger.Logger.Info("Executing simulator binary")
	if err := cmd.Run(); err != nil {
		execSpan.RecordError(err)
		execSpan.End()
		span.RecordError(err)
		logger.Logger.Error("Simulator execution failed", "error", err, "stderr", stderr.String())
		return nil, errors.WrapSimulationFailed(err, stderr.String())
	}
	execSpan.End()

	span.SetAttributes(
		attribute.Int("response.stdout_size", stdout.Len()),
		attribute.Int("response.stderr_size", stderr.Len()),
	)
	logger.Logger.Debug("Simulator execution completed", "stdout_size", stdout.Len(), "stderr_size", stderr.Len())

	// Deserialize Response
	_, unmarshalSpan := tracer.Start(ctx, "unmarshal_response")
	var resp SimulationResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		unmarshalSpan.RecordError(err)
		unmarshalSpan.End()
		span.RecordError(err)
		logger.Logger.Error("Failed to unmarshal simulation response", "error", err, "output", stdout.String())
		return nil, errors.WrapUnmarshalFailed(err, stdout.String())
	}
	unmarshalSpan.End()

	span.SetAttributes(attribute.String("simulation.status", resp.Status))
	logger.Logger.Info("Simulation response received", "status", resp.Status)

	// Check logic error from simulator
	if resp.Status == "error" {
		span.SetAttributes(attribute.String("simulation.error", resp.Error))
		logger.Logger.Error("Simulation logic error", "error", resp.Error)
		return nil, errors.WrapSimulationLogicError(resp.Error)
	}

	logger.Logger.Info("Simulation completed successfully")

	return &resp, nil
}

// RunWithTrace executes the simulator and generates an execution trace
func (r *Runner) RunWithTrace(ctx context.Context, req *SimulationRequest, txHash string) (*SimulationResponse, *trace.ExecutionTrace, error) {
	// Create execution trace
	executionTrace := trace.NewExecutionTrace(txHash, 5) // Snapshot every 5 steps

	// Add initial state
	executionTrace.AddState(trace.ExecutionState{
		Operation:  "simulation_start",
		ContractID: "simulator",
		HostState: map[string]interface{}{
			"envelope_size":      len(req.EnvelopeXdr),
			"has_ledger_entries": req.LedgerEntries != nil,
		},
	})

	// Run the simulation
	resp, err := r.Run(req)
	if err != nil {
		// Add error state
		executionTrace.AddState(trace.ExecutionState{
			Operation: "simulation_error",
			Error:     err.Error(),
		})
		return resp, executionTrace, err
	}

	// Parse events and logs to create trace states
	r.parseSimulationOutput(resp, executionTrace)

	// Add final state
	executionTrace.AddState(trace.ExecutionState{
		Operation: "simulation_complete",
		HostState: map[string]interface{}{
			"status":       resp.Status,
			"events_count": len(resp.Events),
			"logs_count":   len(resp.Logs),
		},
	})

	executionTrace.EndTime = executionTrace.States[len(executionTrace.States)-1].Timestamp

	return resp, executionTrace, nil
}

// parseSimulationOutput parses the simulation response and creates trace states
func (r *Runner) parseSimulationOutput(resp *SimulationResponse, executionTrace *trace.ExecutionTrace) {
	// Parse events to create execution states
	for i, event := range resp.Events {
		state := trace.ExecutionState{
			Operation: "diagnostic_event",
			HostState: map[string]interface{}{
				"event_index": i,
				"event_data":  event,
			},
		}

		// Try to extract contract and function info from event
		// This is a simplified parser - in reality, you'd parse XDR events
		if len(event) > 0 {
			state.ContractID = "contract_from_event" // Would extract from XDR
			state.Function = "function_from_event"   // Would extract from XDR
		}

		executionTrace.AddState(state)
	}

	// Parse logs to create additional states
	for i, logEntry := range resp.Logs {
		state := trace.ExecutionState{
			Operation: "host_log",
			HostState: map[string]interface{}{
				"log_index": i,
				"log_entry": logEntry,
			},
		}
		executionTrace.AddState(state)
	}
}
