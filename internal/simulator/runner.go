// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/ipc"
	"github.com/dotandev/hintents/internal/logger"
)

// Runner handles the execution of the Rust simulator binary
type Runner struct {
	BinaryPath string
	Debug      bool

	mu         sync.Mutex
	activeCmds map[*exec.Cmd]struct{}
	closed     bool
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
		activeCmds: make(map[*exec.Cmd]struct{}),
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

// getSimulatorMemoryLimit returns the effective simulator memory ceiling in bytes:
// request field if set, otherwise env ERST_SIM_MEMORY_LIMIT_BYTES.
func getSimulatorMemoryLimit(req *SimulationRequest) *uint64 {
	if req != nil && req.MemoryLimit != nil {
		return req.MemoryLimit
	}
	if v := os.Getenv("ERST_SIM_MEMORY_LIMIT_BYTES"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			return &n
		}
	}
	return nil
}

func getSimulatorCoverageLCOVPath(req *SimulationRequest) *string {
	if req != nil && req.CoverageLCOVPath != nil {
		return req.CoverageLCOVPath
	}
	if v := os.Getenv("ERST_SIM_COVERAGE_LCOV_PATH"); v != "" {
		return &v
	}
	return nil
}

// -------------------- Execution --------------------

func (r *Runner) Run(ctx context.Context, req *SimulationRequest) (*SimulationResponse, error) {
	if req == nil {
		return nil, errors.NewSimErrorMsg(errors.CodeValidationFailed, "simulation request cannot be nil")
	}

	if req.MemoryLimit == nil {
		req.MemoryLimit = getSimulatorMemoryLimit(req)
	}
	if req.CoverageLCOVPath == nil {
		req.CoverageLCOVPath = getSimulatorCoverageLCOVPath(req)
	}
	if req.CoverageLCOVPath != nil {
		req.EnableCoverage = true
	}
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

	inputBytes, err := json.Marshal(req)
	if err != nil {
		logger.Logger.Error("Failed to marshal simulation request", "error", err)
		return nil, errors.WrapMarshalFailed(err)
	}

	cmd := exec.Command(r.BinaryPath)
	prepareCommand(cmd)
	cmd.Stdin = bytes.NewReader(inputBytes)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, errors.WrapSimCrash(err, "failed to start simulator")
	}

	if err := r.trackCommand(cmd); err != nil {
		_ = terminateCommand(cmd, 100*time.Millisecond)
		_ = cmd.Wait()
		return nil, err
	}
	defer r.untrackCommand(cmd)

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			logger.Logger.Error("Simulator execution failed", "error", err, "stderr", stderr.String())
			return nil, errors.WrapSimCrash(err, stderr.String())
		}
	case <-ctx.Done():
		_ = r.terminateProcessGroup(cmd, 1500*time.Millisecond)
		<-waitCh
		return nil, ctx.Err()
	}

	var resp SimulationResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		logger.Logger.Error("Failed to unmarshal response", "error", err)
		return nil, errors.WrapUnmarshalFailed(err, stdout.String())
	}

	// If the simulator returned a logical error inside the response payload,
	// classify it into a unified ErstError before returning to the caller.
	if resp.Error != "" {
		classified := (&ipc.Error{Code: resp.ErrorCode, Message: resp.Error}).ToErstError()
		logger.Logger.Error("Simulator returned error",
			"code", classified.Code,
			"original", classified.OriginalError,
		)
		return nil, classified
	}

	resp.ProtocolVersion = &proto.Version

	return &resp, nil
}

func (r *Runner) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true

	cmds := make([]*exec.Cmd, 0, len(r.activeCmds))
	for cmd := range r.activeCmds {
		cmds = append(cmds, cmd)
	}
	r.mu.Unlock()

	var firstErr error
	for _, cmd := range cmds {
		if err := r.terminateProcessGroup(cmd, 1500*time.Millisecond); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func (r *Runner) trackCommand(cmd *exec.Cmd) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return fmt.Errorf("runner is closed")
	}
	r.activeCmds[cmd] = struct{}{}
	return nil
}

func (r *Runner) untrackCommand(cmd *exec.Cmd) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.activeCmds, cmd)
}

func (r *Runner) terminateProcessGroup(cmd *exec.Cmd, graceTimeout time.Duration) error {
	if cmd == nil {
		return nil
	}
	err := terminateCommand(cmd, graceTimeout)
	if err != nil {
		logger.Logger.Error("Failed to terminate simulator process", "error", err)
		return err
	}
	return nil
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
