// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import "github.com/dotandev/hintents/internal/authtrace"

type SimulationResponse struct {
	Status            string               `json:"status"`
	Error             string               `json:"error,omitempty"`
	ErrorCode         string               `json:"error_code,omitempty"`
	LCOVReport        string               `json:"lcov_report,omitempty"`
	LCOVReportPath    string               `json:"lcov_report_path,omitempty"`
	Events            []string             `json:"events,omitempty"`
	DiagnosticEvents  []DiagnosticEvent    `json:"diagnostic_events,omitempty"`
	Logs              []string             `json:"logs,omitempty"`
	Flamegraph        string               `json:"flamegraph,omitempty"`
	AuthTrace         *authtrace.AuthTrace `json:"auth_trace,omitempty"`
	BudgetUsage       *BudgetUsage         `json:"budget_usage,omitempty"`
	CategorizedEvents []CategorizedEvent   `json:"categorized_events,omitempty"`
	ProtocolVersion   *uint32              `json:"protocol_version,omitempty"`
	StackTrace        *WasmStackTrace      `json:"stack_trace,omitempty"`
	SourceLocation    string               `json:"source_location,omitempty"`
	WasmOffset        *uint64              `json:"wasm_offset,omitempty"`
	LinearMemoryDump  string               `json:"linear_memory_dump,omitempty"`
}

type BudgetUsage struct {
	CPUInstructions    uint64  `json:"cpu_instructions"`
	MemoryBytes        uint64  `json:"memory_bytes"`
	OperationsCount    int     `json:"operations_count"`
	CPULimit           uint64  `json:"cpu_limit"`
	MemoryLimit        uint64  `json:"memory_limit"`
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
}
