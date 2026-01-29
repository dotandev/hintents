// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

	cmd := exec.Command(r.BinaryPath)
	cmd.Stdin = bytes.NewReader(inputBytes)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Logger.Info("Executing simulator binary")
	if err := cmd.Run(); err != nil {
		logger.Logger.Error("Simulator execution failed", "error", err, "stderr", stderr.String())
		return nil, errors.WrapSimulationFailed(err, stderr.String())
	}

	logger.Logger.Debug("Simulator execution completed", "stdout_size", stdout.Len(), "stderr_size", stderr.Len())

	var resp SimulationResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		logger.Logger.Error("Failed to unmarshal simulation response", "error", err, "output", stdout.String())
		return nil, errors.WrapUnmarshalFailed(err, stdout.String())
	}

	logger.Logger.Info("Simulation response received", "status", resp.Status)

	if resp.Status == "success" {
		violations := analyzeSecurityBoundary(resp.Events)
		resp.SecurityViolations = violations

		if len(violations) > 0 {
			logger.Logger.Warn("Security violations detected", "count", len(violations))
			for _, v := range violations {
				logger.Logger.Warn("Violation",
					"type", v.Type,
					"severity", v.Severity,
					"contract", v.Contract)
			}
		} else {
			logger.Logger.Info("No security violations detected")
		}
	}

	// Check logic error from simulator
	if resp.Status == "error" {
		logger.Logger.Error("Simulation logic error", "error", resp.Error)
		return nil, errors.WrapSimulationLogicError(resp.Error)
	}

	logger.Logger.Info("Simulation completed successfully")

	return &resp, nil
}

type event struct {
	Type      string `json:"type"`
	Contract  string `json:"contract,omitempty"`
	Address   string `json:"address,omitempty"`
	EventType string `json:"event_type,omitempty"`
}

type contractState struct {
	hasAuth     bool
	authChecked map[string]bool
}

func analyzeSecurityBoundary(events []string) []SecurityViolation {
	var violations []SecurityViolation
	contractStates := make(map[string]*contractState)

	for _, eventStr := range events {
		var e event
		if err := json.Unmarshal([]byte(eventStr), &e); err != nil {
			continue
		}

		if e.Contract == "" || e.Contract == "unknown" {
			continue
		}

		if _, exists := contractStates[e.Contract]; !exists {
			contractStates[e.Contract] = &contractState{
				authChecked: make(map[string]bool),
			}
		}

		state := contractStates[e.Contract]

		switch e.Type {
		case "auth":
			state.authChecked[e.Address] = true
			state.hasAuth = true

		case "storage_write":
			if !state.hasAuth {
				if !isSACPattern(e.Contract) {
					violations = append(violations, SecurityViolation{
						Type:        "unauthorized_state_modification",
						Severity:    "high",
						Description: "Storage write operation without prior require_auth check",
						Contract:    e.Contract,
						Details: map[string]interface{}{
							"operation": "storage_write",
						},
					})
				}
			}
		}
	}

	return violations
}

func isSACPattern(contract string) bool {
	if contract == "" || contract == "unknown" {
		return false
	}

	sacPatterns := []string{
		"stellar_asset",
		"SAC",
		"token",
	}

	contractLower := strings.ToLower(contract)
	for _, pattern := range sacPatterns {
		if strings.Contains(contractLower, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}
