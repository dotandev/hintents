// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package decoder

import (
	"fmt"
	"strings"
)

// Suggestion represents a potential fix for a Soroban error
type Suggestion struct {
	Rule        string
	Description string
	Confidence  string // "high", "medium", "low"
}

// ErrorPattern defines a heuristic rule for error detection
type ErrorPattern struct {
	Name        string
	Keywords    []string
	EventChecks []func(DecodedEvent) bool
	Suggestion  Suggestion
}

// SuggestionEngine provides heuristic-based error suggestions
type SuggestionEngine struct {
	rules []ErrorPattern
}

// NewSuggestionEngine creates a new suggestion engine with predefined rules
func NewSuggestionEngine() *SuggestionEngine {
	engine := &SuggestionEngine{
		rules: []ErrorPattern{},
	}
	engine.loadDefaultRules()
	return engine
}

// loadDefaultRules initializes the default heuristic rules
func (e *SuggestionEngine) loadDefaultRules() {
	// Rule 1: Uninitialized contract
	e.rules = append(e.rules, ErrorPattern{
		Name:     "uninitialized_contract",
		Keywords: []string{"empty", "not found", "missing", "null"},
		EventChecks: []func(DecodedEvent) bool{
			func(e DecodedEvent) bool {
				for _, topic := range e.Topics {
					lower := strings.ToLower(topic)
					if strings.Contains(lower, "storage") && strings.Contains(lower, "empty") {
						return true
					}
				}
				return false
			},
		},
		Suggestion: Suggestion{
			Rule:        "uninitialized_contract",
			Description: "Potential Fix: Ensure you have called initialize() on this contract before invoking other functions.",
			Confidence:  "high",
		},
	})

	// Rule 2: Insufficient authorization
	e.rules = append(e.rules, ErrorPattern{
		Name:     "missing_authorization",
		Keywords: []string{"auth", "unauthorized", "permission", "signature"},
		EventChecks: []func(DecodedEvent) bool{
			func(e DecodedEvent) bool {
				for _, topic := range e.Topics {
					lower := strings.ToLower(topic)
					if strings.Contains(lower, "auth") || strings.Contains(lower, "unauthorized") {
						return true
					}
				}
				return false
			},
		},
		Suggestion: Suggestion{
			Rule:        "missing_authorization",
			Description: "Potential Fix: Verify that all required signatures are present and the invoker has proper authorization.",
			Confidence:  "high",
		},
	})

	// Rule 3: Insufficient balance
	e.rules = append(e.rules, ErrorPattern{
		Name:     "insufficient_balance",
		Keywords: []string{"balance", "insufficient", "underfunded", "funds"},
		EventChecks: []func(DecodedEvent) bool{
			func(e DecodedEvent) bool {
				for _, topic := range e.Topics {
					lower := strings.ToLower(topic)
					if strings.Contains(lower, "balance") || strings.Contains(lower, "insufficient") {
						return true
					}
				}
				return false
			},
		},
		Suggestion: Suggestion{
			Rule:        "insufficient_balance",
			Description: "Potential Fix: Ensure the account has sufficient balance to cover the transaction and maintain minimum reserves.",
			Confidence:  "high",
		},
	})

	// Rule 4: Invalid parameters
	e.rules = append(e.rules, ErrorPattern{
		Name:     "invalid_parameters",
		Keywords: []string{"invalid", "malformed", "bad", "parameter"},
		EventChecks: []func(DecodedEvent) bool{
			func(e DecodedEvent) bool {
				for _, topic := range e.Topics {
					lower := strings.ToLower(topic)
					if strings.Contains(lower, "invalid") || strings.Contains(lower, "malformed") {
						return true
					}
				}
				return false
			},
		},
		Suggestion: Suggestion{
			Rule:        "invalid_parameters",
			Description: "Potential Fix: Check that all function parameters match the expected types and constraints.",
			Confidence:  "medium",
		},
	})

	// Rule 5: Contract not found
	e.rules = append(e.rules, ErrorPattern{
		Name:     "contract_not_found",
		Keywords: []string{"not found", "missing contract", "no contract"},
		EventChecks: []func(DecodedEvent) bool{
			func(e DecodedEvent) bool {
				return e.ContractID == "" || e.ContractID == "0000000000000000000000000000000000000000000000000000000000000000"
			},
		},
		Suggestion: Suggestion{
			Rule:        "contract_not_found",
			Description: "Potential Fix: Verify the contract ID is correct and the contract has been deployed to the network.",
			Confidence:  "high",
		},
	})

	// Rule 6: Exceeded resource limits
	e.rules = append(e.rules, ErrorPattern{
		Name:     "resource_limit_exceeded",
		Keywords: []string{"limit", "exceeded", "quota", "budget"},
		EventChecks: []func(DecodedEvent) bool{
			func(e DecodedEvent) bool {
				for _, topic := range e.Topics {
					lower := strings.ToLower(topic)
					if strings.Contains(lower, "limit") || strings.Contains(lower, "exceeded") {
						return true
					}
				}
				return false
			},
		},
		Suggestion: Suggestion{
			Rule:        "resource_limit_exceeded",
			Description: "Potential Fix: Optimize your contract code to reduce CPU/memory usage, or increase resource limits in the transaction.",
			Confidence:  "medium",
		},
	})

	// Rule 7: Reentrancy issue
	e.rules = append(e.rules, ErrorPattern{
		Name:     "reentrancy_detected",
		Keywords: []string{"reentrant", "recursive", "loop"},
		EventChecks: []func(DecodedEvent) bool{
			func(e DecodedEvent) bool {
				for _, topic := range e.Topics {
					lower := strings.ToLower(topic)
					if strings.Contains(lower, "reentrant") || strings.Contains(lower, "recursive") {
						return true
					}
				}
				return false
			},
		},
		Suggestion: Suggestion{
			Rule:        "reentrancy_detected",
			Description: "Potential Fix: Implement reentrancy guards or use the checks-effects-interactions pattern to prevent recursive calls.",
			Confidence:  "medium",
		},
	})
}

// AnalyzeEvents analyzes decoded events and returns suggestions
func (e *SuggestionEngine) AnalyzeEvents(events []DecodedEvent) []Suggestion {
	suggestions := []Suggestion{}
	seenRules := make(map[string]bool)

	for _, event := range events {
		for _, rule := range e.rules {
			// Skip if we already found this rule
			if seenRules[rule.Name] {
				continue
			}

			// Check keywords in topics and data
			keywordMatch := false
			for _, keyword := range rule.Keywords {
				for _, topic := range event.Topics {
					if strings.Contains(strings.ToLower(topic), strings.ToLower(keyword)) {
						keywordMatch = true
						break
					}
				}
				if keywordMatch {
					break
				}
				if strings.Contains(strings.ToLower(event.Data), strings.ToLower(keyword)) {
					keywordMatch = true
					break
				}
			}

			// Check event-specific conditions
			eventCheckMatch := false
			for _, check := range rule.EventChecks {
				if check(event) {
					eventCheckMatch = true
					break
				}
			}

			// If either keywords or event checks match, add suggestion
			if keywordMatch || eventCheckMatch {
				suggestions = append(suggestions, rule.Suggestion)
				seenRules[rule.Name] = true
			}
		}
	}

	return suggestions
}

// AnalyzeCallTree analyzes a call tree and returns suggestions
func (e *SuggestionEngine) AnalyzeCallTree(root *CallNode) []Suggestion {
	if root == nil {
		return []Suggestion{}
	}

	allEvents := e.collectEvents(root)
	return e.AnalyzeEvents(allEvents)
}

// collectEvents recursively collects all events from a call tree
func (e *SuggestionEngine) collectEvents(node *CallNode) []DecodedEvent {
	events := make([]DecodedEvent, 0)
	
	if node == nil {
		return events
	}

	events = append(events, node.Events...)
	
	for _, child := range node.SubCalls {
		events = append(events, e.collectEvents(child)...)
	}

	return events
}

// FormatSuggestions formats suggestions for display
func FormatSuggestions(suggestions []Suggestion) string {
	if len(suggestions) == 0 {
		return ""
	}

	var output strings.Builder
	output.WriteString("\n=== Potential Fixes (Heuristic Analysis) ===\n")
	output.WriteString("⚠️  These are suggestions based on common error patterns. Always verify before applying.\n\n")

	for i, suggestion := range suggestions {
		confidenceIcon := "●"
		switch suggestion.Confidence {
		case "high":
			confidenceIcon = "🔴"
		case "medium":
			confidenceIcon = "🟡"
		case "low":
			confidenceIcon = "🟢"
		}

		output.WriteString(fmt.Sprintf("%d. %s [Confidence: %s]\n", i+1, confidenceIcon, suggestion.Confidence))
		output.WriteString(fmt.Sprintf("   %s\n", suggestion.Description))
		if i < len(suggestions)-1 {
			output.WriteString("\n")
		}
	}

	return output.String()
}

// AddCustomRule allows adding custom heuristic rules
func (e *SuggestionEngine) AddCustomRule(pattern ErrorPattern) {
	e.rules = append(e.rules, pattern)
}
