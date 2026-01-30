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

package security

import (
	"strings"
	"testing"
)

func TestDetector_LargeValueTransfer(t *testing.T) {
	detector := NewDetector()

	// Mock envelope with large payment (simplified - in real test would use proper XDR)
	envelopeXdr := ""
	events := []string{}
	logs := []string{}

	findings := detector.Analyze(envelopeXdr, "", events, logs)

	// Should not panic with empty envelope
	if findings == nil {
		t.Fatal("Expected findings slice, got nil")
	}
}

func TestDetector_IntegerOverflow(t *testing.T) {
	detector := NewDetector()

	logs := []string{
		"overflow detected",
	}

	findings := detector.Analyze("", "", []string{}, logs)

	if len(findings) == 0 {
		t.Fatal("Expected overflow finding, got none")
	}

	found := false
	for _, f := range findings {
		if strings.Contains(strings.ToLower(f.Title), "overflow") && f.Type == FindingVerifiedRisk {
			found = true
			if f.Severity != SeverityHigh {
				t.Errorf("Expected HIGH severity, got %s", f.Severity)
			}
		}
	}

	if !found {
		t.Error("Expected integer overflow finding")
	}
}

func TestDetector_AuthorizationFailure(t *testing.T) {
	detector := NewDetector()

	events := []string{
		"ContractEvent: auth_check_failed",
		"HostFunction: InvokeContract",
		"Error: Invalid authorization signature",
	}

	findings := detector.Analyze("", "", events, []string{})

	found := false
	for _, f := range findings {
		if strings.Contains(f.Title, "Authorization Failure") {
			found = true
			if f.Type != FindingVerifiedRisk {
				t.Errorf("Expected VERIFIED_RISK, got %s", f.Type)
			}
		}
	}

	if !found {
		t.Error("Expected authorization failure finding")
	}
}

func TestDetector_ContractPanic(t *testing.T) {
	detector := NewDetector()

	events := []string{
		"Contract execution started",
		"PANIC: index out of bounds",
		"Transaction failed",
	}

	findings := detector.Analyze("", "", events, []string{})

	found := false
	for _, f := range findings {
		if strings.Contains(f.Title, "Panic") && f.Severity == SeverityHigh {
			found = true
		}
	}

	if !found {
		t.Error("Expected panic finding")
	}
}

func TestDetector_ReentrancyPattern(t *testing.T) {
	detector := NewDetector()

	events := []string{
		"contract_data write operation",
		"state change detected",
	}

	// Would need proper XDR envelope with multiple invocations
	findings := detector.Analyze("", "", events, []string{})

	// Should not panic
	if findings == nil {
		t.Fatal("Expected findings slice")
	}
}

func TestDetector_AuthorizationBypass(t *testing.T) {
	detector := NewDetector()

	logs := []string{
		"Executing admin function",
		"Privileged operation: transfer_ownership",
		"Operation completed",
	}

	findings := detector.Analyze("", "", []string{}, logs)

	found := false
	for _, f := range findings {
		if strings.Contains(f.Title, "Authorization Bypass") {
			found = true
			if f.Type != FindingHeuristicWarn {
				t.Errorf("Expected HEURISTIC_WARNING, got %s", f.Type)
			}
		}
	}

	if !found {
		t.Error("Expected authorization bypass warning")
	}
}

func TestDetector_NoFindings(t *testing.T) {
	detector := NewDetector()

	logs := []string{
		"Contract execution successful",
		"All checks passed",
	}
	events := []string{
		"Transfer completed",
	}

	findings := detector.Analyze("", "", events, logs)

	if len(findings) != 0 {
		t.Errorf("Expected no findings for clean execution, got %d", len(findings))
	}
}

func TestDetector_MultipleFindings(t *testing.T) {
	detector := NewDetector()

	logs := []string{
		"Arithmetic overflow in checked_mul",
		"Admin operation without auth check",
	}
	events := []string{
		"PANIC: division by zero",
	}

	findings := detector.Analyze("", "", events, logs)

	if len(findings) < 2 {
		t.Errorf("Expected at least 2 findings, got %d", len(findings))
	}

	// Verify we have both verified risks and heuristic warnings
	hasVerified := false
	hasHeuristic := false

	for _, f := range findings {
		if f.Type == FindingVerifiedRisk {
			hasVerified = true
		}
		if f.Type == FindingHeuristicWarn {
			hasHeuristic = true
		}
	}

	if !hasVerified {
		t.Error("Expected at least one verified risk")
	}
	if !hasHeuristic {
		t.Error("Expected at least one heuristic warning")
	}
}
