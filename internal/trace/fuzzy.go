// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import "strings"

// FuzzyMatch performs fuzzy matching and returns a score (higher is better)
// Returns -1 if no match, otherwise returns a score based on match quality
func FuzzyMatch(pattern, text string, caseSensitive bool) (score int, positions []int) {
	if pattern == "" {
		return -1, nil
	}

	searchPattern := pattern
	searchText := text

	if !caseSensitive {
		searchPattern = strings.ToLower(pattern)
		searchText = strings.ToLower(text)
	}

	positions = make([]int, 0, len(searchPattern))
	patternIdx := 0
	consecutiveBonus := 0

	for textIdx := 0; textIdx < len(searchText) && patternIdx < len(searchPattern); textIdx++ {
		if searchText[textIdx] == searchPattern[patternIdx] {
			positions = append(positions, textIdx)
			score += 1

			// Bonus for consecutive matches
			if len(positions) > 1 && positions[len(positions)-1] == positions[len(positions)-2]+1 {
				consecutiveBonus++
				score += consecutiveBonus
			} else {
				consecutiveBonus = 0
			}

			// Bonus for match at start
			if textIdx == 0 {
				score += 10
			}

			patternIdx++
		}
	}

	// No match if pattern not fully found
	if patternIdx < len(searchPattern) {
		return -1, nil
	}

	// Penalty for gaps
	if len(positions) > 0 {
		span := positions[len(positions)-1] - positions[0] + 1
		score -= (span - len(positions))
	}

	return score, positions
}
