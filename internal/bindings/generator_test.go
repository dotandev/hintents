// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package bindings

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestMapTypeDefToTS(t *testing.T) {
	g := &Generator{}

	tests := []struct {
		name     string
		typeDef  xdr.ScSpecTypeDef
		expected string
	}{
		{
			name:     "Bool",
			typeDef:  xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeBool},
			expected: "boolean",
		},
		{
			name:     "U64",
			typeDef:  xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU64},
			expected: "bigint",
		},
		{
			name:     "String",
			typeDef:  xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeString},
			expected: "string",
		},
		{
			name:     "Address",
			typeDef:  xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress},
			expected: "Address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.mapTypeDefToTS(tt.typeDef)
			if result != tt.expected {
				t.Errorf("mapTypeDefToTS() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello-world", "HelloWorld"},
		{"my_contract", "MyContract"},
		{"simple", "Simple"},
		{"multi-word-test", "MultiWordTest"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
