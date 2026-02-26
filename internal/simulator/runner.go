// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/ipc"
	"github.com/dotandev/hintents/internal/logger"
)

// Runner handles the execution of the Rust simulator binary
type Runner struct {
	BinaryPath      string
	Debug           bool
	MockTime        int64 // non-zero overrides Timestamp in every SimulationRequest
	EnableStreaming bool  // Enable real-time event streaming via Unix socket
}

// Compile-time check to ensure Runner implements RunnerInterface
var _ RunnerInterface = (*Runner)(nil)

// NewRunner creates a new simulator runner.
// Search order:
// 1. --sim-path override
// 2. ENV var
// 3. Local directory
// 4. Dev target
// 5. Global PATH
func NewRunner(simPathOverride string, debug bool) (*Runner, error) {
	path, source, err := findSimBinary(simPathOverride)
	if err != nil {
		return nil, err
	}

	if debug {
		logger.Logger.Debug(
			"Simulator binary resolved",
			"path", path,
			"source", source,
		)
	}

	return &Runner{
		BinaryPath: path,
		Debug:      debug,
		Validator:  NewValidator(false),
	}, nil
}

// NewRunnerWithMockTime creates a Runner that overrides the ledger timestamp on
// every request with the provided Unix epoch value. Pass 0 to disable the override.
func NewRunnerWithMockTime(simPathOverride string, debug bool, mockTime int64) (*Runner, error) {
	r, err := NewRunner(simPathOverride, debug)
	if err != nil {
		return nil, err
	}
	r.MockTime = mockTime
	return r, nil
}


// -------------------- Binary Discovery --------------------

func findSimBinary(simPathOverride string) (string, string, error) {
	// 1. Flag override
	if simPathOverride != "" {
		if isExecutable(simPathOverride) {
			return abs(simPathOverride), "flag --sim-path", nil
		}
		return "", "", errors.WrapSimulatorNotFound(simPathOverride)
	}

	// 2. Environment variable
	if env := os.Getenv("ERST_SIM_PATH"); env != "" {
		if isExecutable(env) {
			return abs(env), "env ERST_SIM_PATH", nil
		}
	}

	// 3. Local directory
	cwd, err := os.Getwd()
	if err == nil {
		localCandidates := []string{
			filepath.Join(cwd, "erst-sim"),
			filepath.Join(cwd, "bin", "erst-sim"),
		}

		for _, p := range localCandidates {
			if isExecutable(p) {
				return abs(p), "local directory", nil
			}
		}
	}

	// 4. Dev target
	devCandidates := []string{
		filepath.Join("simulator", "target", "debug", "erst-sim"),
		filepath.Join("simulator", "target", "release", "erst-sim"),
	}

	for _, p := range devCandidates {
		if isExecutable(p) {
			return abs(p), "dev target", nil
		}
	}

	// 5. Global PATH
	if p, err := exec.LookPath("erst-sim"); err == nil {
		return p, "global PATH", nil
	}

	return "", "", errors.WrapSimulatorNotFound("erst-sim binary not found (use --sim-path or set ERST_SIM_PATH)")
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true // On Windows, if it's a file and we can stat it, assume it's executable for now
	}
	return info.Mode()&0111 != 0
}

func abs(path string) string {
	if p, err := filepath.Abs(path); err == nil {
		return p
	}
	return path
}

// -------------------- Execution --------------------

func (r *Runner) Run(req *SimulationRequest) (*SimulationResponse, error) {
	// Validate request before processing
	if r.Validator != nil {
		if err := r.Validator.ValidateRequest(req); err != nil {
			logger.Logger.Error("Request validation failed", "error", err)
			return nil, err
		}
	}

	proto := GetOrDefault(req.ProtocolVersion)

	if req.ProtocolVersion != nil {
		if err := Validate(*req.ProtocolVersion); err != nil {
			return nil, err
		}
	}

	if err := r.applyProtocolConfig(req, proto); err != nil {
		return nil, err
	}

	if r.MockTime != 0 {
		req.Timestamp = r.MockTime
	}

	// Set up streaming if enabled
	var listener *SocketListener
	var handler *DefaultStreamHandler
	if r.EnableStreaming {
		handler = NewDefaultStreamHandler()
		var err error
		listener, err = NewSocketListener(handler)
		if err != nil {
			logger.Logger.Warn("Failed to create socket listener, falling back to non-streaming", "error", err)
		} else {
			listener.Start()
			defer listener.Close()
			
			socketPath := listener.GetSocketPath()
			req.SocketPath = &socketPath
			logger.Logger.Debug("Streaming enabled", "socket", socketPath)
		}
	}

	inputBytes, err := json.Marshal(req)
	if err != nil {
		logger.Logger.Error("Failed to marshal simulation request", "error", err)
		return nil, errors.WrapMarshalFailed(err)
	}

	cmd := exec.Command(r.BinaryPath)
	cmd.Stdin = bytes.NewReader(inputBytes)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logger.Logger.Error("Simulator execution failed", "error", err, "stderr", stderr.String())
		return nil, errors.WrapSimCrash(err, stderr.String())
	}

	var resp SimulationResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		logger.Logger.Error("Failed to unmarshal response", "error", err)
		return nil, errors.WrapUnmarshalFailed(err, stdout.String())
	}

	// If streaming was enabled, merge streamed data with final response
	if listener != nil && handler != nil {
		events, logs, cpu, mem, completed, streamErr := handler.GetResults()
		
		if len(events) > 0 {
			// Prefer streamed events over buffered ones
			resp.DiagnosticEvents = events
			logger.Logger.Debug("Using streamed events", "count", len(events))
		}
		
		if len(logs) > 0 {
			// Append streamed logs to response logs
			resp.Logs = append(resp.Logs, logs...)
		}
		
		// Update budget usage if we got streaming updates
		if cpu > 0 || mem > 0 {
			if resp.BudgetUsage == nil {
				resp.BudgetUsage = &BudgetUsage{}
			}
			resp.BudgetUsage.CPUInstructions = cpu
			resp.BudgetUsage.MemoryBytes = mem
		}
		
		if streamErr != "" && resp.Error == "" {
			resp.Error = streamErr
			resp.Status = "error"
		}
		
		if completed {
			logger.Logger.Debug("Simulation completed via streaming")
		}
	}

	resp.ProtocolVersion = &proto.Version

	return &resp, nil
}

func (r *Runner) applyProtocolConfig(req *SimulationRequest, proto *Protocol) error {
	if req.CustomAuthCfg == nil {
		req.CustomAuthCfg = make(map[string]interface{})
	}

	limits := make(map[string]interface{})
	for k, v := range proto.Features {
		switch k {
		case "max_contract_size", "max_contract_data_size", "max_instruction_limit":
			limits[k] = v
		}
	}

	if len(limits) > 0 {
		req.CustomAuthCfg["protocol_limits"] = limits
	}

	if opcodes, ok := proto.Features["supported_opcodes"].([]string); ok {
		req.CustomAuthCfg["supported_opcodes"] = opcodes
	}

	if calib, ok := proto.Features["resource_calibration"].(*ResourceCalibration); ok {
		req.ResourceCalibration = calib
	}

	return nil
}