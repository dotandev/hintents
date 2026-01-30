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

package authtrace

import (
	"testing"
)

func TestTrackerRecordEvent(t *testing.T) {
	tracker := NewTracker(AuthTraceConfig{MaxEventDepth: 100})

	event := AuthEvent{
		EventType:     "signature_verification",
		AccountID:     "GTEST",
		SignerKey:     "key1",
		SignatureType: Ed25519,
		Status:        "valid",
		Weight:        1,
	}

	tracker.RecordEvent(event)

	trace := tracker.GenerateTrace()
	if len(trace.AuthEvents) != 1 {
		t.Errorf("expected 1 event, got %d", len(trace.AuthEvents))
	}
}

func TestTrackerSignatureVerification(t *testing.T) {
	tracker := NewTracker(AuthTraceConfig{})

	signers := []SignerInfo{
		{AccountID: "GTEST", SignerKey: "key1", SignerType: Ed25519, Weight: 1},
	}
	thresholds := ThresholdConfig{LowThreshold: 1, MediumThreshold: 2, HighThreshold: 3}

	tracker.InitializeAccountContext("GTEST", signers, thresholds)
	tracker.RecordSignatureVerification("GTEST", "key1", Ed25519, true, 1)

	trace := tracker.GenerateTrace()
	if trace.ValidSignatures != 1 {
		t.Errorf("expected 1 valid signature, got %d", trace.ValidSignatures)
	}
}

func TestTrackerThresholdCheck(t *testing.T) {
	tracker := NewTracker(AuthTraceConfig{})

	tracker.RecordThresholdCheck("GTEST", 2, 1, false)

	trace := tracker.GenerateTrace()
	if trace.Success {
		t.Error("expected trace to show failure")
	}
	if len(trace.Failures) != 1 {
		t.Errorf("expected 1 failure, got %d", len(trace.Failures))
	}
}

func TestTrackerClear(t *testing.T) {
	tracker := NewTracker(AuthTraceConfig{})

	event := AuthEvent{EventType: "test", Status: "valid"}
	tracker.RecordEvent(event)

	if len(tracker.GenerateTrace().AuthEvents) != 1 {
		t.Error("expected event after recording")
	}

	tracker.Clear()

	if len(tracker.GenerateTrace().AuthEvents) != 0 {
		t.Error("expected no events after clear")
	}
}

func TestMultiSigContractAuth(t *testing.T) {
	signers := map[string]uint32{
		"key1": 1,
		"key2": 1,
	}

	auth := NewMultiSigContractAuth(1, 2, signers)

	if auth.GetAuthName() != "multi_sig" {
		t.Errorf("expected auth name 'multi_sig', got %s", auth.GetAuthName())
	}

	details := auth.GetAuthDetails()
	if details["required_signatures"] != 1 {
		t.Error("expected required_signatures in details")
	}
}

func TestMultiSigValidationInsufficientSigs(t *testing.T) {
	signers := map[string]uint32{"key1": 1}
	auth := NewMultiSigContractAuth(2, 2, signers)

	params := []interface{}{map[string]interface{}{
		"signatures": []interface{}{},
	}}

	valid, err := auth.ValidateAuth("contract1", "method", params)
	if valid {
		t.Error("expected validation to fail with insufficient signatures")
	}
	if err == nil {
		t.Error("expected error for insufficient signatures")
	}
}

func TestMultiSigValidationSufficientSigs(t *testing.T) {
	signers := map[string]uint32{
		"key1": 1,
		"key2": 1,
	}
	auth := NewMultiSigContractAuth(2, 2, signers)

	params := []interface{}{map[string]interface{}{
		"signatures": []interface{}{
			map[string]interface{}{"signer_key": "key1"},
			map[string]interface{}{"signer_key": "key2"},
		},
	}}

	valid, err := auth.ValidateAuth("contract1", "method", params)
	if !valid {
		t.Errorf("expected validation to succeed, got error: %v", err)
	}
}

func TestRecoveryAuthValidation(t *testing.T) {
	recovery := NewRecoveryAuth("recovery_key_123", 0)

	params := []interface{}{"recovery_key_123", nil}

	valid, err := recovery.ValidateAuth("contract1", "recover", params)
	if !valid {
		t.Errorf("expected recovery validation to succeed, got error: %v", err)
	}
}

func TestRecoveryAuthValidationWrongKey(t *testing.T) {
	recovery := NewRecoveryAuth("recovery_key_123", 0)

	params := []interface{}{"wrong_key"}

	valid, err := recovery.ValidateAuth("contract1", "recover", params)
	if valid {
		t.Error("expected recovery validation to fail with wrong key")
	}
	if err == nil {
		t.Error("expected error for wrong recovery key")
	}
}

func TestCustomContractAuthValidatorRegister(t *testing.T) {
	validator := NewCustomContractAuthValidator()

	auth := NewMultiSigContractAuth(1, 1, nil)
	err := validator.RegisterContract("contract1", auth)
	if err != nil {
		t.Errorf("failed to register contract: %v", err)
	}

	contracts := validator.ListContracts()
	if len(contracts) != 1 || contracts[0] != "contract1" {
		t.Error("failed to list registered contracts")
	}
}

func TestCustomContractAuthValidatorValidate(t *testing.T) {
	validator := NewCustomContractAuthValidator()

	signers := map[string]uint32{"key1": 1}
	auth := NewMultiSigContractAuth(1, 1, signers)
	validator.RegisterContract("contract1", auth)

	params := []interface{}{map[string]interface{}{
		"signatures": []interface{}{map[string]interface{}{"signer_key": "key1"}},
	}}

	valid, err := validator.ValidateContract("contract1", "method", params)
	if !valid {
		t.Errorf("expected contract validation to succeed, got error: %v", err)
	}
}

func TestDetailedReporterGenerateReport(t *testing.T) {
	trace := &AuthTrace{
		Success:   false,
		AccountID: "GTEST",
		AuthEvents: []AuthEvent{
			{EventType: "signature_verification", SignerKey: "key1", Status: "valid", Weight: 1},
		},
		Failures: []AuthFailure{
			{AccountID: "GTEST", FailureReason: ReasonThresholdNotMet, RequiredWeight: 2, CollectedWeight: 1},
		},
	}

	reporter := NewDetailedReporter(trace)
	report := reporter.GenerateReport()

	if report == "" {
		t.Error("expected non-empty report")
	}

	if !contains(report, "FAILED") {
		t.Error("expected failure indicator in report")
	}
}

func TestDetailedReporterSummaryMetrics(t *testing.T) {
	trace := &AuthTrace{
		Success:         true,
		AccountID:       "GTEST",
		SignerCount:     2,
		ValidSignatures: 2,
		AuthEvents:      make([]AuthEvent, 0),
		Failures:        make([]AuthFailure, 0),
		CustomContracts: make([]CustomContractAuth, 0),
	}

	reporter := NewDetailedReporter(trace)
	metrics := reporter.SummaryMetrics()

	if metrics["success"] != true {
		t.Error("expected success in metrics")
	}

	if metrics["total_signers"] != uint32(2) {
		t.Error("expected total_signers in metrics")
	}
}

func TestDetailedReporterIdentifyMissingKeys(t *testing.T) {
	failedSigners := []SignerInfo{
		{SignerKey: "key1", Weight: 1},
		{SignerKey: "key2", Weight: 2},
	}

	trace := &AuthTrace{
		Success: false,
		Failures: []AuthFailure{
			{FailedSigners: failedSigners},
		},
	}

	reporter := NewDetailedReporter(trace)
	missing := reporter.IdentifyMissingKeys()

	if len(missing) != 2 {
		t.Errorf("expected 2 missing keys, got %d", len(missing))
	}
}

func contains(str, substr string) bool {
	for i := 0; i < len(str)-len(substr)+1; i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
