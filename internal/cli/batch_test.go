// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRunBatch_ProcessesAllFiles(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}

	// Create test files
	numFiles := 5
	for i := 0; i < numFiles; i++ {
		testFile := filepath.Join(inputDir, fmt.Sprintf("tx%d.json", i))
		if err := os.WriteFile(testFile, []byte(`{"test": true}`), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	// Mock simulator that succeeds
	mockBinary := createMockSimulator(t, tmpDir, "success")
	t.Setenv("ERST_SIM_PATH", mockBinary)

	cfg := BatchConfig{
		InputDir:    inputDir,
		OutputDir:   outputDir,
		Concurrency: 2,
		FilePattern: "*.json",
		Timeout:     5 * time.Second,
	}

	results, err := RunBatch(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunBatch failed: %v", err)
	}

	if len(results) != numFiles {
		t.Errorf("expected %d results, got %d", numFiles, len(results))
	}

	for _, result := range results {
		if !result.Success {
			t.Errorf("expected success for %s, got error: %v", result.FilePath, result.Error)
		}
	}
}

func TestRunBatch_ErrorsWhenInputDirNotExists(t *testing.T) {
	cfg := BatchConfig{
		InputDir:    "/nonexistent/path",
		OutputDir:   t.TempDir(),
		Concurrency: 1,
	}

	_, err := RunBatch(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for non-existent InputDir")
	}

	if !contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' in error, got: %v", err)
	}
}

func TestRunBatch_ReturnsEmptyResultsForNoMatchingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")

	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}

	// Create files that don't match pattern
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(inputDir, fmt.Sprintf("tx%d.txt", i))
		if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	cfg := BatchConfig{
		InputDir:    inputDir,
		OutputDir:   t.TempDir(),
		Concurrency: 1,
		FilePattern: "*.json", // won't match *.txt files
	}

	results, err := RunBatch(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunBatch failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for no matching files, got %d", len(results))
	}
}

func TestRunBatch_CollectsSuccessResults(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}

	// Create test files
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(inputDir, fmt.Sprintf("tx%d.json", i))
		if err := os.WriteFile(testFile, []byte(`{"test": true}`), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	mockBinary := createMockSimulator(t, tmpDir, "success")
	t.Setenv("ERST_SIM_PATH", mockBinary)

	cfg := BatchConfig{
		InputDir:    inputDir,
		OutputDir:   outputDir,
		Concurrency: 1,
		FilePattern: "*.json",
		Timeout:     5 * time.Second,
		FailFast:    false,
	}

	results, err := RunBatch(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunBatch failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	for _, result := range results {
		if !result.Success {
			t.Errorf("expected success for %s, got error: %v", result.FilePath, result.Error)
		}
	}
}

func TestRunBatch_CollectsFailureResults(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}

	// Create test files
	for i := 0; i < 2; i++ {
		testFile := filepath.Join(inputDir, fmt.Sprintf("tx%d.json", i))
		if err := os.WriteFile(testFile, []byte(`{"test": true}`), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	// Mock simulator that always fails
	mockBinary := createMockSimulatorAlwaysFail(t, tmpDir)
	t.Setenv("ERST_SIM_PATH", mockBinary)

	cfg := BatchConfig{
		InputDir:    inputDir,
		OutputDir:   outputDir,
		Concurrency: 1,
		FilePattern: "*.json",
		Timeout:     5 * time.Second,
		FailFast:    false,
	}

	results, _ := RunBatch(context.Background(), cfg)

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	for _, result := range results {
		if result.Success {
			t.Errorf("expected failure for %s", result.FilePath)
		}

		if result.Error == nil {
			t.Errorf("expected error for %s", result.FilePath)
		}
	}
}

func TestRunBatch_FailFastStopsOnFirstFailure(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}

	// Create test files
	numFiles := 3
	for i := 0; i < numFiles; i++ {
		testFile := filepath.Join(inputDir, fmt.Sprintf("tx%d.json", i))
		if err := os.WriteFile(testFile, []byte(`{"test": true}`), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	// Mock simulator that fails immediately
	mockBinary := createMockSimulatorAlwaysFail(t, tmpDir)
	t.Setenv("ERST_SIM_PATH", mockBinary)

	cfg := BatchConfig{
		InputDir:    inputDir,
		OutputDir:   outputDir,
		Concurrency: 1,
		FilePattern: "*.json",
		Timeout:     5 * time.Second,
		FailFast:    true,
	}

	results, err := RunBatch(context.Background(), cfg)

	// Should have an error due to FailFast
	if err == nil {
		t.Fatal("expected error when FailFast is triggered")
	}

	if !contains(err.Error(), "stopped after first failure") {
		t.Errorf("expected 'stopped after first failure' in error, got: %v", err)
	}

	// Should have at least one result
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
}

func TestRunBatch_PerSimulationTimeoutKillsSlow(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}

	// Create one test file
	if err := os.WriteFile(filepath.Join(inputDir, "tx.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock simulator that sleeps longer than timeout
	mockBinary := createMockSimulatorSlow(t, tmpDir, 5*time.Second)
	t.Setenv("ERST_SIM_PATH", mockBinary)

	cfg := BatchConfig{
		InputDir:    inputDir,
		OutputDir:   outputDir,
		Concurrency: 1,
		FilePattern: "*.json",
		Timeout:     100 * time.Millisecond,
	}

	results, _ := RunBatch(context.Background(), cfg)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Success {
		t.Error("expected failure due to timeout")
	}

	if result.Error == nil {
		t.Error("expected error to be set")
	} else if !contains(result.Error.Error(), "timeout") {
		t.Errorf("expected timeout error, got: %v", result.Error)
	}
}

func TestRunBatch_ResultsIncludeDuration(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}

	// Create test files
	for i := 0; i < 2; i++ {
		testFile := filepath.Join(inputDir, fmt.Sprintf("tx%d.json", i))
		if err := os.WriteFile(testFile, []byte(`{"test": true}`), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	mockBinary := createMockSimulator(t, tmpDir, "success")
	t.Setenv("ERST_SIM_PATH", mockBinary)

	cfg := BatchConfig{
		InputDir:    inputDir,
		OutputDir:   outputDir,
		Concurrency: 1,
		FilePattern: "*.json",
		Timeout:     5 * time.Second,
	}

	results, err := RunBatch(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunBatch failed: %v", err)
	}

	for _, result := range results {
		if result.Duration == 0 {
			t.Errorf("expected non-zero Duration for %s", result.FilePath)
		}

		if result.Duration < 0 {
			t.Errorf("expected positive Duration, got %v for %s", result.Duration, result.FilePath)
		}
	}
}

func TestRunBatch_FilePatternFilteringWorks(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}

	// Create mixed files
	if err := os.WriteFile(filepath.Join(inputDir, "tx1.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(inputDir, "tx2.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(inputDir, "tx3.txt"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(inputDir, "tx4.xml"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	mockBinary := createMockSimulator(t, tmpDir, "success")
	t.Setenv("ERST_SIM_PATH", mockBinary)

	cfg := BatchConfig{
		InputDir:    inputDir,
		OutputDir:   outputDir,
		Concurrency: 1,
		FilePattern: "*.json",
		Timeout:     5 * time.Second,
	}

	results, err := RunBatch(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunBatch failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results for *.json pattern, got %d", len(results))
	}

	for _, result := range results {
		if !contains(result.FilePath, ".json") {
			t.Errorf("expected .json file, got: %s", result.FilePath)
		}
	}
}

func TestRunBatch_CreatesOutputDirectoryIfNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	outputDir := filepath.Join(tmpDir, "nonexistent", "nested", "output")

	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}

	// Create test file
	if err := os.WriteFile(filepath.Join(inputDir, "tx.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	mockBinary := createMockSimulator(t, tmpDir, "success")
	t.Setenv("ERST_SIM_PATH", mockBinary)

	cfg := BatchConfig{
		InputDir:    inputDir,
		OutputDir:   outputDir,
		Concurrency: 1,
		FilePattern: "*.json",
		Timeout:     5 * time.Second,
	}

	results, err := RunBatch(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunBatch failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	// Check if output directory was created
	if _, err := os.Stat(outputDir); err != nil {
		t.Errorf("expected output directory to be created: %v", err)
	}
}

// Helper functions for mock simulators

func createMockSimulator(t *testing.T, tmpDir, mode string) string {
	mockPath := filepath.Join(tmpDir, "mock-sim")
	script := `#!/bin/bash
exit 0
`
	if runtime.GOOS == "windows" {
		mockPath += ".bat"
		script = "@echo off\nexit /b 0\n"
	}

	if err := os.WriteFile(mockPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create mock simulator: %v", err)
	}

	return mockPath
}

func createMockSimulatorAlwaysFail(t *testing.T, tmpDir string) string {
	mockPath := filepath.Join(tmpDir, "mock-sim-fail")
	script := `#!/bin/bash
exit 1
`
	if runtime.GOOS == "windows" {
		mockPath += ".bat"
		script = "@echo off\nexit /b 1\n"
	}

	if err := os.WriteFile(mockPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create mock simulator: %v", err)
	}

	return mockPath
}

func createMockSimulatorSlow(t *testing.T, tmpDir string, delay time.Duration) string {
	mockPath := filepath.Join(tmpDir, "mock-sim-slow")
	seconds := int(delay.Seconds())
	if seconds < 1 {
		seconds = 1
	}

	script := fmt.Sprintf(`#!/bin/bash
sleep %d
exit 0
`, seconds)

	if runtime.GOOS == "windows" {
		mockPath += ".bat"
		// On Windows, use a more reliable blocking mechanism that responds to process termination
		// The "timeout" command with /nobreak will block and can be killed by process termination
		// We add extra buffer to ensure it runs longer than the test timeout
		script = fmt.Sprintf("@echo off\nping -n %d 127.0.0.1 >nul 2>&1\nexit /b 0\n", seconds+2)
	}

	if err := os.WriteFile(mockPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create mock simulator: %v", err)
	}

	return mockPath
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
