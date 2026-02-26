// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import "testing"

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
