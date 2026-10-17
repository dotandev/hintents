// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package decoder

import (
	"strings"
	"testing"
)

func TestNewSuggestionEngine(t *testing.T) {
	engine := NewSuggestionEngine()
	if engine == nil {
		t.Fatal("Expected non-nil engine")
	}
	if len(engine.rules) == 0 {
		t.Error("Expected default rules to be loaded")
	}
}

func TestAnalyzeEvents_UninitializedContract(t *testing.T) {
	engine := NewSuggestionEngine()

	events := []DecodedEvent{
		{
			ContractID: "abc123",
			Topics:     []string{"storage_empty", "error"},
			Data:       "ScvVoid",
		},
	}

	suggestions := engine.AnalyzeEvents(events)

	if len(suggestions) == 0 {
		t.Fatal("Expected at least one suggestion")
	}

	found := false
	for _, s := range suggestions {
		if s.Rule == "uninitialized_contract" {
			found = true
			if !strings.Contains(s.Description, "initialize()") {
				t.Errorf("Expected suggestion to mention initialize(), got: %s", s.Description)
			}
		}
	}

	if !found {
		t.Error("Expected uninitialized_contract suggestion")
	}
}

func TestAnalyzeEvents_MissingAuthorization(t *testing.T) {
	engine := NewSuggestionEngine()

	events := []DecodedEvent{
		{
			ContractID: "abc123",
			Topics:     []string{"auth_failed", "unauthorized"},
			Data:       "ScvVoid",
		},
	}

	suggestions := engine.AnalyzeEvents(events)

	if len(suggestions) == 0 {
		t.Fatal("Expected at least one suggestion")
	}

	found := false
	for _, s := range suggestions {
		if s.Rule == "missing_authorization" {
			found = true
			if !strings.Contains(s.Description, "authorization") {
				t.Errorf("Expected suggestion to mention authorization, got: %s", s.Description)
			}
		}
	}

	if !found {
		t.Error("Expected missing_authorization suggestion")
	}
}

func TestAnalyzeEvents_InsufficientBalance(t *testing.T) {
	engine := NewSuggestionEngine()

	events := []DecodedEvent{
		{
			ContractID: "abc123",
			Topics:     []string{"insufficient_balance", "error"},
			Data:       "ScvVoid",
		},
	}

	suggestions := engine.AnalyzeEvents(events)

	if len(suggestions) == 0 {
		t.Fatal("Expected at least one suggestion")
	}

	found := false
	for _, s := range suggestions {
		if s.Rule == "insufficient_balance" {
			found = true
			if !strings.Contains(s.Description, "balance") {
				t.Errorf("Expected suggestion to mention balance, got: %s", s.Description)
			}
		}
	}

	if !found {
		t.Error("Expected insufficient_balance suggestion")
	}
}

func TestAnalyzeEvents_InvalidParameters(t *testing.T) {
	engine := NewSuggestionEngine()

	events := []DecodedEvent{
		{
			ContractID: "abc123",
			Topics:     []string{"invalid_parameter", "error"},
			Data:       "ScvVoid",
		},
	}

	suggestions := engine.AnalyzeEvents(events)

	if len(suggestions) == 0 {
		t.Fatal("Expected at least one suggestion")
	}

	found := false
	for _, s := range suggestions {
		if s.Rule == "invalid_parameters" {
			found = true
			if s.Confidence != confidenceHigh {
				t.Errorf("Expected high confidence for a highly specific match, got: %s", s.Confidence)
			}
		}
	}

	if !found {
		t.Error("Expected invalid_parameters suggestion")
	}
}

func TestAnalyzeEvents_ContractNotFound(t *testing.T) {
	engine := NewSuggestionEngine()

	events := []DecodedEvent{
		{
			ContractID: "0000000000000000000000000000000000000000000000000000000000000000",
			Topics:     []string{"error"},
			Data:       "ScvVoid",
		},
	}

	suggestions := engine.AnalyzeEvents(events)

	if len(suggestions) == 0 {
		t.Fatal("Expected at least one suggestion")
	}

	found := false
	for _, s := range suggestions {
		if s.Rule == "contract_not_found" {
			found = true
			if !strings.Contains(s.Description, "contract ID") {
				t.Errorf("Expected suggestion to mention contract ID, got: %s", s.Description)
			}
		}
	}

	if !found {
		t.Error("Expected contract_not_found suggestion")
	}
}

func TestAnalyzeEvents_ResourceLimitExceeded(t *testing.T) {
	engine := NewSuggestionEngine()

	events := []DecodedEvent{
		{
			ContractID: "abc123",
			Topics:     []string{"limit_exceeded", "budget"},
			Data:       "ScvVoid",
		},
	}

	suggestions := engine.AnalyzeEvents(events)

	if len(suggestions) == 0 {
		t.Fatal("Expected at least one suggestion")
	}

	found := false
	for _, s := range suggestions {
		if s.Rule == "resource_limit_exceeded" {
			found = true
			if !strings.Contains(s.Description, "resource") {
				t.Errorf("Expected suggestion to mention resource, got: %s", s.Description)
			}
		}
	}

	if !found {
		t.Error("Expected resource_limit_exceeded suggestion")
	}
}

func TestAnalyzeEvents_NoMatch(t *testing.T) {
	engine := NewSuggestionEngine()

	events := []DecodedEvent{
		{
			ContractID: "abc123",
			Topics:     []string{"success", "completed"},
			Data:       "ScvVoid",
		},
	}

	suggestions := engine.AnalyzeEvents(events)

	if len(suggestions) != 0 {
		t.Errorf("Expected no suggestions for success events, got %d", len(suggestions))
	}
}

func TestAnalyzeCallTree(t *testing.T) {
	engine := NewSuggestionEngine()

	root := &CallNode{
		ContractID: "ROOT",
		Function:   "TOP_LEVEL",
		Events: []DecodedEvent{
			{
				ContractID: "abc123",
				Topics:     []string{"storage_empty"},
				Data:       "ScvVoid",
			},
		},
		SubCalls: []*CallNode{
			{
				ContractID: "child1",
				Function:   "transfer",
				Events: []DecodedEvent{
					{
						ContractID: "child1",
						Topics:     []string{"insufficient_balance"},
						Data:       "ScvVoid",
					},
				},
			},
		},
	}

	suggestions := engine.AnalyzeCallTree(root)

	if len(suggestions) < 2 {
		t.Errorf("Expected at least 2 suggestions (one from root, one from child), got %d", len(suggestions))
	}
}

func TestFormatSuggestions(t *testing.T) {
	suggestions := []Suggestion{
		{
			Rule:        "test_rule",
			Description: "Test suggestion",
			Confidence:  confidenceHigh,
		},
	}

	output := FormatSuggestions(suggestions)

	if !strings.Contains(output, "Potential Fixes") {
		t.Error("Expected output to contain 'Potential Fixes'")
	}
	if !strings.Contains(output, "Test suggestion") {
		t.Error("Expected output to contain suggestion description")
	}
	if !strings.Contains(output, "Confidence: High") {
		t.Error("Expected output to contain title-cased confidence level")
	}
	if !strings.Contains(output, "🟢") {
		t.Error("Expected output to contain a green confidence indicator for high confidence")
	}
}

func TestAnalyzeEvents_ConfidenceBasedOnSpecificity(t *testing.T) {
	engine := NewSuggestionEngine()
	engine.AddCustomRule(ErrorPattern{
		Name:     "specificity_probe",
		Keywords: []string{"timeout", "deadline"},
		EventChecks: []func(DecodedEvent) bool{
			func(event DecodedEvent) bool {
				for _, topic := range event.Topics {
					if strings.EqualFold(topic, "timeout_signal") {
						return true
					}
				}
				return false
			},
		},
		Suggestion: Suggestion{
			Rule:        "specificity_probe",
			Description: "Potential Fix: Review timeout configuration.",
			Confidence:  confidenceLow,
		},
	})

	tests := []struct {
		name       string
		events     []DecodedEvent
		confidence string
	}{
		{
			name: "low specificity keyword match",
			events: []DecodedEvent{{
				ContractID: "abc123",
				Topics:     []string{"warning"},
				Data:       "timeout while waiting",
			}},
			confidence: confidenceLow,
		},
		{
			name: "medium specificity multiple keywords",
			events: []DecodedEvent{{
				ContractID: "abc123",
				Topics:     []string{"warning", "deadline-near"},
				Data:       "timeout while waiting",
			}},
			confidence: confidenceMedium,
		},
		{
			name: "high specificity event check and exact keyword",
			events: []DecodedEvent{{
				ContractID: "abc123",
				Topics:     []string{"timeout_signal", "deadline"},
				Data:       "timeout while waiting",
			}},
			confidence: confidenceHigh,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			suggestions := engine.AnalyzeEvents(test.events)
			if len(suggestions) != 1 {
				t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
			}
			if suggestions[0].Confidence != test.confidence {
				t.Fatalf("expected %s confidence, got %s", test.confidence, suggestions[0].Confidence)
			}
		})
	}
}

func TestFormatSuggestions_Empty(t *testing.T) {
	suggestions := []Suggestion{}
	output := FormatSuggestions(suggestions)

	if output != "" {
		t.Errorf("Expected empty output for no suggestions, got: %s", output)
	}
}

func TestAddCustomRule(t *testing.T) {
	engine := NewSuggestionEngine()
	initialRuleCount := len(engine.rules)

	customRule := ErrorPattern{
		Name:     "custom_error",
		Keywords: []string{"custom"},
		Suggestion: Suggestion{
			Rule:        "custom_error",
			Description: "Custom error suggestion",
			Confidence:  "low",
		},
	}

	engine.AddCustomRule(customRule)

	if len(engine.rules) != initialRuleCount+1 {
		t.Errorf("Expected %d rules, got %d", initialRuleCount+1, len(engine.rules))
	}

	// Test that custom rule works
	events := []DecodedEvent{
		{
			ContractID: "abc123",
			Topics:     []string{"custom_error"},
			Data:       "ScvVoid",
		},
	}

	suggestions := engine.AnalyzeEvents(events)

	found := false
	for _, s := range suggestions {
		if s.Rule == "custom_error" {
			found = true
		}
	}

	if !found {
		t.Error("Expected custom rule to match")
	}
}

func TestAnalyzeEvents_DuplicateRules(t *testing.T) {
	engine := NewSuggestionEngine()

	// Create events that match the same rule multiple times
	events := []DecodedEvent{
		{
			ContractID: "abc123",
			Topics:     []string{"storage_empty"},
			Data:       "ScvVoid",
		},
		{
			ContractID: "abc123",
			Topics:     []string{"empty", "not found"},
			Data:       "ScvVoid",
		},
	}

	suggestions := engine.AnalyzeEvents(events)

	// Count how many times uninitialized_contract appears
	count := 0
	for _, s := range suggestions {
		if s.Rule == "uninitialized_contract" {
			count++
		}
	}

	if count > 1 {
		t.Errorf("Expected rule to appear only once, got %d times", count)
	}
}

func TestAnalyzeEvents_StructuredMatch_RewordedHostError(t *testing.T) {
	engine := NewSuggestionEngine()

	// Text message completely reworded (no 'auth' or 'permission' in topics/data)
	code := 5
	event := DecodedEvent{
		ContractID: "C123",
		Topics:     []string{"trap", "host_reworded_message_v2"},
		Data:       "some arbitrary host string without keywords",
		ErrorType:  "AuthError",
		ErrorCode:  &code,
	}

	suggestions := engine.AnalyzeEvents([]DecodedEvent{event})
	if len(suggestions) == 0 {
		t.Fatal("Expected suggestion to fire via structured match despite reworded text")
	}

	found := false
	for _, s := range suggestions {
		if s.Rule == "missing_authorization" {
			found = true
			if s.Confidence != confidenceHigh {
				t.Errorf("Expected confidence high, got: %s", s.Confidence)
			}
		}
	}
	if !found {
		t.Errorf("Expected missing_authorization rule to match structured AuthError")
	}
}

func TestAnalyzeEvents_StructuredMatch_StorageError(t *testing.T) {
	engine := NewSuggestionEngine()

	code := 3
	event := DecodedEvent{
		ContractID: "C123",
		Topics:     []string{"opaque_failure"},
		Data:       "unknown internal trap",
		ErrorType:  "StorageError",
		ErrorCode:  &code,
	}

	suggestions := engine.AnalyzeEvents([]DecodedEvent{event})
	found := false
	for _, s := range suggestions {
		if s.Rule == "uninitialized_contract" {
			found = true
			if !strings.Contains(s.Description, "initialize()") {
				t.Errorf("Expected description to mention initialize(), got: %s", s.Description)
			}
		}
	}
	if !found {
		t.Error("Expected uninitialized_contract to match structured StorageError")
	}
}

func TestAnalyzeEvents_FallbackToStringPattern(t *testing.T) {
	engine := NewSuggestionEngine()

	// No structured error fields provided; relies purely on keyword string fallback
	event := DecodedEvent{
		ContractID: "C123",
		Topics:     []string{"budget_exceeded"},
		Data:       "quota limit reached",
	}

	suggestions := engine.AnalyzeEvents([]DecodedEvent{event})
	found := false
	for _, s := range suggestions {
		if s.Rule == "resource_limit_exceeded" {
			found = true
		}
	}
	if !found {
		t.Error("Expected resource_limit_exceeded to match via keyword fallback")
	}
}

func TestAnalyzeEvents_CustomStructuredRule_FunctionAndArgs(t *testing.T) {
	engine := NewSuggestionEngine()

	engine.AddCustomRule(ErrorPattern{
		Name: "custom_transfer_type_error",
		StructuredMatch: &StructuredCriteria{
			ContractFunction: "transfer",
			ArgTypes:         []string{"ScvAddress", "ScvI128"},
		},
		Suggestion: Suggestion{
			Rule:        "custom_transfer_type_error",
			Description: "Verify transfer amount is positive and recipient address is valid",
			Confidence:  confidenceHigh,
		},
	})

	event := DecodedEvent{
		ContractID:       "CTOKEN",
		ContractFunction: "transfer",
		ArgTypes:         []string{"ScvAddress", "ScvI128"},
	}

	suggestions := engine.AnalyzeEvents([]DecodedEvent{event})
	found := false
	for _, s := range suggestions {
		if s.Rule == "custom_transfer_type_error" {
			found = true
		}
	}
	if !found {
		t.Error("Expected custom_transfer_type_error to match structured function and arguments")
	}
}
