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
	"strconv"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/ipc"
	"github.com/dotandev/hintents/internal/logger"
)

// Runner handles the execution of the Rust simulator binary
type Runner struct {
	BinaryPath string
	Debug      bool
	MockTime   int64 // non-zero overrides Timestamp in every SimulationRequest
	Validator  *Validator
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

// getSandboxNativeTokenCap returns the effective sandbox native token cap (stroops):
// request field if set, otherwise env ERST_SANDBOX_NATIVE_TOKEN_CAP_STROOPS.
func getSandboxNativeTokenCap(req *SimulationRequest) *uint64 {
	if req != nil && req.SandboxNativeTokenCapStroops != nil {
		return req.SandboxNativeTokenCapStroops
	}
	if v := os.Getenv("ERST_SANDBOX_NATIVE_TOKEN_CAP_STROOPS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			return &n
		}
	}
	return nil
}

// -------------------- Execution --------------------

func (r *Runner) Run(req *SimulationRequest) (*SimulationResponse, error) {
	// Enforce sandbox native token cap when set (local/sandbox economic constraint)
	if capStroops := getSandboxNativeTokenCap(req); capStroops != nil {
		if err := EnforceSandboxNativeTokenCap(req.EnvelopeXdr, *capStroops); err != nil {
			logger.Logger.Error("Sandbox native token cap exceeded", "error", err)
			return nil, err
		}
	}

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

	cmd := exec.Command(r.BinaryPath)

	// Use pipes to stream data and avoid large buffer allocations
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, errors.WrapSimCrash(err, "failed to create stdin pipe")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.WrapSimCrash(err, "failed to create stdout pipe")
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, errors.WrapSimCrash(err, "failed to start simulator")
	}

	// Stream request to stdin in a separate goroutine to prevent deadlock
	encErrCh := make(chan error, 1)
	go func() {
		defer close(encErrCh)
		defer stdin.Close()
		if err := json.NewEncoder(stdin).Encode(req); err != nil {
			encErrCh <- err
		}
	}()

	var resp SimulationResponse
	// Stream response from stdout
	decErr := json.NewDecoder(stdout).Decode(&resp)

	// Wait for command to finish
	cmdErr := cmd.Wait()

	// Check encoding error first
	if err := <-encErrCh; err != nil {
		logger.Logger.Error("Failed to marshal simulation request", "error", err)
		return nil, errors.WrapMarshalFailed(err)
	}

	// Check command error (e.g. crash)
	if cmdErr != nil {
		logger.Logger.Error("Simulator execution failed", "error", cmdErr, "stderr", stderr.String())
		return nil, errors.WrapSimCrash(cmdErr, stderr.String())
	}

	// Check decoding error
	if decErr != nil {
		logger.Logger.Error("Failed to unmarshal response", "error", decErr)
		return nil, errors.WrapUnmarshalFailed(decErr, stderr.String())
	}

	// If the simulator returned a logical error inside the response payload,
	// classify it into a unified ErstError before returning to the caller.
	if resp.Error != "" {
		classified := (&ipc.Error{Message: resp.Error}).ToErstError()
		logger.Logger.Error("Simulator returned error",
			"code", classified.Code,
			"original", classified.OriginalError,
		)
		return nil, classified
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