// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/trace"
)

// TestPrintTraceTree_NoColor verifies that PrintTraceTree emits expected
// structural elements (tree connectors, icons, labels, summary) when colours
// are disabled.
func TestPrintTraceTree_NoColor(t *testing.T) {
	root := trace.CreateMockTrace()

	var buf bytes.Buffer
	opts := trace.PrintOptions{
		NoColor:  true,
		MaxWidth: 80,
		Output:   &buf,
	}
	trace.PrintTraceTree(root, opts)

	out := buf.String()

	checks := []struct {
		name string
		want string
	}{
		{"header", "Transaction Execution Trace"},
		{"root node present", "Transaction: 5c0a1234567890abcdef"},
		{"tree connector", "├─"},
		{"leaf connector", "└─"},
		{"contract func transfer", "transfer"},
		{"error icon", "[FAIL]"},
		{"error text", "Insufficient balance"},
		{"cpu budget", "CPU:"},
		{"mem budget", "MEM:"},
		{"summary steps", "Steps:"},
		{"summary errors", "Errors: 1"},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(out, c.want) {
				t.Errorf("expected output to contain %q\ngot:\n%s", c.want, out)
			}
		})
	}
}

// TestPrintExecutionTrace_NoColor verifies that PrintExecutionTrace emits the
// expected fields from an ExecutionTrace built from the sample states.
func TestPrintExecutionTrace_NoColor(t *testing.T) {
	et := trace.NewExecutionTrace("test-tx-abc", 0)

	cpuVal := uint64(100000)
	et.AddState(trace.ExecutionState{
		Operation:  "contract_call",
		ContractID: "CONTRACT_A",
		Function:   "init",
		HostState:  map[string]interface{}{"cpu_instructions": cpuVal},
	})
	et.AddState(trace.ExecutionState{
		Operation:   "contract_call",
		ContractID:  "CONTRACT_A",
		Function:    "transfer",
		ReturnValue: "ok",
	})
	et.AddState(trace.ExecutionState{
		Operation:  "contract_call",
		ContractID: "CONTRACT_A",
		Function:   "fail_fn",
		Error:      "out of gas",
	})

	var buf bytes.Buffer
	trace.PrintExecutionTrace(et, trace.PrintOptions{
		NoColor:  true,
		MaxWidth: 80,
		Output:   &buf,
	})

	out := buf.String()

	for _, want := range []string{
		"test-tx-abc",
		"Steps : 3",
		"CONTRACT_CALL",
		"transfer",
		"fail_fn",
		"[FAIL]",
		"out of gas",
		"Steps: 3",
		"Errors: 1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q\ngot:\n%s", want, out)
		}
	}
}

// TestPrintOptions_Width verifies that a narrow MaxWidth is respected and does
// not cause a panic or infinite loop.
func TestPrintOptions_Width(t *testing.T) {
	root := trace.CreateMockTrace()
	var buf bytes.Buffer
	// Very narrow to exercise truncation paths
	trace.PrintTraceTree(root, trace.PrintOptions{
		NoColor:  true,
		MaxWidth: 30,
		Output:   &buf,
	})
	if buf.Len() == 0 {
		t.Error("expected non-empty output for narrow terminal")
	}
}
