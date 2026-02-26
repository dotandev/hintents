// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFuzzyMatch_ExactMatch(t *testing.T) {
	score, positions := FuzzyMatch("test", "test", false)
	assert.Greater(t, score, 0)
	assert.Equal(t, []int{0, 1, 2, 3}, positions)
}

func TestFuzzyMatch_SubstringMatch(t *testing.T) {
	score, positions := FuzzyMatch("test", "testing", false)
	assert.Greater(t, score, 0)
	assert.Equal(t, []int{0, 1, 2, 3}, positions)
}

func TestFuzzyMatch_FuzzyMatch(t *testing.T) {
	score, positions := FuzzyMatch("tst", "test", false)
	assert.Greater(t, score, 0)
	assert.Equal(t, []int{0, 2, 3}, positions)
}

func TestFuzzyMatch_NoMatch(t *testing.T) {
	score, positions := FuzzyMatch("xyz", "test", false)
	assert.Equal(t, -1, score)
	assert.Nil(t, positions)
}

func TestFuzzyMatch_CaseInsensitive(t *testing.T) {
	score, positions := FuzzyMatch("TEST", "testing", false)
	assert.Greater(t, score, 0)
	assert.Equal(t, []int{0, 1, 2, 3}, positions)
}

func TestFuzzyMatch_CaseSensitive(t *testing.T) {
	score, _ := FuzzyMatch("TEST", "testing", true)
	assert.Equal(t, -1, score)

	score, positions := FuzzyMatch("test", "testing", true)
	assert.Greater(t, score, 0)
	assert.Equal(t, []int{0, 1, 2, 3}, positions)
}

func TestFuzzyMatch_EmptyPattern(t *testing.T) {
	score, positions := FuzzyMatch("", "test", false)
	assert.Equal(t, -1, score)
	assert.Nil(t, positions)
}

func TestFuzzyMatch_ConsecutiveBonus(t *testing.T) {
	// Consecutive matches should score higher
	scoreConsecutive, _ := FuzzyMatch("test", "test", false)
	scoreGapped, _ := FuzzyMatch("test", "t_e_s_t", false)

	assert.Greater(t, scoreConsecutive, scoreGapped)
}

func TestFuzzyMatch_StartBonus(t *testing.T) {
	// Match at start should score higher
	scoreStart, _ := FuzzyMatch("test", "test_function", false)
	scoreMiddle, _ := FuzzyMatch("test", "my_test_function", false)

	assert.Greater(t, scoreStart, scoreMiddle)
}

func TestFuzzyMatch_ContractID(t *testing.T) {
	score, positions := FuzzyMatch("CDLZ", "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC", false)
	assert.Greater(t, score, 0)
	assert.Equal(t, []int{0, 1, 2, 3}, positions)
}

func TestFuzzyMatch_PartialOrder(t *testing.T) {
	score, positions := FuzzyMatch("err", "error_occurred", false)
	assert.Greater(t, score, 0)
	assert.Equal(t, []int{0, 1, 2}, positions)
}

func TestFuzzyMatch_OutOfOrder(t *testing.T) {
	// Pattern characters must be in order
	score, _ := FuzzyMatch("tse", "test", false)
	assert.Equal(t, -1, score)
}
