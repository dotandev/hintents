// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/dotandev/hintents/internal/session"
)

const (
	sessionLookupListLimit = 200
)

func resourceNotFoundError(suggestion string) error {
	if suggestion != "" {
		return fmt.Errorf("Resource not found. Did you mean %s?", suggestion)
	}
	return fmt.Errorf("Resource not found.")
}

// resolvePartialID returns the matching candidate when input is a unique
// case-insensitive prefix. If zero or more than one candidate matches, it
// returns an empty string.
func resolvePartialID(input string, candidates []string) string {
	in := strings.ToLower(strings.TrimSpace(input))
	if in == "" {
		return ""
	}

	var match string
	for _, c := range candidates {
		if strings.HasPrefix(strings.ToLower(c), in) {
			if match != "" {
				return "" // ambiguous
			}
			match = c
		}
	}
	return match
}

// resolveByTxHash returns the session ID whose transaction hash starts with
// the given prefix. Returns empty when the prefix is ambiguous or absent.
func resolveByTxHash(input string, sessions []*session.SessionData) string {
	in := strings.ToLower(strings.TrimSpace(input))
	if in == "" {
		return ""
	}

	var match string
	for _, s := range sessions {
		if s == nil || s.TxHash == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(s.TxHash), in) {
			if match != "" {
				return "" // ambiguous
			}
			match = s.ID
		}
	}
	return match
}

// resolveSessionInput attempts to load a session by trying, in order:
//  1. Exact ID match
//  2. Unique prefix match against session IDs
//  3. Unique prefix match against transaction hashes
//  4. Fuzzy suggestion via Levenshtein distance
//
// It returns the resolved session or an error with a "Did you mean?" hint.
func resolveSessionInput(ctx context.Context, store *session.Store, input string) (*session.SessionData, error) {
	// 1. Exact ID
	data, err := store.Load(ctx, input)
	if err == nil {
		return data, nil
	}

	sessions, listErr := store.List(ctx, sessionLookupListLimit)
	if listErr != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", listErr)
	}

	ids := make([]string, 0, len(sessions))
	for _, s := range sessions {
		if s != nil && s.ID != "" {
			ids = append(ids, s.ID)
		}
	}

	// 2. Unique prefix match on IDs
	if match := resolvePartialID(input, ids); match != "" {
		resolved, loadErr := store.Load(ctx, match)
		if loadErr == nil {
			return resolved, nil
		}
	}

	// 3. Unique prefix match on tx hashes
	if match := resolveByTxHash(input, sessions); match != "" {
		resolved, loadErr := store.Load(ctx, match)
		if loadErr == nil {
			return resolved, nil
		}
	}

	// 4. Fuzzy suggestion (IDs + tx hashes)
	candidates := make([]string, 0, len(ids)*2)
	candidates = append(candidates, ids...)
	for _, s := range sessions {
		if s != nil && s.TxHash != "" {
			candidates = append(candidates, s.TxHash)
		}
	}
	suggestion := closestStringMatch(input, candidates)
	return nil, resourceNotFoundError(suggestion)
}

// suggestSessionID returns the closest session ID for a failed lookup.
// Kept for backward compatibility; new callers should use resolveSessionInput.
func suggestSessionID(ctx context.Context, store *session.Store, input string) (string, error) {
	sessions, err := store.List(ctx, sessionLookupListLimit)
	if err != nil {
		return "", err
	}

	candidates := make([]string, 0, len(sessions))
	for _, s := range sessions {
		if s == nil || s.ID == "" {
			continue
		}
		candidates = append(candidates, s.ID)
	}

	return closestStringMatch(input, candidates), nil
}

func closestStringMatch(input string, candidates []string) string {
	in := strings.ToLower(strings.TrimSpace(input))
	if in == "" || len(candidates) == 0 {
		return ""
	}

	bestCandidate := ""
	bestScore := math.Inf(-1)
	bestDistance := int(^uint(0) >> 1)

	for _, candidate := range candidates {
		c := strings.TrimSpace(candidate)
		if c == "" {
			continue
		}

		cn := strings.ToLower(c)
		if cn == in {
			return candidate
		}

		distance := levenshteinDistance(in, cn)
		maxLen := len(in)
		if len(cn) > maxLen {
			maxLen = len(cn)
		}
		if maxLen == 0 {
			continue
		}

		score := 1.0 - float64(distance)/float64(maxLen)

		// Prefix and containment bonuses make CLI IDs more forgiving.
		if strings.HasPrefix(cn, in) || strings.HasPrefix(in, cn) {
			score += 0.25
		}
		if strings.Contains(cn, in) || strings.Contains(in, cn) {
			score += 0.10
		}

		if score > bestScore || (score == bestScore && distance < bestDistance) {
			bestScore = score
			bestDistance = distance
			bestCandidate = candidate
		}
	}

	// Keep suggestions conservative to avoid noisy wrong hints.
	if bestCandidate == "" {
		return ""
	}
	if bestScore >= 0.55 || bestDistance <= 2 {
		return bestCandidate
	}
	return ""
}

func levenshteinDistance(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}

			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost

			curr[j] = del
			if ins < curr[j] {
				curr[j] = ins
			}
			if sub < curr[j] {
				curr[j] = sub
			}
		}
		prev, curr = curr, prev
	}

	return prev[len(b)]
}
