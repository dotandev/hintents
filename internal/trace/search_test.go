// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSearchEngine(t *testing.T) {
	engine := NewSearchEngine()

	assert.NotNil(t, engine)
	assert.False(t, engine.IsCaseSensitive())
	assert.Equal(t, "", engine.GetQuery())
	assert.Equal(t, 0, engine.MatchCount())
}

func TestSearchEngine_CaseInsensitive(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{ID: "1", Error: "Error occurred"},
		{ID: "2", Error: "ERROR in contract"},
		{ID: "3", Error: "error message"},
	}

	engine.SetQuery("error")
	matches := engine.Search(nodes)

	assert.Equal(t, 3, len(matches))
	assert.Equal(t, 3, engine.MatchCount())
}

func TestSearchEngine_CaseSensitive(t *testing.T) {
	engine := NewSearchEngine()
	engine.ToggleCaseSensitive(nil)

	nodes := []*TraceNode{
		{ID: "1", Error: "Error occurred"},
		{ID: "2", Error: "ERROR in contract"},
		{ID: "3", Error: "error message"},
	}

	engine.SetQuery("error")
	matches := engine.Search(nodes)

	// Only lowercase "error" should match
	assert.Equal(t, 1, len(matches))
	assert.Equal(t, "3", matches[0].NodeID)
}

func TestSearchEngine_MultipleMatches(t *testing.T) {
	engine := NewSearchEngine()

	node := &TraceNode{
		ID:         "1",
		ContractID: "contract_error_123",
		Function:   "handle_error",
		Error:      "error occurred",
	}

	engine.SetQuery("error")
	matches := engine.Search([]*TraceNode{node})

	require.Equal(t, 1, len(matches))
	// Fuzzy matching finds error in multiple fields
	assert.GreaterOrEqual(t, len(matches[0].MatchRanges), 1)
}

func TestSearchEngine_NavigateMatches(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{ID: "1", Function: "test1"},
		{ID: "2", Function: "test2"},
		{ID: "3", Function: "test3"},
	}

	engine.SetQuery("test")
	engine.Search(nodes)

	// Should start at first match
	assert.Equal(t, 1, engine.CurrentMatchNumber())

	// Navigate forward
	match := engine.NextMatch()
	assert.Equal(t, "2", match.NodeID)
	assert.Equal(t, 2, engine.CurrentMatchNumber())

	match = engine.NextMatch()
	assert.Equal(t, "3", match.NodeID)
	assert.Equal(t, 3, engine.CurrentMatchNumber())

	// Should wrap around
	match = engine.NextMatch()
	assert.Equal(t, "1", match.NodeID)
	assert.Equal(t, 1, engine.CurrentMatchNumber())

	// Navigate backward
	match = engine.PreviousMatch()
	assert.Equal(t, "3", match.NodeID)
	assert.Equal(t, 3, engine.CurrentMatchNumber())
}

func TestSearchEngine_EmptyQuery(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{ID: "1", Function: "test"},
	}

	engine.SetQuery("")
	matches := engine.Search(nodes)

	assert.Nil(t, matches)
	assert.Equal(t, 0, engine.MatchCount())
	assert.Nil(t, engine.CurrentMatch())
}

func TestSearchEngine_NoMatches(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{ID: "1", Function: "test"},
	}

	engine.SetQuery("nonexistent")
	matches := engine.Search(nodes)

	assert.Equal(t, 0, len(matches))
	assert.Equal(t, 0, engine.MatchCount())
	assert.Nil(t, engine.CurrentMatch())
}

func TestSearchEngine_SpecialCharacters(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{ID: "1", ContractID: "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"},
		{ID: "2", ContractID: "CA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAXE"},
	}

	engine.SetQuery("CDLZFC3")
	matches := engine.Search(nodes)

	assert.Equal(t, 1, len(matches))
	assert.Equal(t, "1", matches[0].NodeID)
}

func TestSearchEngine_Unicode(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{ID: "1", Error: "Error: 文字化け occurred"},
		{ID: "2", Error: "Normal error"},
	}

	engine.SetQuery("文字化け")
	matches := engine.Search(nodes)

	assert.Equal(t, 1, len(matches))
	assert.Equal(t, "1", matches[0].NodeID)
}

func TestSearchEngine_SearchAllFields(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{ID: "1", ContractID: "match_here"},
		{ID: "2", Function: "match_here"},
		{ID: "3", Error: "match_here"},
		{ID: "4", EventData: "match_here"},
		{ID: "5", Type: "match_here"},
	}

	engine.SetQuery("match_here")
	matches := engine.Search(nodes)

	assert.Equal(t, 5, len(matches))
}

func TestSearchEngine_HighlightMatches(t *testing.T) {
	engine := NewSearchEngine()
	engine.SetQuery("error")

	node := &TraceNode{
		ID:    "1",
		Error: "error occurred",
	}

	ranges := engine.HighlightMatches(node, "error")

	require.Equal(t, 1, len(ranges))
	assert.Equal(t, 0, ranges[0].Start)
	assert.Equal(t, "error", ranges[0].Field)
}

func TestSearchEngine_HighlightMatches_NoQuery(t *testing.T) {
	engine := NewSearchEngine()

	node := &TraceNode{
		ID:    "1",
		Error: "error occurred",
	}

	ranges := engine.HighlightMatches(node, "error")

	assert.Nil(t, ranges)
}

func TestSearchEngine_ToggleCaseSensitive(t *testing.T) {
	engine := NewSearchEngine()

	assert.False(t, engine.IsCaseSensitive())

	engine.ToggleCaseSensitive(nil)
	assert.True(t, engine.IsCaseSensitive())

	engine.ToggleCaseSensitive(nil)
	assert.False(t, engine.IsCaseSensitive())
}

func TestSearchEngine_SetQueryResetsState(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{ID: "1", Function: "test"},
	}

	engine.SetQuery("test")
	engine.Search(nodes)

	assert.Equal(t, 1, engine.MatchCount())

	// Setting new query should reset matches
	engine.SetQuery("new")
	assert.Equal(t, 0, engine.MatchCount())
	assert.Nil(t, engine.CurrentMatch())
}

func TestSearchEngine_LongTrace(t *testing.T) {
	engine := NewSearchEngine()

	// Create 1000 nodes
	nodes := make([]*TraceNode, 1000)
	for i := 0; i < 1000; i++ {
		nodes[i] = &TraceNode{
			ID:       string(rune(i)),
			Function: "function_test",
		}
	}

	engine.SetQuery("test")
	matches := engine.Search(nodes)

	assert.Equal(t, 1000, len(matches))
	assert.Equal(t, 1000, engine.MatchCount())
}

func TestSearchEngine_PreviousMatch_EmptyMatches(t *testing.T) {
	engine := NewSearchEngine()

	match := engine.PreviousMatch()
	assert.Nil(t, match)
}

func TestSearchEngine_NextMatch_EmptyMatches(t *testing.T) {
	engine := NewSearchEngine()

	match := engine.NextMatch()
	assert.Nil(t, match)
}

func TestSearchEngine_CurrentMatchNumber_NoMatches(t *testing.T) {
	engine := NewSearchEngine()

	assert.Equal(t, 0, engine.CurrentMatchNumber())
}

func TestSearchEngine_PartialMatch(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{ID: "1", Function: "testing"},
		{ID: "2", Function: "test"},
		{ID: "3", Function: "contest"},
	}

	engine.SetQuery("test")
	matches := engine.Search(nodes)

	// Should match all three (testing, test, contest)
	assert.Equal(t, 3, len(matches))
}

func TestSearchEngine_CurrentMatch_WithMatches(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{ID: "1", Function: "test1"},
		{ID: "2", Function: "test2"},
	}

	engine.SetQuery("test")
	engine.Search(nodes)

	// Should have current match after search
	current := engine.CurrentMatch()
	assert.NotNil(t, current)
	assert.Equal(t, "1", current.NodeID)
}

func TestSearchEngine_ToggleCaseSensitive_WithQuery(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{ID: "1", Error: "Error occurred"},
		{ID: "2", Error: "error message"},
	}

	engine.SetQuery("error")
	engine.Search(nodes)

	// Should find 2 matches (case-insensitive)
	assert.Equal(t, 2, engine.MatchCount())

	// Toggle to case-sensitive
	engine.ToggleCaseSensitive(nodes)

	// Should find 1 match (only lowercase)
	assert.Equal(t, 1, engine.MatchCount())
}

func TestSearchEngine_HighlightMatches_AllFields(t *testing.T) {
	engine := NewSearchEngine()
	engine.SetQuery("test")

	node := &TraceNode{
		ID:         "1",
		ContractID: "test_contract",
		Function:   "test_function",
		Error:      "test_error",
		EventData:  "test_event",
		Type:       "test_type",
	}

	// Test each field
	fields := []string{"contractID", "function", "error", "event", "type"}
	for _, field := range fields {
		ranges := engine.HighlightMatches(node, field)
		assert.NotNil(t, ranges, "Field %s should have matches", field)
		assert.Greater(t, len(ranges), 0, "Field %s should have at least one match", field)
	}
}

func TestSearchEngine_HighlightMatches_InvalidField(t *testing.T) {
	engine := NewSearchEngine()
	engine.SetQuery("test")

	node := &TraceNode{
		ID:    "1",
		Error: "test error",
	}

	ranges := engine.HighlightMatches(node, "invalid_field")
	assert.Nil(t, ranges)
}

func TestSearchEngine_HighlightMatches_EmptyField(t *testing.T) {
	engine := NewSearchEngine()
	engine.SetQuery("test")

	node := &TraceNode{
		ID:    "1",
		Error: "", // Empty field
	}

	ranges := engine.HighlightMatches(node, "error")
	assert.Nil(t, ranges)
}

func TestSearchEngine_FuzzyMatching(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{ID: "1", Function: "test"},
		{ID: "2", Function: "testing"},
		{ID: "3", Function: "contest"},
	}

	// Fuzzy search for "tst" should match all three
	engine.SetQuery("tst")
	matches := engine.Search(nodes)

	assert.Equal(t, 3, len(matches))
}

func TestSearchEngine_FuzzyMatchingContractID(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{ID: "1", ContractID: "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"},
		{ID: "2", ContractID: "CA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAXE"},
	}

	// Fuzzy search for abbreviated contract ID
	engine.SetQuery("CDLZFC")
	matches := engine.Search(nodes)

	assert.Equal(t, 1, len(matches))
	assert.Equal(t, "1", matches[0].NodeID)
}

func TestSearchEngine_FuzzyMatchingError(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{ID: "1", Error: "insufficient balance"},
		{ID: "2", Error: "invalid signature"},
		{ID: "3", Error: "timeout error"},
	}

	// Fuzzy search for "isg" should match "invalid signature"
	engine.SetQuery("isg")
	matches := engine.Search(nodes)

	assert.Equal(t, 1, len(matches))
	assert.Equal(t, "2", matches[0].NodeID)
}
