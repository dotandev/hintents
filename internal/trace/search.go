// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

// SearchEngine handles searching through trace nodes
type SearchEngine struct {
	query         string
	caseSensitive bool
	matches       []TraceNodeMatch
	currentIndex  int
}

// TraceNodeMatch represents a search match in the trace
type TraceNodeMatch struct {
	NodeID      string       // Unique identifier for the node
	NodeIndex   int          // Position in flat trace list
	MatchRanges []MatchRange // Multiple matches within same node
	NodeData    *TraceNode   // Reference to actual node
}

// MatchRange represents a substring match
type MatchRange struct {
	Start int    // Start position of match
	End   int    // End position of match
	Field string // Which field matched: "contractID", "function", "error", "event"
}

// NewSearchEngine creates a new search engine
func NewSearchEngine() *SearchEngine {
	return &SearchEngine{
		caseSensitive: false, // Default: case-insensitive
		currentIndex:  -1,
	}
}

// SetQuery sets the search query and resets matches
func (s *SearchEngine) SetQuery(query string) {
	s.query = query
	s.currentIndex = -1
	s.matches = nil
}

// GetQuery returns the current search query
func (s *SearchEngine) GetQuery() string {
	return s.query
}

// Search performs the search across all trace nodes
func (s *SearchEngine) Search(nodes []*TraceNode) []TraceNodeMatch {
	if s.query == "" {
		return nil
	}

	s.matches = []TraceNodeMatch{}

	for i, node := range nodes {
		match := s.searchNode(node, i)
		if len(match.MatchRanges) > 0 {
			s.matches = append(s.matches, match)
		}
	}

	if len(s.matches) > 0 {
		s.currentIndex = 0
	}

	return s.matches
}

// searchNode searches within a single trace node
func (s *SearchEngine) searchNode(node *TraceNode, index int) TraceNodeMatch {
	match := TraceNodeMatch{
		NodeID:      node.ID,
		NodeIndex:   index,
		NodeData:    node,
		MatchRanges: []MatchRange{},
	}

	// Search in contract ID
	if ranges := s.findInString(node.ContractID, "contractID"); len(ranges) > 0 {
		match.MatchRanges = append(match.MatchRanges, ranges...)
	}

	// Search in function name
	if ranges := s.findInString(node.Function, "function"); len(ranges) > 0 {
		match.MatchRanges = append(match.MatchRanges, ranges...)
	}

	// Search in error message
	if ranges := s.findInString(node.Error, "error"); len(ranges) > 0 {
		match.MatchRanges = append(match.MatchRanges, ranges...)
	}

	// Search in event data
	if ranges := s.findInString(node.EventData, "event"); len(ranges) > 0 {
		match.MatchRanges = append(match.MatchRanges, ranges...)
	}

	// Search in type
	if ranges := s.findInString(node.Type, "type"); len(ranges) > 0 {
		match.MatchRanges = append(match.MatchRanges, ranges...)
	}

	return match
}

// findInString finds all occurrences of query in the given string using fuzzy matching
func (s *SearchEngine) findInString(text, field string) []MatchRange {
	if text == "" {
		return nil
	}

	score, positions := FuzzyMatch(s.query, text, s.caseSensitive)
	if score == -1 {
		return nil
	}

	if len(positions) == 0 {
		return nil
	}

	return []MatchRange{{
		Start: positions[0],
		End:   positions[len(positions)-1] + 1,
		Field: field,
	}}
}

// NextMatch moves to the next search match
func (s *SearchEngine) NextMatch() *TraceNodeMatch {
	if len(s.matches) == 0 {
		return nil
	}

	s.currentIndex = (s.currentIndex + 1) % len(s.matches)
	return &s.matches[s.currentIndex]
}

// PreviousMatch moves to the previous search match
func (s *SearchEngine) PreviousMatch() *TraceNodeMatch {
	if len(s.matches) == 0 {
		return nil
	}

	s.currentIndex--
	if s.currentIndex < 0 {
		s.currentIndex = len(s.matches) - 1
	}

	return &s.matches[s.currentIndex]
}

// CurrentMatch returns the current match
func (s *SearchEngine) CurrentMatch() *TraceNodeMatch {
	if len(s.matches) == 0 || s.currentIndex < 0 {
		return nil
	}
	return &s.matches[s.currentIndex]
}

// MatchCount returns total number of matches
func (s *SearchEngine) MatchCount() int {
	return len(s.matches)
}

// CurrentMatchNumber returns 1-based index of current match
func (s *SearchEngine) CurrentMatchNumber() int {
	if s.currentIndex < 0 {
		return 0
	}
	return s.currentIndex + 1
}

// ToggleCaseSensitive toggles case sensitivity and re-searches
func (s *SearchEngine) ToggleCaseSensitive(nodes []*TraceNode) {
	s.caseSensitive = !s.caseSensitive
	if s.query != "" {
		s.Search(nodes)
	}
}

// IsCaseSensitive returns whether search is case-sensitive
func (s *SearchEngine) IsCaseSensitive() bool {
	return s.caseSensitive
}

// HighlightMatches returns match ranges for a specific field in a node
func (s *SearchEngine) HighlightMatches(node *TraceNode, field string) []MatchRange {
	if s.query == "" {
		return nil
	}

	var text string
	switch field {
	case "contractID":
		text = node.ContractID
	case "function":
		text = node.Function
	case "error":
		text = node.Error
	case "event":
		text = node.EventData
	case "type":
		text = node.Type
	default:
		return nil
	}

	return s.findInString(text, field)
}
