// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package decoder

import "fmt"

// ExampleSuggestionEngine demonstrates how to use the suggestion engine
func ExampleSuggestionEngine() {
	// Create a new suggestion engine
	engine := NewSuggestionEngine()

	// Example 1: Analyze events from a failed transaction
	events := []DecodedEvent{
		{
			ContractID: "abc123def456",
			Topics:     []string{"storage_empty", "error"},
			Data:       "ScvVoid",
		},
	}

	suggestions := engine.AnalyzeEvents(events)
	fmt.Println("Suggestions found:", len(suggestions))
	for _, s := range suggestions {
		fmt.Printf("- %s (Confidence: %s)\n", s.Description, s.Confidence)
	}

	// Example 2: Analyze a call tree
	callTree := &CallNode{
		ContractID: "ROOT",
		Function:   "TOP_LEVEL",
		Events: []DecodedEvent{
			{
				ContractID: "contract1",
				Topics:     []string{"auth_failed"},
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
						Topics:     []string{"insufficient_balance"},
						Data:       "ScvVoid",
					},
				},
			},
		},
	}

	suggestions = engine.AnalyzeCallTree(callTree)
	output := FormatSuggestions(suggestions)
	fmt.Println(output)

	// Example 3: Add a custom rule
	customRule := ErrorPattern{
		Name:     "custom_timeout",
		Keywords: []string{"timeout", "deadline"},
		Suggestion: Suggestion{
			Rule:        "custom_timeout",
			Description: "Potential Fix: Increase the transaction timeout or optimize contract execution time.",
			Confidence:  "medium",
		},
	}
	engine.AddCustomRule(customRule)
}
