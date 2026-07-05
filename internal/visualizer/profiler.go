// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var hostFunctionDocumentation = map[string]string{
	"require_auth":    "require_auth(account): ensures the given account has authorized this contract invocation by checking its signature.",
	"create_contract": "create_contract(code, salt): deploys a new Soroban contract from WASM bytecode and a deterministic salt.",
	"contract_call":   "contract_call(contract_id, function, args...): invokes a contract function on the Soroban host.",
	"invoke":          "invoke(host_fn, args...): calls a host function or contract function at runtime.",
	"storage_put":     "storage_put(key, value): stores a value in contract storage.",
	"storage_get":     "storage_get(key): reads a value from contract storage.",
}

var hostFunctionMatcher = regexp.MustCompile(`\b(require_auth|create_contract|contract_call|invoke|storage_put|storage_get)\b`)

// DiagnosticHint represents a source-level hint for a diagnostics engine.
type DiagnosticHint struct {
	Message string
	Line    int
	Start   uint32
	End     uint32
}

// DescribeHostFunction returns a human-readable description for a Soroban host function.
func DescribeHostFunction(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	doc, ok := hostFunctionDocumentation[name]
	if ok {
		return doc
	}

	return fmt.Sprintf("%s is a Soroban host function. Hover for more details.", name)
}

// HostFunctionHoverContent builds markdown-friendly hover content for the given host function.
func HostFunctionHoverContent(name string) string {
	desc := DescribeHostFunction(name)
	return fmt.Sprintf("**%s**\n\n%s", name, desc)
}

// FormatGasSummary returns a short, readable summary of CPU and memory consumption.
func FormatGasSummary(cpu, mem uint64) string {
	return fmt.Sprintf("CPU: %d instructions · Memory: %d bytes", cpu, mem)
}

// EstimateGasHint returns a quick gas estimation hint based on CPU and memory deltas.
func EstimateGasHint(cpu, mem uint64) string {
	if cpu == 0 && mem == 0 {
		return "No budget information is available for this node."
	}

	parts := []string{FormatGasSummary(cpu, mem)}
	if cpu > 200_000 || mem > 4_096 {
		parts = append(parts, "High resource usage detected; consider optimizing this call.")
	} else if cpu < 50_000 && mem < 1_024 {
		parts = append(parts, "Low resource usage.")
	}

	return strings.Join(parts, " ")
}

// DiagnosticsForSource scans a source string and generates diagnostics for known host functions.
func DiagnosticsForSource(source string) []DiagnosticHint {
	if source == "" {
		return nil
	}

	lines := strings.Split(source, "\n")
	hints := make([]DiagnosticHint, 0)

	for lineIndex, line := range lines {
		matches := hostFunctionMatcher.FindAllStringIndex(line, -1)
		for _, bounds := range matches {
			functionName := strings.TrimSpace(line[bounds[0]:bounds[1]])
			message := fmt.Sprintf("%s — %s", functionName, DescribeHostFunction(functionName))
			hints = append(hints, DiagnosticHint{
				Message: message,
				Line:    lineIndex,
				Start:   uint32(bounds[0]),
				End:     uint32(bounds[1]),
			})
		}
	}

	return hints
}

// KnownHostFunctions returns a sorted list of known host function names.
func KnownHostFunctions() []string {
	keys := make([]string, 0, len(hostFunctionDocumentation))
	for name := range hostFunctionDocumentation {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}
