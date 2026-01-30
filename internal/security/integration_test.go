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
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stellar/go/xdr"
)

// TestDetector_FlawedContract simulates a known flawed contract with multiple vulnerabilities
func TestDetector_FlawedContract(t *testing.T) {
	detector := NewDetector()

	// Simulate a flawed contract scenario with:
	// 1. Integer overflow
	// 2. Missing authorization
	// 3. Contract panic
	// 4. Large value transfer

	events := []string{
		"ContractEvent: transfer initiated",
		"PANIC: arithmetic overflow in balance calculation",
		"ContractEvent: state_write attempted",
		"Error: transaction aborted",
	}

	logs := []string{
		"Executing admin_withdraw function",
		"Amount: 999999999999999",
		"checked_add failed: overflow detected",
		"Privileged admin operation executed",
		"Contract trap: division by zero",
	}

	// Create a mock envelope with large payment
	envelope := createMockEnvelopeWithLargePayment(t)
	envelopeXdr := encodeEnvelope(t, envelope)

	findings := detector.Analyze(envelopeXdr, "", events, logs)

	if len(findings) == 0 {
		t.Fatal("Expected multiple findings for flawed contract, got none")
	}

	t.Logf("Detected %d security findings", len(findings))

	// Verify we caught the major issues
	foundOverflow := false
	foundAuthBypass := false
	foundPanic := false
	foundLargeTransfer := false

	for _, finding := range findings {
		t.Logf("Finding: [%s] %s - %s", finding.Type, finding.Severity, finding.Title)
		t.Logf("  Description: %s", finding.Description)
		if finding.Evidence != "" {
			t.Logf("  Evidence: %s", finding.Evidence)
		}

		if strings.Contains(finding.Title, "Integer Overflow") {
			foundOverflow = true
			if finding.Type != FindingVerifiedRisk {
				t.Errorf("Overflow should be VERIFIED_RISK, got %s", finding.Type)
			}
			if finding.Severity != SeverityHigh {
				t.Errorf("Overflow should be HIGH severity, got %s", finding.Severity)
			}
		}

		if strings.Contains(finding.Title, "Authorization Bypass") {
			foundAuthBypass = true
			if finding.Type != FindingHeuristicWarn {
				t.Errorf("Auth bypass should be HEURISTIC_WARNING, got %s", finding.Type)
			}
		}

		if strings.Contains(finding.Title, "Panic") {
			foundPanic = true
			if finding.Type != FindingVerifiedRisk {
				t.Errorf("Panic should be VERIFIED_RISK, got %s", finding.Type)
			}
		}

		if strings.Contains(finding.Title, "Large") && strings.Contains(finding.Title, "Transfer") {
			foundLargeTransfer = true
		}
	}

	if !foundOverflow {
		t.Error("Failed to detect integer overflow vulnerability")
	}
	if !foundAuthBypass {
		t.Error("Failed to detect authorization bypass")
	}
	if !foundPanic {
		t.Error("Failed to detect contract panic")
	}
	if !foundLargeTransfer {
		t.Error("Failed to detect large value transfer")
	}

	// Verify distinction between verified risks and heuristic warnings
	verifiedCount := 0
	heuristicCount := 0

	for _, finding := range findings {
		if finding.Type == FindingVerifiedRisk {
			verifiedCount++
		} else if finding.Type == FindingHeuristicWarn {
			heuristicCount++
		}
	}

	if verifiedCount == 0 {
		t.Error("Expected at least one verified risk")
	}
	if heuristicCount == 0 {
		t.Error("Expected at least one heuristic warning")
	}

	t.Logf("Summary: %d verified risks, %d heuristic warnings", verifiedCount, heuristicCount)
}

// TestDetector_TypeDistinction verifies clear distinction between risk types
func TestDetector_TypeDistinction(t *testing.T) {
	tests := []struct {
		name            string
		events          []string
		logs            []string
		expectVerified  bool
		expectHeuristic bool
	}{
		{
			name:            "Verified Risk - Overflow",
			events:          []string{},
			logs:            []string{"overflow detected"},
			expectVerified:  true,
			expectHeuristic: false,
		},
		{
			name:            "Heuristic Warning - Auth Pattern",
			events:          []string{},
			logs:            []string{"admin operation", "privileged access"},
			expectVerified:  false,
			expectHeuristic: true,
		},
		{
			name:            "Verified Risk - Auth Failure",
			events:          []string{"auth check failed"},
			logs:            []string{},
			expectVerified:  true,
			expectHeuristic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewDetector()
			findings := detector.Analyze("", "", tt.events, tt.logs)

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

			if tt.expectVerified && !hasVerified {
				t.Error("Expected verified risk but none found")
			}
			if !tt.expectVerified && hasVerified {
				t.Error("Found verified risk when none expected")
			}
			if tt.expectHeuristic && !hasHeuristic {
				t.Error("Expected heuristic warning but none found")
			}
			if !tt.expectHeuristic && hasHeuristic {
				t.Error("Found heuristic warning when none expected")
			}
		})
	}
}

// Helper functions for test

func createMockEnvelopeWithLargePayment(t *testing.T) xdr.TransactionEnvelope {
	t.Helper()

	// Use valid Stellar addresses (both using the same valid address for simplicity)
	sourceAccount := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
	destAccount := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")

	payment := xdr.PaymentOp{
		Destination: destAccount.ToMuxedAccount(),
		Asset:       xdr.MustNewNativeAsset(),
		Amount:      xdr.Int64(20000000 * 10000000), // 20M XLM
	}

	op := xdr.Operation{
		SourceAccount: nil,
		Body: xdr.OperationBody{
			Type:      xdr.OperationTypePayment,
			PaymentOp: &payment,
		},
	}

	tx := xdr.Transaction{
		SourceAccount: sourceAccount.ToMuxedAccount(),
		Fee:           100,
		SeqNum:        1,
		Operations:    []xdr.Operation{op},
	}

	envelope := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: tx,
		},
	}

	return envelope
}

func encodeEnvelope(t *testing.T, envelope xdr.TransactionEnvelope) string {
	t.Helper()

	xdrBytes, err := envelope.MarshalBinary()
	if err != nil {
		t.Fatalf("Failed to marshal envelope: %v", err)
	}

	return base64.StdEncoding.EncodeToString(xdrBytes)
}
