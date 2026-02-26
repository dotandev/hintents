// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"testing"

	"github.com/dotandev/hintents/internal/session"
)

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{a: "", b: "", want: 0},
		{a: "kitten", b: "sitting", want: 3},
		{a: "abc123", b: "abc124", want: 1},
		{a: "session-1", b: "session-1", want: 0},
	}

	for _, tt := range tests {
		if got := levenshteinDistance(tt.a, tt.b); got != tt.want {
			t.Fatalf("levenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestClosestStringMatch(t *testing.T) {
	candidates := []string{
		"abc123-1700000000",
		"def456-1700001111",
		"session-prod-001",
	}

	if got := closestStringMatch("abc123-170000000", candidates); got != "abc123-1700000000" {
		t.Fatalf("closestStringMatch prefix failed: got %q", got)
	}

	if got := closestStringMatch("def457-1700001111", candidates); got != "def456-1700001111" {
		t.Fatalf("closestStringMatch typo failed: got %q", got)
	}

	if got := closestStringMatch("totally-unrelated-id", candidates); got != "" {
		t.Fatalf("closestStringMatch should be conservative, got %q", got)
	}
}

func TestResourceNotFoundError(t *testing.T) {
	if got := resourceNotFoundError("abc123").Error(); got != "Resource not found. Did you mean abc123?" {
		t.Fatalf("unexpected suggestion message: %q", got)
	}

	if got := resourceNotFoundError("").Error(); got != "Resource not found." {
		t.Fatalf("unexpected no-suggestion message: %q", got)
	}
}

func TestResolvePartialID(t *testing.T) {
	candidates := []string{
		"abc123-1700000000",
		"def456-1700001111",
		"abc999-1700002222",
	}

	// Unique prefix resolves to the single match.
	if got := resolvePartialID("def4", candidates); got != "def456-1700001111" {
		t.Fatalf("resolvePartialID unique prefix: got %q, want %q", got, "def456-1700001111")
	}

	// Ambiguous prefix returns empty.
	if got := resolvePartialID("abc", candidates); got != "" {
		t.Fatalf("resolvePartialID ambiguous prefix: got %q, want empty", got)
	}

	// No match returns empty.
	if got := resolvePartialID("zzz", candidates); got != "" {
		t.Fatalf("resolvePartialID no match: got %q, want empty", got)
	}

	// Empty input returns empty.
	if got := resolvePartialID("", candidates); got != "" {
		t.Fatalf("resolvePartialID empty input: got %q, want empty", got)
	}

	// Full ID still resolves.
	if got := resolvePartialID("def456-1700001111", candidates); got != "def456-1700001111" {
		t.Fatalf("resolvePartialID full ID: got %q, want %q", got, "def456-1700001111")
	}

	// Case-insensitive matching.
	if got := resolvePartialID("DEF4", candidates); got != "def456-1700001111" {
		t.Fatalf("resolvePartialID case-insensitive: got %q, want %q", got, "def456-1700001111")
	}
}

func TestResolveByTxHash(t *testing.T) {
	sessions := []*session.SessionData{
		{ID: "abc123-1700000000", TxHash: "aabbccdd11223344"},
		{ID: "def456-1700001111", TxHash: "eeff001122334455"},
	}

	// Unique tx hash prefix resolves.
	if got := resolveByTxHash("aabbcc", sessions); got != "abc123-1700000000" {
		t.Fatalf("resolveByTxHash unique prefix: got %q, want %q", got, "abc123-1700000000")
	}

	// No match returns empty.
	if got := resolveByTxHash("999999", sessions); got != "" {
		t.Fatalf("resolveByTxHash no match: got %q, want empty", got)
	}

	// Empty input returns empty.
	if got := resolveByTxHash("", sessions); got != "" {
		t.Fatalf("resolveByTxHash empty input: got %q, want empty", got)
	}
}

func TestClosestStringMatchCaseInsensitive(t *testing.T) {
	candidates := []string{"MySession-001", "PROD-session-42"}

	if got := closestStringMatch("mysession-001", candidates); got != "MySession-001" {
		t.Fatalf("closestStringMatch case-insensitive: got %q, want %q", got, "MySession-001")
	}
}

func TestClosestStringMatchEmptyCandidates(t *testing.T) {
	if got := closestStringMatch("anything", nil); got != "" {
		t.Fatalf("closestStringMatch nil candidates: got %q", got)
	}
	if got := closestStringMatch("anything", []string{}); got != "" {
		t.Fatalf("closestStringMatch empty candidates: got %q", got)
	}
}
