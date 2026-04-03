// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package protocolreg

import "testing"

func TestParseDebugURI(t *testing.T) {
	parsed, err := ParseDebugURI("erst://debug/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef?network=testnet&operation=2&source=dashboard")
	if err != nil {
		t.Fatalf("ParseDebugURI returned error: %v", err)
	}

	if parsed.TransactionHash != "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Fatalf("unexpected transaction hash: %s", parsed.TransactionHash)
	}
	if parsed.Network != "testnet" {
		t.Fatalf("unexpected network: %s", parsed.Network)
	}
	if parsed.Operation == nil || *parsed.Operation != 2 {
		t.Fatalf("unexpected operation: %#v", parsed.Operation)
	}
	if parsed.Source != "dashboard" {
		t.Fatalf("unexpected source: %s", parsed.Source)
	}
}

func TestParseDebugURIRejectsInvalidValues(t *testing.T) {
	tests := []string{
		"",
		"https://example.com",
		"erst://decode/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef?network=testnet",
		"erst://debug/not-a-hash?network=testnet",
		"erst://debug/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"erst://debug/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef?network=invalid",
		"erst://debug/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef?network=testnet&operation=-1",
	}

	for _, tc := range tests {
		if _, err := ParseDebugURI(tc); err == nil {
			t.Fatalf("expected ParseDebugURI to fail for %q", tc)
		}
	}
}