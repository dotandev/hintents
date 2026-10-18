// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package decoder

import (
	"fmt"
	"strings"
)

const (
	confidenceHigh   = "high"
	confidenceMedium = "medium"
	confidenceLow    = "low"
)

// Suggestion represents a potential fix for a Soroban error
type Suggestion struct {
	Rule        string
	Description string
	Confidence  string // Derived from match specificity: "high", "medium", "low"
}

type ruleMatch struct {
	keywordMatches  int
	exactKeywordHit bool
	eventCheckMatch bool
}

type scoredSuggestion struct {
	suggestion Suggestion
	score      int
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
			Confidence:  confidenceHigh,
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
			Confidence:  confidenceHigh,
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
			Confidence:  confidenceHigh,
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
			Confidence:  confidenceMedium,
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
			Confidence:  confidenceHigh,
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
			Confidence:  confidenceMedium,
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
			Confidence:  confidenceMedium,
		},
	})
}

// AnalyzeEvents analyzes decoded events and returns suggestions
func (e *SuggestionEngine) AnalyzeEvents(events []DecodedEvent) []Suggestion {
	ruleOrder := make([]string, 0, len(e.rules))
	bestMatches := make(map[string]scoredSuggestion, len(e.rules))

	for _, event := range events {
		for _, rule := range e.rules {
			match, matched := e.matchRule(rule, event)
			if !matched {
				continue
			}

			suggestion := rule.Suggestion
			suggestion.Confidence = confidenceFromMatch(match)
			score := match.specificityScore()

			existing, exists := bestMatches[rule.Name]
			if !exists {
				ruleOrder = append(ruleOrder, rule.Name)
				bestMatches[rule.Name] = scoredSuggestion{suggestion: suggestion, score: score}
				continue
			}

			if score > existing.score {
				bestMatches[rule.Name] = scoredSuggestion{suggestion: suggestion, score: score}
			}
		}
	}

	suggestions := make([]Suggestion, 0, len(ruleOrder))
	for _, ruleName := range ruleOrder {
		suggestions = append(suggestions, bestMatches[ruleName].suggestion)
	}

	return suggestions
}

func (e *SuggestionEngine) matchRule(rule ErrorPattern, event DecodedEvent) (ruleMatch, bool) {
	match := ruleMatch{}
	topics := make([]string, 0, len(event.Topics))
	for _, topic := range event.Topics {
		topics = append(topics, strings.ToLower(topic))
	}
	data := strings.ToLower(event.Data)

	for _, keyword := range rule.Keywords {
		lowerKeyword := strings.ToLower(keyword)
		for _, topic := range topics {
			if topic == lowerKeyword {
				match.exactKeywordHit = true
				match.keywordMatches++
				break
			}
			if strings.Contains(topic, lowerKeyword) {
				match.keywordMatches++
				break
			}
		}

		if strings.Contains(data, lowerKeyword) {
			match.keywordMatches++
		}
	}

	for _, check := range rule.EventChecks {
		if check(event) {
			match.eventCheckMatch = true
			break
		}
	}

	return match, match.keywordMatches > 0 || match.eventCheckMatch
}

func (m ruleMatch) specificityScore() int {
	score := 0
	if m.eventCheckMatch {
		score += 2
	}
	if m.keywordMatches > 0 {
		score++
	}
	if m.keywordMatches > 1 {
		score++
	}
	if m.exactKeywordHit {
		score++
	}
	return score
}

func confidenceFromMatch(match ruleMatch) string {
	score := match.specificityScore()
	switch {
	case score >= 4:
		return confidenceHigh
	case score >= 2:
		return confidenceMedium
	default:
		return confidenceLow
	}
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
	if node == nil {
		return nil
	}

	// Pre-allocate with estimated capacity to reduce re-allocations
	// Estimate: node.Events + 5 events per child call
	capacity := len(node.Events) + len(node.SubCalls)*5
	events := make([]DecodedEvent, 0, capacity)

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
		confidenceIcon := "⚪"
		switch suggestion.Confidence {
		case confidenceHigh:
			confidenceIcon = "🟢"
		case confidenceMedium:
			confidenceIcon = "🟡"
		case confidenceLow:
			confidenceIcon = "🔴"
		}

		conf := suggestion.Confidence
		if len(conf) > 0 {
			conf = strings.ToUpper(conf[:1]) + conf[1:]
		}
		fmt.Fprintf(&output, "%d. %s [Confidence: %s]\n", i+1, confidenceIcon, conf)
		fmt.Fprintf(&output, "   %s\n", suggestion.Description)
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
