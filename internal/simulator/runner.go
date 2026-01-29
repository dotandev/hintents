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
	"go.opentelemetry.io/otel/attribute"
)

// RunnerInterface defines the contract for simulator execution
type RunnerInterface interface {
	Run(req *SimulationRequest) (*SimulationResponse, error)
}

// Runner handles the execution of the Rust simulator binary
type Runner struct {
// ConcreteRunner handles the execution of the Rust simulator binary
type ConcreteRunner struct {
	BinaryPath string
}

// Compile-time check to ensure Runner implements RunnerInterface
var _ RunnerInterface = (*Runner)(nil)

// NewRunner creates a new simulator runner.
// It checks for the binary in common locations.
func NewRunner() (*ConcreteRunner, error) {
	// 1. Check environment variable
	if envPath := os.Getenv("ERST_SIMULATOR_PATH"); envPath != "" {
		return &ConcreteRunner{BinaryPath: envPath}, nil
	}

	// 2. Check current directory (for Docker/Production)
	cwd, err := os.Getwd()
	if err == nil {
		localPath := filepath.Join(cwd, "erst-sim")
		if _, err := os.Stat(localPath); err == nil {
			return &ConcreteRunner{BinaryPath: localPath}, nil
		}
	}

	// 3. Check development path (assuming running from sdk root)
	devPath := filepath.Join("simulator", "target", "release", "erst-sim")
	if _, err := os.Stat(devPath); err == nil {
		return &ConcreteRunner{BinaryPath: devPath}, nil
	}

	// 4. Check global PATH
	if path, err := exec.LookPath("erst-sim"); err == nil {
		return &ConcreteRunner{BinaryPath: path}, nil
	}

	return nil, errors.WrapSimulatorNotFound("Please build it or set ERST_SIMULATOR_PATH")
}

// Run executes the simulation with the given request
func (r *Runner) Run(ctx context.Context, req *SimulationRequest) (*SimulationResponse, error) {
	tracer := telemetry.GetTracer()
	ctx, span := tracer.Start(ctx, "simulate_transaction")
	span.SetAttributes(attribute.String("simulator.binary_path", r.BinaryPath))
	defer span.End()

func (r *ConcreteRunner) Run(req *SimulationRequest) (*SimulationResponse, error) {
	logger.Logger.Debug("Starting simulation", "binary", r.BinaryPath)

	// Serialize Request
	ctx, marshalSpan := tracer.Start(ctx, "marshal_request")
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
	ctx, execSpan := tracer.Start(ctx, "execute_simulator")
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
