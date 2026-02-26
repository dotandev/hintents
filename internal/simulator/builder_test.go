// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"errors"
	"strings"
	"testing"

	internalErrors "github.com/dotandev/hintents/internal/errors"
)

func TestSimulationRequestBuilder_WithRestorePreamble(t *testing.T) {
	builder := NewSimulationRequestBuilder()

	preamble := map[string]interface{}{
		"foo":   "bar",
		"count": 42,
	}

	req, err := builder.
		WithEnvelopeXDR("envelope").
		WithResultMetaXDR("result").
		WithRestorePreamble(preamble).
		Build()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if req.RestorePreamble == nil {
		t.Fatalf("expected RestorePreamble to be set")
	}
	if req.RestorePreamble["foo"] != "bar" {
		t.Errorf("expected RestorePreamble['foo'] to be 'bar', got: %v", req.RestorePreamble["foo"])
	}
	if req.RestorePreamble["count"] != 42 {
		t.Errorf("expected RestorePreamble['count'] to be 42, got: %v", req.RestorePreamble["count"])
	}
}

func TestSimulationRequestBuilder_Basic(t *testing.T) {
	builder := NewSimulationRequestBuilder()

	req, err := builder.
		WithEnvelopeXDR("envelope_xdr_data").
		WithResultMetaXDR("result_meta_xdr_data").
		Build()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if req.EnvelopeXdr != "envelope_xdr_data" {
		t.Errorf("expected EnvelopeXdr to be 'envelope_xdr_data', got: %s", req.EnvelopeXdr)
	}

	if req.ResultMetaXdr != "result_meta_xdr_data" {
		t.Errorf("expected ResultMetaXdr to be 'result_meta_xdr_data', got: %s", req.ResultMetaXdr)
	}

	if req.LedgerEntries != nil {
		t.Errorf("expected LedgerEntries to be nil, got: %v", req.LedgerEntries)
	}
}

func TestSimulationRequestBuilder_WithLedgerEntry(t *testing.T) {
	builder := NewSimulationRequestBuilder()

	req, err := builder.
		WithEnvelopeXDR("envelope").
		WithResultMetaXDR("result").
		WithLedgerEntry("key1", "value1").
		WithLedgerEntry("key2", "value2").
		Build()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(req.LedgerEntries) != 2 {
		t.Errorf("expected 2 ledger entries, got: %d", len(req.LedgerEntries))
	}

	if req.LedgerEntries["key1"] != "value1" {
		t.Errorf("expected ledger entry 'key1' to be 'value1', got: %s", req.LedgerEntries["key1"])
	}

	if req.LedgerEntries["key2"] != "value2" {
		t.Errorf("expected ledger entry 'key2' to be 'value2', got: %s", req.LedgerEntries["key2"])
	}
}

func TestSimulationRequestBuilder_WithLedgerEntries(t *testing.T) {
	builder := NewSimulationRequestBuilder()

	entries := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	req, err := builder.
		WithEnvelopeXDR("envelope").
		WithResultMetaXDR("result").
		WithLedgerEntries(entries).
		Build()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(req.LedgerEntries) != 3 {
		t.Errorf("expected 3 ledger entries, got: %d", len(req.LedgerEntries))
	}

	for key, expectedValue := range entries {
		if actualValue, ok := req.LedgerEntries[key]; !ok {
			t.Errorf("expected ledger entry '%s' to exist", key)
		} else if actualValue != expectedValue {
			t.Errorf("expected ledger entry '%s' to be '%s', got: %s", key, expectedValue, actualValue)
		}
	}
}

func TestSimulationRequestBuilder_MissingEnvelopeXDR(t *testing.T) {
	builder := NewSimulationRequestBuilder()

	_, err := builder.
		WithResultMetaXDR("result").
		Build()

	if !errors.Is(err, internalErrors.ErrValidationFailed) {
		t.Errorf("expected ErrValidationFailed, got: %v", err)
	}

	expectedSubError := "envelope XDR is required"
	if !strings.Contains(err.Error(), expectedSubError) {
		t.Errorf("expected error to contain '%s', got: %v", expectedSubError, err)
	}
}

func TestSimulationRequestBuilder_MissingResultMetaXDR(t *testing.T) {
	builder := NewSimulationRequestBuilder()

	_, err := builder.
		WithEnvelopeXDR("envelope").
		Build()

	if err == nil {
		t.Fatal("expected error for missing result meta XDR, got nil")
	}

	if !errors.Is(err, internalErrors.ErrValidationFailed) {
		t.Errorf("expected ErrValidationFailed, got: %v", err)
	}

	expectedSubError := "result meta XDR is required"
	if !strings.Contains(err.Error(), expectedSubError) {
		t.Errorf("expected error to contain '%s', got: %v", expectedSubError, err)
	}
}

func TestSimulationRequestBuilder_EmptyLedgerEntryKey(t *testing.T) {
	builder := NewSimulationRequestBuilder()

	_, err := builder.
		WithEnvelopeXDR("envelope").
		WithResultMetaXDR("result").
		WithLedgerEntry("", "value").
		Build()

	if err == nil {
		t.Fatal("expected error for empty ledger entry key, got nil")
	}
}

func TestSimulationRequestBuilder_EmptyLedgerEntryValue(t *testing.T) {
	builder := NewSimulationRequestBuilder()

	_, err := builder.
		WithEnvelopeXDR("envelope").
		WithResultMetaXDR("result").
		WithLedgerEntry("key", "").
		Build()

	if err == nil {
		t.Fatal("expected error for empty ledger entry value, got nil")
	}
}

func TestSimulationRequestBuilder_MustBuild_Success(t *testing.T) {
	builder := NewSimulationRequestBuilder()

	// Should not panic
	req := builder.
		WithEnvelopeXDR("envelope").
		WithResultMetaXDR("result").
		MustBuild()

	if req == nil {
		t.Fatal("expected non-nil request")
	}
}

func TestSimulationRequestBuilder_MustBuild_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustBuild to panic, but it didn't")
		}
	}()

	builder := NewSimulationRequestBuilder()

	// Should panic due to missing required fields
	builder.MustBuild()
}

func TestSimulationRequestBuilder_Reset(t *testing.T) {
	builder := NewSimulationRequestBuilder()

	// Build first request
	req1, err := builder.
		WithEnvelopeXDR("envelope1").
		WithResultMetaXDR("result1").
		WithLedgerEntry("key1", "value1").
		Build()

	if err != nil {
		t.Fatalf("expected no error for first build, got: %v", err)
	}

	if req1.EnvelopeXdr != "envelope1" {
		t.Errorf("expected first request EnvelopeXdr to be 'envelope1', got: %s", req1.EnvelopeXdr)
	}

	// Reset and build second request
	req2, err := builder.
		Reset().
		WithEnvelopeXDR("envelope2").
		WithResultMetaXDR("result2").
		Build()

	if err != nil {
		t.Fatalf("expected no error for second build, got: %v", err)
	}

	if req2.EnvelopeXdr != "envelope2" {
		t.Errorf("expected second request EnvelopeXdr to be 'envelope2', got: %s", req2.EnvelopeXdr)
	}

	if req2.LedgerEntries != nil {
		t.Errorf("expected second request LedgerEntries to be nil after reset, got: %v", req2.LedgerEntries)
	}
}

func TestSimulationRequestBuilder_MethodChaining(t *testing.T) {
	// Test that all methods return the builder for chaining
	builder := NewSimulationRequestBuilder()

	result := builder.
		WithEnvelopeXDR("envelope").
		WithResultMetaXDR("result").
		WithLedgerEntry("key1", "value1").
		WithLedgerEntries(map[string]string{"key2": "value2"}).
		Reset().
		WithEnvelopeXDR("envelope2").
		WithResultMetaXDR("result2")

	if result != builder {
		t.Error("expected method chaining to return the same builder instance")
	}
}

func TestSimulationRequestBuilder_NilLedgerEntries(t *testing.T) {
	builder := NewSimulationRequestBuilder()

	req, err := builder.
		WithEnvelopeXDR("envelope").
		WithResultMetaXDR("result").
		WithLedgerEntries(nil).
		Build()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if req.LedgerEntries != nil {
		t.Errorf("expected LedgerEntries to be nil, got: %v", req.LedgerEntries)
	}
}
