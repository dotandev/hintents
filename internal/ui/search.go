// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"strings"
)

// SearchFilter represents a parsed search filter
type SearchFilter struct {
	Type string // "changed-key"
	Key  string // the ledger key
}

// ParseSearchQuery parses a search query string and returns the corresponding filter
func ParseSearchQuery(query string) (*SearchFilter, error) {
	query = strings.TrimSpace(query)
	if strings.HasPrefix(query, "changed-key:") {
		key := strings.TrimPrefix(query, "changed-key:")
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("changed-key filter requires a key")
		}
		return &SearchFilter{
			Type: "changed-key",
			Key:  key,
		}, nil
	}
	return nil, fmt.Errorf("unsupported search query: %s", query)
}