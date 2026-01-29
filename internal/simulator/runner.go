package simulator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dotandev/hintents/internal/logger"
)

// Runner handles the execution of the Rust simulator binary
type Runner struct {
	BinaryPath string
	Debug      bool
}

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
	}, nil
}

// -------------------- Binary Discovery --------------------

func findSimBinary(simPathOverride string) (string, string, error) {
	// 1. Flag override
	if simPathOverride != "" {
		if isExecutable(simPathOverride) {
			return abs(simPathOverride), "flag --sim-path", nil
		}
		return "", "", fmt.Errorf("sim-path provided but not executable: %s", simPathOverride)
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

	return "", "", fmt.Errorf(
		"erst-sim binary not found (use --sim-path or set ERST_SIM_PATH)",
	)
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir() && info.Mode()&0111 != 0
}

func abs(path string) string {
	if p, err := filepath.Abs(path); err == nil {
		return p
	}
	return path
}

// -------------------- Execution --------------------

// Run executes the simulation with the given request
func (r *Runner) Run(req *SimulationRequest) (*SimulationResponse, error) {
	logger.Logger.Debug("Starting simulation", "binary", r.BinaryPath)

	// Serialize Request
	inputBytes, err := json.Marshal(req)
	if err != nil {
		logger.Logger.Error("Failed to marshal simulation request", "error", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Prepare Command
	cmd := exec.Command(r.BinaryPath)
	cmd.Stdin = bytes.NewReader(inputBytes)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Logger.Info("Executing simulator binary")

	if err := cmd.Run(); err != nil {
		logger.Logger.Error(
			"Simulator execution failed",
			"error", err,
			"stderr", stderr.String(),
		)
		return nil, fmt.Errorf("simulator execution failed: %w, stderr: %s", err, stderr.String())
	}

	// Deserialize Response
	var resp SimulationResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		logger.Logger.Error(
			"Failed to unmarshal simulation response",
			"error", err,
			"output", stdout.String(),
		)
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.Status == "error" {
		logger.Logger.Error("Simulation logic error", "error", resp.Error)
		return nil, fmt.Errorf("simulation error: %s", resp.Error)
	}

	logger.Logger.Info("Simulation completed successfully")

	return &resp, nil
}
