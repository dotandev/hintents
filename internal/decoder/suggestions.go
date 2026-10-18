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
	structuredMatch bool
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

	// Structured matchers evaluated before the free-text fallback. Empty fields
	// are ignored. EventTypes and ErrorCodes are alternatives (a rule matches if
	// the event type or the error code is one of them), while Functions and
	// ArgumentTypes further narrow the match.
	EventTypes    []string // leading topic symbol, e.g. "fn_call", "storage_empty"
	ErrorCodes    []string // error code carried by an "error" topic, e.g. "budget_exceeded"
	Functions     []string // contract function name from fn_call/fn_return/log events
	ArgumentTypes []string // required xdr type names, e.g. "ScvError"

	Suggestion Suggestion
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
		Name:       "uninitialized_contract",
		Keywords:   []string{"empty", "not found", "missing", "null"},
		EventTypes: []string{"storage_empty", "not_initialized", "uninitialized_contract"},
		ErrorCodes: []string{"not_initialized", "uninitialized_contract"},
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
		Name:       "missing_authorization",
		Keywords:   []string{"auth", "unauthorized", "permission", "signature"},
		EventTypes: []string{"auth_failed", "unauthorized", "missing_authorization"},
		ErrorCodes: []string{"auth_failed", "unauthorized", "permission_denied"},
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
		Name:       "insufficient_balance",
		Keywords:   []string{"balance", "insufficient", "underfunded", "funds"},
		EventTypes: []string{"insufficient_balance", "underfunded"},
		ErrorCodes: []string{"insufficient_balance", "underfunded"},
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
		Name:       "invalid_parameters",
		Keywords:   []string{"invalid", "malformed", "bad", "parameter"},
		EventTypes: []string{"invalid_parameter", "invalid_parameters", "invalid_argument"},
		ErrorCodes: []string{"invalid_argument", "invalid_params"},
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
		Name:       "contract_not_found",
		Keywords:   []string{"not found", "missing contract", "no contract"},
		EventTypes: []string{"contract_not_found", "no_such_contract"},
		ErrorCodes: []string{"contract_not_found", "no_such_contract"},
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
		Name:       "resource_limit_exceeded",
		Keywords:   []string{"limit", "exceeded", "quota", "budget"},
		EventTypes: []string{"limit_exceeded", "budget_exceeded", "resource_limit_exceeded"},
		ErrorCodes: []string{"budget_exceeded", "resource_exceeded"},
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
		Name:       "reentrancy_detected",
		Keywords:   []string{"reentrant", "recursive", "loop"},
		EventTypes: []string{"reentrant", "reentrant_call", "recursive"},
		ErrorCodes: []string{"reentrant_call"},
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

	// Structured matching runs first: these fields come from the stable shape of
	// the decoded event, so a reworded host message cannot suppress a rule.
	if matchesStructured(rule, event.signature()) {
		match.structuredMatch = true
	}

	// Keyword/event-check matching is kept as a free-text fallback.
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

	return match, match.keywordMatches > 0 || match.eventCheckMatch || match.structuredMatch
}

// eventSignature is a structured, positional view of a DecodedEvent. Unlike the
// free-text fallback, it is derived from the event's stable shape (leading topic,
// function-name topic and xdr type names), so it survives host message rewording.
type eventSignature struct {
	EventType     string
	Function      string
	ErrorCode     string
	ArgumentTypes []string
}

// functionEvents are event types whose second topic holds the contract function.
var functionEvents = map[string]bool{
	"fn_call":   true,
	"fn_return": true,
	"log":       true,
}

// errorEvents are event types whose second topic holds the error code.
var errorEvents = map[string]bool{
	"error":            true,
	"contract_error":   true,
	"diagnostic_error": true,
	"host_error":       true,
}

func (e DecodedEvent) signature() eventSignature {
	sig := eventSignature{}

	if len(e.Topics) > 0 {
		sig.EventType = normalizeToken(e.Topics[0])
		if len(e.Topics) > 1 {
			switch {
			case functionEvents[sig.EventType]:
				sig.Function = strings.TrimSpace(e.Topics[1])
			case errorEvents[sig.EventType]:
				sig.ErrorCode = normalizeToken(e.Topics[1])
			}
		}
	}

	sig.ArgumentTypes = collectScValTypes(e)
	return sig
}

// collectScValTypes gathers the xdr type names exposed by the decoder, which
// renders non-symbol topics and the event data as their type name (for example
// "ScvAddress" or "ScvError").
func collectScValTypes(e DecodedEvent) []string {
	types := make([]string, 0, len(e.Topics)+1)
	add := func(raw string) {
		if name := scValTypeName(raw); name != "" {
			types = append(types, name)
		}
	}

	add(e.Data)
	for _, topic := range e.Topics {
		add(topic)
	}
	return types
}

// scValTypeName returns the xdr type name for a decoded ScVal (for example
// "ScvU32"), or an empty string when the token is not a type name.
func scValTypeName(raw string) string {
	token := strings.TrimSpace(raw)
	if strings.HasPrefix(token, "Scv") {
		return token
	}
	return ""
}

func normalizeToken(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// matchesStructured reports whether the rule's structured fields all match the
// event signature. EventTypes and ErrorCodes are alternatives; Functions and
// ArgumentTypes narrow the match.
func matchesStructured(rule ErrorPattern, sig eventSignature) bool {
	hasTypeOrCode := len(rule.EventTypes) > 0 || len(rule.ErrorCodes) > 0
	if !hasTypeOrCode && len(rule.Functions) == 0 && len(rule.ArgumentTypes) == 0 {
		return false
	}

	if hasTypeOrCode {
		matched := len(rule.EventTypes) > 0 && matchesAnyToken(rule.EventTypes, sig.EventType)
		if !matched && len(rule.ErrorCodes) > 0 {
			matched = matchesAnyToken(rule.ErrorCodes, sig.ErrorCode)
		}
		if !matched {
			return false
		}
	}

	if !matchesAnyToken(rule.Functions, sig.Function) {
		return false
	}

	return hasAllTokens(sig.ArgumentTypes, rule.ArgumentTypes)
}

// matchesAnyToken reports whether actual equals one of the allowed tokens.
// An empty allowed list is not a constraint and always matches.
func matchesAnyToken(allowed []string, actual string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, want := range allowed {
		if strings.EqualFold(strings.TrimSpace(want), actual) {
			return true
		}
	}
	return false
}

// hasAllTokens reports whether every required token is present in the haystack.
func hasAllTokens(haystack, required []string) bool {
	for _, want := range required {
		found := false
		for _, have := range haystack {
			if strings.EqualFold(want, have) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (m ruleMatch) specificityScore() int {
	score := 0
	if m.eventCheckMatch || m.structuredMatch {
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
