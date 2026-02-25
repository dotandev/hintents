// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package decoder

import (
	"strings"
	"testing"
)

// TestIntegration_SuggestionEngineWithDecoder tests the full flow
func TestIntegration_SuggestionEngineWithDecoder(t *testing.T) {
	// Simulate a scenario where we have XDR events from a failed transaction
	// In a real scenario, these would come from DecodeEvents()
	
	engine := NewSuggestionEngine()
	
	// Scenario 1: Contract not initialized
	t.Run("UninitializedContract", func(t *testing.T) {
		events := []DecodedEvent{
			{
				ContractID: "abc123",
				Topics:     []string{"fn_call", "transfer"},
				Data:       "ScvVoid",
			},
			{
				ContractID: "abc123",
				Topics:     []string{"storage_empty"},
				Data:       "ScvVoid",
			},
			{
				ContractID: "abc123",
				Topics:     []string{"fn_return", "transfer"},
				Data:       "ScvVoid",
			},
		}

		suggestions := engine.AnalyzeEvents(events)
		
		if len(suggestions) == 0 {
			t.Fatal("Expected suggestions for uninitialized contract")
		}

		found := false
		for _, s := range suggestions {
			if strings.Contains(s.Description, "initialize()") {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected suggestion to mention initialize()")
		}

		// Test formatting
		output := FormatSuggestions(suggestions)
		if !strings.Contains(output, "Potential Fixes") {
			t.Error("Expected formatted output to contain 'Potential Fixes'")
		}
	})

	// Scenario 2: Multiple errors in nested calls
	t.Run("NestedCallsWithMultipleErrors", func(t *testing.T) {
		callTree := &CallNode{
			ContractID: "ROOT",
			Function:   "TOP_LEVEL",
			Events: []DecodedEvent{
				{
					ContractID: "contract1",
					Topics:     []string{"fn_call", "execute"},
					Data:       "ScvVoid",
				},
			},
			SubCalls: []*CallNode{
				{
					ContractID: "contract2",
					Function:   "transfer",
					Events: []DecodedEvent{
						{
							ContractID: "contract2",
							Topics:     []string{"fn_call", "transfer"},
							Data:       "ScvVoid",
						},
						{
							ContractID: "contract2",
							Topics:     []string{"insufficient_balance"},
							Data:       "ScvVoid",
						},
					},
					SubCalls: []*CallNode{
						{
							ContractID: "contract3",
							Function:   "check_auth",
							Events: []DecodedEvent{
								{
									ContractID: "contract3",
									Topics:     []string{"unauthorized"},
									Data:       "ScvVoid",
								},
							},
						},
					},
				},
			},
		}

		suggestions := engine.AnalyzeCallTree(callTree)
		
		if len(suggestions) < 2 {
			t.Errorf("Expected at least 2 suggestions, got %d", len(suggestions))
		}

		// Should have both balance and auth suggestions
		hasBalance := false
		hasAuth := false
		for _, s := range suggestions {
			if strings.Contains(s.Description, "balance") {
				hasBalance = true
			}
			if strings.Contains(s.Description, "authorization") {
				hasAuth = true
			}
		}

		if !hasBalance {
			t.Error("Expected balance-related suggestion")
		}
		if !hasAuth {
			t.Error("Expected authorization-related suggestion")
		}
	})

	// Scenario 3: No errors (success case)
	t.Run("SuccessfulTransaction", func(t *testing.T) {
		events := []DecodedEvent{
			{
				ContractID: "abc123",
				Topics:     []string{"fn_call", "transfer"},
				Data:       "ScvVoid",
			},
			{
				ContractID: "abc123",
				Topics:     []string{"success"},
				Data:       "ScvVoid",
			},
			{
				ContractID: "abc123",
				Topics:     []string{"fn_return", "transfer"},
				Data:       "ScvVoid",
			},
		}

		suggestions := engine.AnalyzeEvents(events)
		
		if len(suggestions) != 0 {
			t.Errorf("Expected no suggestions for successful transaction, got %d", len(suggestions))
		}

		output := FormatSuggestions(suggestions)
		if output != "" {
			t.Error("Expected empty output for no suggestions")
		}
	})
}

// TestIntegration_CustomRuleWorkflow tests adding and using custom rules
func TestIntegration_CustomRuleWorkflow(t *testing.T) {
	engine := NewSuggestionEngine()

	// Add a custom rule for a project-specific error
	customRule := ErrorPattern{
		Name:     "rate_limit_exceeded",
		Keywords: []string{"rate_limit", "throttle", "too_many_requests"},
		EventChecks: []func(DecodedEvent) bool{
			func(e DecodedEvent) bool {
				for _, topic := range e.Topics {
					if strings.Contains(strings.ToLower(topic), "rate") {
						return true
					}
				}
				return false
			},
		},
		Suggestion: Suggestion{
			Rule:        "rate_limit_exceeded",
			Description: "Potential Fix: Wait before retrying or implement exponential backoff in your application.",
			Confidence:  "high",
		},
	}

	engine.AddCustomRule(customRule)

	// Test that the custom rule works
	events := []DecodedEvent{
		{
			ContractID: "abc123",
			Topics:     []string{"rate_limit_exceeded", "error"},
			Data:       "ScvVoid",
		},
	}

	suggestions := engine.AnalyzeEvents(events)
	
	if len(suggestions) == 0 {
		t.Fatal("Expected custom rule to trigger")
	}

	found := false
	for _, s := range suggestions {
		if s.Rule == "rate_limit_exceeded" {
			found = true
			if !strings.Contains(s.Description, "backoff") {
				t.Errorf("Expected custom suggestion text, got: %s", s.Description)
			}
		}
	}

	if !found {
		t.Error("Expected custom rule suggestion")
	}
}

// TestIntegration_RealWorldScenario simulates a realistic debugging session
func TestIntegration_RealWorldScenario(t *testing.T) {
	// Simulate a junior developer debugging a failed token transfer
	engine := NewSuggestionEngine()

	// The transaction failed because:
	// 1. Contract wasn't initialized
	// 2. User didn't have enough balance
	// 3. Authorization was missing
	
	callTree := &CallNode{
		ContractID: "token_contract",
		Function:   "transfer",
		Events: []DecodedEvent{
			{
				ContractID: "token_contract",
				Topics:     []string{"fn_call", "transfer"},
				Data:       "ScvVoid",
			},
			{
				ContractID: "token_contract",
				Topics:     []string{"storage_empty", "admin_not_set"},
				Data:       "ScvVoid",
			},
			{
				ContractID: "token_contract",
				Topics:     []string{"error", "not_initialized"},
				Data:       "ScvVoid",
			},
		},
	}

	suggestions := engine.AnalyzeCallTree(callTree)
	
	// Should get initialization suggestion
	if len(suggestions) == 0 {
		t.Fatal("Expected suggestions for failed transaction")
	}

	// Format for display
	output := FormatSuggestions(suggestions)
	
	// Verify output is helpful for junior developers
	if !strings.Contains(output, "Potential Fixes") {
		t.Error("Output should clearly mark suggestions")
	}
	if !strings.Contains(output, "Confidence") {
		t.Error("Output should show confidence levels")
	}
	if !strings.Contains(output, "initialize()") {
		t.Error("Should suggest calling initialize()")
	}

	t.Logf("Suggestions for junior developer:\n%s", output)
}
