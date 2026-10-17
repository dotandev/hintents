// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestCheckFootprintConflicts_NoConflict(t *testing.T) {
	declared := Footprint{
		ReadOnly:  []string{"key_ro_1", "key_ro_2"},
		ReadWrite: []string{"key_rw_1", "key_rw_2"},
	}
	observed := ObservedAccess{
		Reads:  []string{"key_ro_1", "key_rw_1"},
		Writes: []string{"key_rw_1", "key_rw_2"},
	}

	conflicts := CheckFootprintConflicts(declared, observed)
	if len(conflicts) != 0 {
		t.Fatalf("expected 0 conflicts, got %d: %+v", len(conflicts), conflicts)
	}

	v := NewValidator(false)
	if err := v.ValidateFootprint(declared, observed); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestCheckFootprintConflicts_ReadOnlyKeyWritten(t *testing.T) {
	declared := Footprint{
		ReadOnly:  []string{"contract_data_immutable_key"},
		ReadWrite: []string{"contract_data_mutable_key"},
	}
	observed := ObservedAccess{
		Reads:  []string{"contract_data_immutable_key"},
		Writes: []string{"contract_data_immutable_key"}, // Writing to a read-only key!
	}

	conflicts := CheckFootprintConflicts(declared, observed)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}

	c := conflicts[0]
	if c.Key != "contract_data_immutable_key" {
		t.Errorf("expected conflicting key 'contract_data_immutable_key', got %s", c.Key)
	}
	if c.Conflict != "read_only_violation" {
		t.Errorf("expected conflict type 'read_only_violation', got %s", c.Conflict)
	}
	if !strings.Contains(c.Message, "declared as read-only") {
		t.Errorf("expected message to mention read-only, got: %s", c.Message)
	}

	v := NewValidator(false)
	err := v.ValidateFootprint(declared, observed)
	if err == nil {
		t.Fatal("expected ValidationError, got nil")
	}

	valErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if valErr.Code != "ERR_FOOTPRINT_CONFLICT" {
		t.Errorf("expected code ERR_FOOTPRINT_CONFLICT, got %s", valErr.Code)
	}
}

func TestCheckFootprintConflicts_DuplicateDeclaration(t *testing.T) {
	declared := Footprint{
		ReadOnly:  []string{"shared_key"},
		ReadWrite: []string{"shared_key"},
	}
	observed := ObservedAccess{
		Reads:  []string{"shared_key"},
		Writes: []string{"shared_key"},
	}

	conflicts := CheckFootprintConflicts(declared, observed)
	foundDuplicate := false
	for _, c := range conflicts {
		if c.Conflict == "duplicate_declaration" && c.Key == "shared_key" {
			foundDuplicate = true
			break
		}
	}

	if !foundDuplicate {
		t.Fatalf("expected duplicate_declaration conflict for shared_key, got: %+v", conflicts)
	}
}

func TestCheckFootprintConflicts_UndeclaredWrite(t *testing.T) {
	declared := Footprint{
		ReadOnly:  []string{"ro_key"},
		ReadWrite: []string{"rw_key"},
	}
	observed := ObservedAccess{
		Writes: []string{"unexpected_write_key"},
	}

	conflicts := CheckFootprintConflicts(declared, observed)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}

	c := conflicts[0]
	if c.Conflict != "undeclared_write" || c.Key != "unexpected_write_key" {
		t.Fatalf("unexpected conflict: %+v", c)
	}
}

func TestValidateRequest_Basic(t *testing.T) {
	v := NewValidator(false)
	validXDR := base64.StdEncoding.EncodeToString([]byte("valid-xdr-payload"))

	req := &SimulationRequest{
		EnvelopeXdr:   validXDR,
		ResultMetaXdr: validXDR,
	}

	if err := v.ValidateRequest(req); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	// Missing EnvelopeXdr
	reqEmpty := &SimulationRequest{
		EnvelopeXdr:   "",
		ResultMetaXdr: validXDR,
	}
	if err := v.ValidateRequest(reqEmpty); err == nil {
		t.Fatal("expected error for empty envelope_xdr")
	}
}
