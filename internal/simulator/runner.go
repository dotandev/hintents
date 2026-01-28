// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
)

// ConcreteRunner handles the execution of the Rust simulator binary
type ConcreteRunner struct {
	BinaryPath string
}

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
func (r *ConcreteRunner) Run(req *SimulationRequest) (*SimulationResponse, error) {
	logger.Logger.Debug("Starting simulation", "binary", r.BinaryPath)

	// Serialize Request
	inputBytes, err := json.Marshal(req)
	if err != nil {
		logger.Logger.Error("Failed to marshal simulation request", "error", err)
		return nil, errors.WrapMarshalFailed(err)
	}

	logger.Logger.Debug("Simulation request marshaled", "input_size", len(inputBytes))

	// Prepare Command
	cmd := exec.Command(r.BinaryPath)
	cmd.Stdin = bytes.NewReader(inputBytes)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	logger.Logger.Info("Executing simulator binary")
	if err := cmd.Run(); err != nil {
		logger.Logger.Error("Simulator execution failed", "error", err, "stderr", stderr.String())
		return nil, errors.WrapSimulationFailed(err, stderr.String())
	}

	logger.Logger.Debug("Simulator execution completed", "stdout_size", stdout.Len(), "stderr_size", stderr.Len())

	// Deserialize Response
	var resp SimulationResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		logger.Logger.Error("Failed to unmarshal simulation response", "error", err, "output", stdout.String())
		return nil, errors.WrapUnmarshalFailed(err, stdout.String())
	}

	logger.Logger.Info("Simulation response received", "status", resp.Status)

	// Check logic error from simulator
	if resp.Status == "error" {
		logger.Logger.Error("Simulation logic error", "error", resp.Error)
		return nil, errors.WrapSimulationLogicError(resp.Error)
	}

	logger.Logger.Info("Simulation completed successfully")

	return &resp, nil
}
