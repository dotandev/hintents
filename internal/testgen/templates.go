// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testgen

// goTestTemplate is the template for generating Go regression tests
const goTestTemplate = `package regression_tests

import (
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegression_{{.TestName}} is a regression test for transaction {{.TxHash}}
func TestRegression_{{.TestName}}(t *testing.T) {
	// Create simulation request with captured transaction data
	req := &simulator.SimulationRequest{
		EnvelopeXdr:   "{{.EnvelopeXdr}}",
		ResultMetaXdr: "{{.ResultMetaXdr}}",
		LedgerEntries: map[string]string{
{{- range .LedgerEntries}}
			"{{.Key}}": "{{.Value}}",
{{- end}}
		},
	}

	// Create simulator runner
	runner, err := simulator.NewRunner("", false)
	require.NoError(t, err, "Failed to create simulator runner")

	// Run simulation
	resp, err := runner.Run(req)
	require.NoError(t, err, "Simulation failed")

	// Assert expected behavior
	assert.Equal(t, "success", resp.Status, "Expected successful simulation")
	assert.NotEmpty(t, resp.Logs, "Expected simulation logs")
	
	// TODO: Add more specific assertions based on expected behavior
	t.Logf("Simulation completed successfully")
	t.Logf("Events: %v", resp.Events)
	t.Logf("Logs: %v", resp.Logs)
}
`

// rustTestTemplate is the template for generating Rust regression tests
const rustTestTemplate = `use serde_json;
use std::collections::HashMap;

// Test data structures (should match main.rs)
#[derive(serde::Serialize)]
struct SimulationRequest {
    envelope_xdr: String,
    result_meta_xdr: String,
    ledger_entries: Option<HashMap<String, String>>,
}

#[derive(serde::Deserialize, Debug)]
struct SimulationResponse {
    status: String,
    error: Option<String>,
    events: Vec<String>,
    logs: Vec<String>,
}

/// Regression test for transaction {{.TxHash}}
#[test]
fn test_regression_{{.TestName}}() {
    // Create simulation request with captured transaction data
    let mut ledger_entries = HashMap::new();
{{- range .LedgerEntries}}
    ledger_entries.insert("{{.Key}}".to_string(), "{{.Value}}".to_string());
{{- end}}

    let request = SimulationRequest {
        envelope_xdr: "{{.EnvelopeXdr}}".to_string(),
        result_meta_xdr: "{{.ResultMetaXdr}}".to_string(),
        ledger_entries: if ledger_entries.is_empty() {
            None
        } else {
            Some(ledger_entries)
        },
    };

    // Serialize request to JSON
    let request_json = serde_json::to_string(&request).expect("Failed to serialize request");

    // Run the simulator binary
    let output = std::process::Command::new("../target/release/erst-sim")
        .stdin(std::process::Stdio::piped())
        .stdout(std::process::Stdio::piped())
        .stderr(std::process::Stdio::piped())
        .spawn()
        .expect("Failed to spawn simulator")
        .stdin
        .unwrap()
        .write_all(request_json.as_bytes())
        .expect("Failed to write to stdin");

    // TODO: Complete the test implementation
    // This is a placeholder - full implementation would:
    // 1. Capture stdout/stderr
    // 2. Parse response JSON
    // 3. Assert expected behavior

    println!("Regression test for {{.TxHash}} - implementation pending");
}
`
