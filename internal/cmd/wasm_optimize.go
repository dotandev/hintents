// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/wasmopt"
)

func optimizeWasmBytesIfRequested(input []byte, enabled bool) ([]byte, *wasmopt.Report, error) {
	if !enabled {
		return input, nil, nil
	}
	optimized, report, err := wasmopt.EliminateDeadCode(input)
	if err != nil {
		return nil, nil, err
	}
	return optimized, &report, nil
}

func optimizeWasmFileIfRequested(path string, enabled bool) (string, *wasmopt.Report, func(), error) {
	cleanup := func() {}
	if !enabled {
		return path, nil, cleanup, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, cleanup, err
	}
	optimized, report, err := wasmopt.EliminateDeadCode(raw)
	if err != nil {
		return "", nil, cleanup, err
	}

	tmp, err := os.CreateTemp("", "erst-opt-*.wasm")
	if err != nil {
		return "", nil, cleanup, err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(optimized); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", nil, cleanup, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", nil, cleanup, err
	}

	cleanup = func() { _ = os.Remove(tmpPath) }
	return tmpPath, &report, cleanup, nil
}

func printOptimizationReport(report *wasmopt.Report) {
	if report == nil {
		return
	}
	fmt.Printf(
		"Optimization (DCE): removed %d/%d functions, kept %d\n",
		report.RemovedDefinedFunctions,
		report.OriginalDefinedFunctions,
		report.KeptDefinedFunctions,
	)
}
