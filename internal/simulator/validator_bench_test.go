// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/base64"
	"testing"
)

func BenchmarkValidateRequest(b *testing.B) {
	validXDR := base64.StdEncoding.EncodeToString([]byte("valid xdr data"))
	validator := NewValidator(false)
	
	req := &SimulationRequest{
		EnvelopeXdr:     validXDR,
		ResultMetaXdr:   validXDR,
		ProtocolVersion: uint32Ptr(21),
		Timestamp:       1735689600,
		LedgerSequence:  12345,
		LedgerEntries: map[string]string{
			validXDR: validXDR,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateRequest(req)
	}
}

func BenchmarkValidateRequestStrictMode(b *testing.B) {
	validXDR := base64.StdEncoding.EncodeToString([]byte("valid xdr data"))
	validator := NewValidator(true)
	
	req := &SimulationRequest{
		EnvelopeXdr:     validXDR,
		ResultMetaXdr:   validXDR,
		ProtocolVersion: uint32Ptr(21),
		Timestamp:       1735689600,
		LedgerSequence:  12345,
		LedgerEntries: map[string]string{
			validXDR: validXDR,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateRequest(req)
	}
}

func BenchmarkValidateRequestWithLargeEntries(b *testing.B) {
	validXDR := base64.StdEncoding.EncodeToString([]byte("valid xdr data"))
	validator := NewValidator(false)
	
	entries := make(map[string]string, 1000)
	for i := 0; i < 1000; i++ {
		key := base64.StdEncoding.EncodeToString([]byte{byte(i % 256), byte(i / 256)})
		entries[key] = validXDR
	}
	
	req := &SimulationRequest{
		EnvelopeXdr:   validXDR,
		ResultMetaXdr: validXDR,
		LedgerEntries: entries,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateRequest(req)
	}
}

func BenchmarkValidateResponse(b *testing.B) {
	validator := NewValidator(false)
	
	resp := &SimulationResponse{
		Status: "success",
		BudgetUsage: &BudgetUsage{
			CPUInstructions:    1000,
			MemoryBytes:        2000,
			CPUUsagePercent:    20.0,
			MemoryUsagePercent: 20.0,
		},
		DiagnosticEvents: []DiagnosticEvent{
			{
				EventType: "contract",
				Topics:    []string{"topic1", "topic2"},
				Data:      "data1",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateResponse(resp)
	}
}

func BenchmarkValidateContractID(b *testing.B) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateContractID(contractID)
	}
}

func BenchmarkValidateBase64(b *testing.B) {
	validXDR := base64.StdEncoding.EncodeToString([]byte("valid xdr data"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isValidBase64(validXDR)
	}
}

func BenchmarkValidateRequestWithCustomValidator(b *testing.B) {
	validXDR := base64.StdEncoding.EncodeToString([]byte("valid xdr data"))
	validator := NewValidator(false).WithCustomValidator("test", func(v interface{}) error {
		return nil
	})
	
	req := &SimulationRequest{
		EnvelopeXdr:   validXDR,
		ResultMetaXdr: validXDR,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateRequest(req)
	}
}
