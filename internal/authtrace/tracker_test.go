// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package authtrace

import (
	"fmt"
	"testing"
)

func TestTrackerRecordEvent(t *testing.T) {
	tracker := NewTracker(Config{MaxEventDepth: 100})

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
	tracker := NewTracker(Config{})

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
	tracker := NewTracker(Config{})

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
	tracker := NewTracker(Config{})

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
	_ = validator.RegisterContract("contract1", auth)

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

func TestRegisterContract_EmptyID(t *testing.T) {
	v := NewCustomContractAuthValidator()
	if v.RegisterContract("", NewMultiSigContractAuth(1, 1, nil)) == nil {
		t.Error("expected error for empty ID")
	}
}
func TestRegisterContract_NilHandler(t *testing.T) {
	v := NewCustomContractAuthValidator()
	if v.RegisterContract("contract1", nil) == nil {
		t.Error("expected error for nil handler")
	}
}
func TestRegisterContract_Overwrite(t *testing.T) {
	v := NewCustomContractAuthValidator()
	_ = v.RegisterContract("c1", NewMultiSigContractAuth(1, 1, map[string]uint32{"k1": 1}))
	_ = v.RegisterContract("c1", NewMultiSigContractAuth(2, 5, map[string]uint32{"k2": 3}))
	info, err := v.GetContractInfo("c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info["details"].(map[string]interface{})["required_signatures"] != 2 {
		t.Error("expected overwritten handler")
	}
}
func TestUnregisterContract_Existing(t *testing.T) {
	v := NewCustomContractAuthValidator()
	_ = v.RegisterContract("c1", NewMultiSigContractAuth(1, 1, nil))
	v.UnregisterContract("c1")
	if len(v.ListContracts()) != 0 {
		t.Error("expected empty list after unregister")
	}
}
func TestUnregisterContract_NonExistent(t *testing.T) {
	NewCustomContractAuthValidator().UnregisterContract("ghost")
}
func TestUnregisterContract_ThenValidate(t *testing.T) {
	v := NewCustomContractAuthValidator()
	_ = v.RegisterContract("c1", NewMultiSigContractAuth(1, 1, nil))
	v.UnregisterContract("c1")
	if _, err := v.ValidateContract("c1", "method", nil); err == nil {
		t.Error("expected error after unregister")
	}
}
func TestGetContractInfo_Missing(t *testing.T) {
	v := NewCustomContractAuthValidator()
	if _, err := v.GetContractInfo("nonexistent"); err == nil {
		t.Error("expected error for missing contract")
	}
}
func TestGetContractInfo_RecoveryHandler(t *testing.T) {
	v := NewCustomContractAuthValidator()
	_ = v.RegisterContract("rc", NewRecoveryAuth("secret_key", 1000))
	info, err := v.GetContractInfo("rc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info["auth_name"] != "recovery" {
		t.Errorf("expected recovery, got %v", info["auth_name"])
	}
	if info["details"].(map[string]interface{})["recovery_key"] != "secret_key" {
		t.Error("expected secret_key")
	}
}
func TestMultiSigValidation_NoParams(t *testing.T) {
	auth := NewMultiSigContractAuth(1, 1, nil)
	valid, err := auth.ValidateAuth("c", "m", []interface{}{})
	if valid || err == nil {
		t.Error("expected failure with no params")
	}
}
func TestMultiSigValidation_InvalidParamType(t *testing.T) {
	auth := NewMultiSigContractAuth(1, 1, nil)
	valid, err := auth.ValidateAuth("c", "m", []interface{}{"not-a-map"})
	if valid || err == nil {
		t.Error("expected failure for non-map param")
	}
}
func TestMultiSigValidation_MissingSignaturesField(t *testing.T) {
	auth := NewMultiSigContractAuth(1, 1, nil)
	valid, err := auth.ValidateAuth("c", "m", []interface{}{map[string]interface{}{"other": "val"}})
	if valid || err == nil {
		t.Error("expected failure for missing signatures field")
	}
}
func TestMultiSigValidation_UnknownSignerKeys(t *testing.T) {
	auth := NewMultiSigContractAuth(1, 5, map[string]uint32{"key1": 5})
	params := []interface{}{map[string]interface{}{"signatures": []interface{}{map[string]interface{}{"signer_key": "unknown"}}}}
	valid, err := auth.ValidateAuth("c", "m", params)
	if valid {
		t.Error("expected failure for unknown signer")
	}
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}
func TestMultiSigValidation_PartialWeightBelowThreshold(t *testing.T) {
	auth := NewMultiSigContractAuth(2, 5, map[string]uint32{"key1": 1, "key2": 1})
	params := []interface{}{map[string]interface{}{"signatures": []interface{}{map[string]interface{}{"signer_key": "key1"}, map[string]interface{}{"signer_key": "key2"}}}}
	if valid, _ := auth.ValidateAuth("c", "m", params); valid {
		t.Error("expected failure: weight below threshold")
	}
}
func TestMultiSigValidation_ExactThreshold(t *testing.T) {
	auth := NewMultiSigContractAuth(2, 5, map[string]uint32{"key1": 3, "key2": 2})
	params := []interface{}{map[string]interface{}{"signatures": []interface{}{map[string]interface{}{"signer_key": "key1"}, map[string]interface{}{"signer_key": "key2"}}}}
	valid, err := auth.ValidateAuth("c", "m", params)
	if !valid {
		t.Errorf("expected success at exact threshold: %v", err)
	}
}
func TestMultiSigValidation_DuplicateSignerEntries(t *testing.T) {
	auth := NewMultiSigContractAuth(2, 3, map[string]uint32{"key1": 2})
	params := []interface{}{map[string]interface{}{"signatures": []interface{}{map[string]interface{}{"signer_key": "key1"}, map[string]interface{}{"signer_key": "key1"}}}}
	if valid, _ := auth.ValidateAuth("c", "m", params); !valid {
		t.Error("expected success with duplicate weight accumulation")
	}
}
func TestMultiSigValidation_NonMapSignatureEntries(t *testing.T) {
	auth := NewMultiSigContractAuth(1, 10, map[string]uint32{"key1": 10})
	params := []interface{}{map[string]interface{}{"signatures": []interface{}{"not-a-map", map[string]interface{}{"signer_key": "key1"}}}}
	valid, err := auth.ValidateAuth("c", "m", params)
	if !valid {
		t.Errorf("expected success after skipping bad entry: %v", err)
	}
}
func TestMultiSigDetails_ContainsTotalSigners(t *testing.T) {
	auth := NewMultiSigContractAuth(2, 4, map[string]uint32{"a": 1, "b": 2, "c": 3})
	details := auth.GetAuthDetails()
	if details["total_signers"] != 3 {
		t.Errorf("expected 3, got %v", details["total_signers"])
	}
	if details["signer_threshold"] != uint32(4) {
		t.Errorf("expected 4, got %v", details["signer_threshold"])
	}
}
func TestNewMultiSigContractAuth_NilSigners(t *testing.T) {
	if NewMultiSigContractAuth(1, 1, nil).Signers == nil {
		t.Error("expected non-nil Signers map")
	}
}
func TestRecoveryAuth_InsufficientParams(t *testing.T) {
	r := NewRecoveryAuth("key", 0)
	valid, err := r.ValidateAuth("c", "m", []interface{}{"key"})
	if valid || err == nil {
		t.Error("expected failure with 1 param")
	}
}
func TestRecoveryAuth_InvalidKeyType(t *testing.T) {
	r := NewRecoveryAuth("key", 0)
	valid, err := r.ValidateAuth("c", "m", []interface{}{12345, nil})
	if valid || err == nil {
		t.Error("expected failure for non-string key")
	}
}
func TestRecoveryAuth_EmptyKeyMatch(t *testing.T) {
	r := NewRecoveryAuth("", 0)
	valid, err := r.ValidateAuth("c", "m", []interface{}{"", nil})
	if !valid {
		t.Errorf("expected success for matching empty keys: %v", err)
	}
}
func TestRecoveryAuthDetails(t *testing.T) {
	details := NewRecoveryAuth("my_key", 5000).GetAuthDetails()
	if details["recovery_key"] != "my_key" {
		t.Errorf("expected my_key, got %v", details["recovery_key"])
	}
	if details["delay_ms"] != uint64(5000) {
		t.Errorf("expected 5000, got %v", details["delay_ms"])
	}
}
func TestNestedAuth_MultipleContractsInSameValidator(t *testing.T) {
	v := NewCustomContractAuthValidator()
	_ = v.RegisterContract("ms", NewMultiSigContractAuth(1, 3, map[string]uint32{"k1": 3}))
	_ = v.RegisterContract("rc", NewRecoveryAuth("recover_me", 0))
	valid, err := v.ValidateContract("ms", "transfer", []interface{}{map[string]interface{}{"signatures": []interface{}{map[string]interface{}{"signer_key": "k1"}}}})
	if !valid {
		t.Errorf("multi-sig failed: %v", err)
	}
	valid, err = v.ValidateContract("rc", "recover", []interface{}{"recover_me", nil})
	if !valid {
		t.Errorf("recovery failed: %v", err)
	}
}
func TestNestedAuth_SequentialValidation(t *testing.T) {
	v := NewCustomContractAuthValidator()
	_ = v.RegisterContract("gate", NewRecoveryAuth("gate_key", 0))
	_ = v.RegisterContract("action", NewMultiSigContractAuth(1, 2, map[string]uint32{"signer1": 2}))
	if ok, err := v.ValidateContract("gate", "unlock", []interface{}{"gate_key", nil}); !ok {
		t.Fatalf("gate failed: %v", err)
	}
	valid, err := v.ValidateContract("action", "execute", []interface{}{map[string]interface{}{"signatures": []interface{}{map[string]interface{}{"signer_key": "signer1"}}}})
	if !valid {
		t.Errorf("action failed after gate passed: %v", err)
	}
}
func TestNestedAuth_FailAtGate(t *testing.T) {
	v := NewCustomContractAuthValidator()
	_ = v.RegisterContract("gate", NewRecoveryAuth("correct_key", 0))
	if ok, _ := v.ValidateContract("gate", "unlock", []interface{}{"wrong_key", nil}); ok {
		t.Fatal("gate should reject wrong key")
	}
}
func TestNestedAuth_ContractReplacementMidFlow(t *testing.T) {
	v := NewCustomContractAuthValidator()
	_ = v.RegisterContract("auth", NewMultiSigContractAuth(1, 1, map[string]uint32{"k": 1}))
	params := []interface{}{map[string]interface{}{"signatures": []interface{}{map[string]interface{}{"signer_key": "k"}}}}
	if ok, _ := v.ValidateContract("auth", "m", params); !ok {
		t.Fatal("expected success with weak auth")
	}
	_ = v.RegisterContract("auth", NewMultiSigContractAuth(1, 100, map[string]uint32{"k": 1}))
	if ok, _ := v.ValidateContract("auth", "m", params); ok {
		t.Error("expected failure after strict replacement")
	}
}
func TestUnmarshalCustomContractAuth_ValidMultiSig(t *testing.T) {
	v, err := UnmarshalCustomContractAuth([]byte(`{"c1":{"type":"multi_sig","required_signatures":2,"signer_threshold":3}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v.ListContracts()) != 1 {
		t.Errorf("expected 1, got %d", len(v.ListContracts()))
	}
}
func TestUnmarshalCustomContractAuth_ValidRecovery(t *testing.T) {
	v, err := UnmarshalCustomContractAuth([]byte(`{"rc1":{"type":"recovery","recovery_key":"my_secret","delay":500}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, _ := v.GetContractInfo("rc1")
	if info["auth_name"] != "recovery" {
		t.Errorf("expected recovery, got %v", info["auth_name"])
	}
}
func TestUnmarshalCustomContractAuth_UnknownType(t *testing.T) {
	if _, err := UnmarshalCustomContractAuth([]byte(`{"c1":{"type":"unknown_type"}}`)); err == nil {
		t.Error("expected error")
	}
}
func TestUnmarshalCustomContractAuth_MissingType(t *testing.T) {
	if _, err := UnmarshalCustomContractAuth([]byte(`{"c1":{"required_signatures":1}}`)); err == nil {
		t.Error("expected error")
	}
}
func TestUnmarshalCustomContractAuth_InvalidJSON(t *testing.T) {
	if _, err := UnmarshalCustomContractAuth([]byte(`not-json`)); err == nil {
		t.Error("expected error")
	}
}
func TestUnmarshalCustomContractAuth_MultipleContracts(t *testing.T) {
	v, err := UnmarshalCustomContractAuth([]byte(`{"ms":{"type":"multi_sig","required_signatures":1,"signer_threshold":1},"rc":{"type":"recovery","recovery_key":"k","delay":0}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v.ListContracts()) != 2 {
		t.Errorf("expected 2, got %d", len(v.ListContracts()))
	}
}
func TestTrackerRecordCustomContractCall_Success(t *testing.T) {
	tracker := NewTracker(Config{})
	tracker.RecordCustomContractCall("ACCT1", "contract1", "transfer", nil, "success", nil)
	trace := tracker.GenerateTrace()
	if len(trace.AuthEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(trace.AuthEvents))
	}
	if trace.AuthEvents[0].EventType != "custom_contract_auth" {
		t.Errorf("wrong type: %s", trace.AuthEvents[0].EventType)
	}
	if trace.AuthEvents[0].ErrorReason != "" {
		t.Errorf("expected no error reason, got: %s", trace.AuthEvents[0].ErrorReason)
	}
}
func TestTrackerRecordCustomContractCall_WithError(t *testing.T) {
	tracker := NewTracker(Config{})
	tracker.RecordCustomContractCall("ACCT1", "contract1", "transfer", nil, "failed", fmt.Errorf("auth denied"))
	if tracker.GenerateTrace().AuthEvents[0].ErrorReason != ReasonCustomContractFailed {
		t.Error("expected ReasonCustomContractFailed")
	}
}
func TestTrackerGetAuthEvents_FiltersByAccount(t *testing.T) {
	tracker := NewTracker(Config{})
	tracker.RecordEvent(AuthEvent{EventType: "e", AccountID: "A", Status: "valid"})
	tracker.RecordEvent(AuthEvent{EventType: "e", AccountID: "B", Status: "valid"})
	tracker.RecordEvent(AuthEvent{EventType: "e", AccountID: "A", Status: "valid"})
	if events := tracker.GetAuthEvents("A"); len(events) != 2 {
		t.Errorf("expected 2, got %d", len(events))
	}
}
func TestTrackerGetFailureReport_ReturnsNilForUnknown(t *testing.T) {
	if NewTracker(Config{}).GetFailureReport("UNKNOWN") != nil {
		t.Error("expected nil")
	}
}
func TestTrackerMaxEventDepth_IgnoresOverflow(t *testing.T) {
	tracker := NewTracker(Config{MaxEventDepth: 2})
	for i := 0; i < 5; i++ {
		tracker.RecordEvent(AuthEvent{EventType: "e", Status: "valid"})
	}
	if trace := tracker.GenerateTrace(); len(trace.AuthEvents) != 2 {
		t.Errorf("expected 2, got %d", len(trace.AuthEvents))
	}
}
