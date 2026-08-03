// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/dotandev/hintents/internal/logger"
)

// BatchConfig holds configuration for parallel batch simulation.
type BatchConfig struct {
	InputDir    string        // directory containing transaction files
	OutputDir   string        // directory for simulation results
	Concurrency int           // number of parallel simulator instances (default: runtime.NumCPU())
	FilePattern string        // glob pattern for transaction files (default: "*.json")
	Timeout     time.Duration // per-simulation timeout (default: 30s)
	FailFast    bool          // stop all on first failure
}

// BatchResult represents the result of simulating a single transaction file.
type BatchResult struct {
	FilePath string        // relative path to the transaction file
	Success  bool          // whether the simulation succeeded
	Output   []byte        // stdout+stderr from the simulator
	Error    error         // error if the simulation failed
	Duration time.Duration // time taken for this simulation
}

// RunBatch executes parallel simulations of transaction files in a directory.
// It walks InputDir, collects files matching FilePattern, spawns up to cfg.Concurrency
// goroutines, and collects results. Context cancellation stops all in-flight simulations.
// If FailFast is true, cancellation stops remaining work on first failure.
// Returns all BatchResult entries (success and failure) when done.
func RunBatch(ctx context.Context, cfg BatchConfig) ([]BatchResult, error) {
	// Validate required fields
	if cfg.InputDir == "" {
		return nil, fmt.Errorf("batch simulate: InputDir is required")
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "./batch-output"
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = runtime.NumCPU()
	}
	if cfg.FilePattern == "" {
		cfg.FilePattern = "*.json"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	// Check InputDir exists
	info, err := os.Stat(cfg.InputDir)
	if err != nil {
		return nil, fmt.Errorf("batch simulate: InputDir does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("batch simulate: InputDir is not a directory: %s", cfg.InputDir)
	}

	// Create OutputDir if it doesn't exist
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("batch simulate: failed to create OutputDir: %w", err)
	}

	// Walk InputDir and collect files matching FilePattern
	var files []string
	err = filepath.Walk(cfg.InputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Check if file matches pattern
		base := filepath.Base(path)
		match, err := filepath.Match(cfg.FilePattern, base)
		if err != nil {
			return err
		}
		if match {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("batch simulate: failed to walk InputDir: %w", err)
	}

	if len(files) == 0 {
		return []BatchResult{}, nil
	}

	// Create context with cancellation for FailFast
	batchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up worker pool
	workQueue := make(chan *workItem, len(files))
	results := make(chan *BatchResult, len(files))

	// Launch workers
	var wg sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go worker(batchCtx, &wg, workQueue, results, cfg)
	}

	// Feed work queue
	for _, file := range files {
		relPath, err := filepath.Rel(cfg.InputDir, file)
		if err != nil {
			relPath = file
		}
		workQueue <- &workItem{
			inputPath: file,
			relPath:   relPath,
		}
	}
	close(workQueue)

	// Collect results
	var batchResults []BatchResult
	failureSeen := false
	resultsReceived := 0
	ctxCancelled := false

	for resultsReceived < len(files) && !ctxCancelled {
		select {
		case <-ctx.Done():
			ctxCancelled = true
			// Don't break yet - let workers know context is done, they'll stop
		case result := <-results:
			logger.Logger.Info(
				fmt.Sprintf("Processing file %d of %d: %s", resultsReceived+1, len(files), result.FilePath),
			)

			batchResults = append(batchResults, *result)
			resultsReceived++

			if !result.Success {
				failureSeen = true
				if cfg.FailFast {
					cancel()
					ctxCancelled = true
				}
			}
		}
	}

	// Wait for all workers to finish (with timeout to prevent hanging)
	// Use a separate goroutine with timeout to ensure we don't deadlock
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Workers finished normally
	case <-time.After(5 * time.Second):
		// Timeout - workers might be stuck
		// This shouldn't happen in normal operation
	}

	// Drain any remaining results to avoid goroutine leaks
	for {
		select {
		case result := <-results:
			batchResults = append(batchResults, *result)
			resultsReceived++
		default:
			// No more results
			goto done_draining
		}
	}

done_draining:
	// Close results channel
	close(results)

	if failureSeen && cfg.FailFast {
		// Note: We still return all collected results, even if we stopped early
		return batchResults, fmt.Errorf("batch simulate: stopped after first failure")
	}

	return batchResults, nil
}

type workItem struct {
	inputPath string
	relPath   string
}

// worker is a goroutine that processes items from the work queue.
func worker(ctx context.Context, wg *sync.WaitGroup, workQueue <-chan *workItem, results chan<- *BatchResult, cfg BatchConfig) {
	defer wg.Done()

	for {
		// Use non-blocking select to check context first, then try to get work
		select {
		case <-ctx.Done():
			// Context cancelled, exit immediately
			return
		default:
			// Not cancelled yet, try to get work (with timeout)
		}

		// Try to get work from queue
		select {
		case <-ctx.Done():
			// Context cancelled while waiting for work
			return
		case item, ok := <-workQueue:
			if !ok {
				// Work queue closed, exit gracefully
				return
			}

			start := time.Now()
			result := &BatchResult{FilePath: item.relPath}

			// Determine output file path
			outputFile := filepath.Join(cfg.OutputDir, item.relPath+".out")
			outputDir := filepath.Dir(outputFile)
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				result.Success = false
				result.Error = fmt.Errorf("failed to create output directory: %w", err)
				result.Duration = time.Since(start)
				results <- result
				continue
			}

			// Create context with timeout for this simulation
			simCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)

			// Invoke the simulator
			output, err := invokeSimulator(simCtx, item.inputPath, outputFile)
			result.Output = output
			result.Duration = time.Since(start)

			cancel()

			if err != nil {
				result.Success = false
				result.Error = err
			} else {
				result.Success = true
			}

			results <- result
		}
	}
}

// invokeSimulator finds the simulator binary and executes it for a single transaction file.
// The simulator is invoked as: simulator <inputFile> <outputFile>
// It returns combined stdout+stderr and any error.
func invokeSimulator(ctx context.Context, inputFile, outputFile string) ([]byte, error) {
	// Find simulator binary using the same discovery logic as the main simulator
	simBin, err := findSimulatorBinary()
	if err != nil {
		return nil, fmt.Errorf("batch simulate: %w", err)
	}

	// Create command with context for cancellation
	cmd := exec.CommandContext(ctx, simBin, inputFile, outputFile)

	// Capture output
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err = cmd.Run()

	// Combine stdout and stderr
	output := append(stdout.Bytes(), stderr.Bytes()...)

	if err != nil {
		// Context timeout becomes a specific error
		if ctx.Err() == context.DeadlineExceeded {
			return output, fmt.Errorf("simulation timeout exceeded for %s: %w", inputFile, context.DeadlineExceeded)
		}
		return output, fmt.Errorf("simulation failed for %s: %w", inputFile, err)
	}

	return output, nil
}

// findSimulatorBinary locates the simulator binary using the same discovery order as runner.go.
// Priority:
// 1. ERST_SIM_PATH environment variable
// 2. Local directory (./erst-sim, ./bin/erst-sim)
// 3. Dev targets (simulator/target/debug/erst-sim, simulator/target/release/erst-sim)
// 4. Global PATH
func findSimulatorBinary() (string, error) {
	// 1. Environment variable
	if env := os.Getenv("ERST_SIM_PATH"); env != "" {
		if isExecutable(env) {
			return env, nil
		}
	}

	// 2. Local directory
	cwd, err := os.Getwd()
	if err == nil {
		localCandidates := []string{
			filepath.Join(cwd, "erst-sim"),
			filepath.Join(cwd, "bin", "erst-sim"),
		}
		for _, p := range localCandidates {
			if isExecutable(p) {
				return p, nil
			}
		}
	}

	// 3. Dev targets
	devCandidates := []string{
		filepath.Join("simulator", "target", "debug", "erst-sim"),
		filepath.Join("simulator", "target", "release", "erst-sim"),
	}
	for _, p := range devCandidates {
		if isExecutable(p) {
			return p, nil
		}
	}

	// 4. Global PATH
	if p, err := exec.LookPath("erst-sim"); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("simulator binary not found (use ERST_SIM_PATH env var or ensure erst-sim is in PATH)")
}

// isExecutable checks if a file exists and is executable.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true // On Windows, if it's a file and we can stat it, assume it's executable
	}
	return info.Mode()&0111 != 0
}
