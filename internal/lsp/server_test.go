// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package lsp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.lsp.dev/protocol"
)

func TestIsWordCharacter(t *testing.T) {
	alphanumAndUnderscore := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_")
	for _, b := range alphanumAndUnderscore {
		assert.True(t, isWordCharacter(b), "expected %q to be a word character", b)
	}

	// Rust path and method-chain separators must be word characters.
	assert.True(t, isWordCharacter(':'), "expected ':' to be a word character for Rust paths")
	assert.True(t, isWordCharacter('.'), "expected '.' to be a word character for method chaining")

	// Common delimiters must not be word characters.
	for _, b := range []byte("()[]{}\"' \t\n,;") {
		assert.False(t, isWordCharacter(b), "expected %q NOT to be a word character", b)
	}
}

func TestHostFunctionAtPositionRustNamespaces(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		cursor    uint32
		wantMatch string
	}{
		{
			name:      "simple name",
			line:      "require_auth(account)",
			cursor:    5,
			wantMatch: "require_auth",
		},
		{
			name:      "Rust path separator (::)",
			line:      "soroban_sdk::require_auth(account)",
			cursor:    20,
			wantMatch: "require_auth",
		},
		{
			name:      "method chaining (.)",
			line:      "env.require_auth(account)",
			cursor:    10,
			wantMatch: "require_auth",
		},
		{
			name:      "method chaining storage_put",
			line:      "host.storage_put(k, v)",
			cursor:    10,
			wantMatch: "storage_put",
		},
		{
			name:      "unknown function returns empty",
			line:      "some_unknown_fn()",
			cursor:    5,
			wantMatch: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := protocol.Position{Line: 0, Character: tt.cursor}
			fn, _, _ := hostFunctionAtPosition(tt.line, pos)
			assert.Equal(t, tt.wantMatch, fn)
		})
	}
}

func TestHoverWithRustQualifiedPaths(t *testing.T) {
	srv := NewServer()
	uri := protocol.DocumentURI("file:///test_rust.soroban")

	openParams := &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  uri,
			Text: "soroban_sdk::require_auth(account)\nenv.storage_put(key, value)",
		},
	}
	assert.NoError(t, srv.DidOpen(context.Background(), openParams))

	// Hover over "require_auth" in "soroban_sdk::require_auth"
	hoverParams := &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 20},
		},
	}
	hover, err := srv.Hover(context.Background(), hoverParams)
	assert.NoError(t, err)
	assert.NotNil(t, hover, "expected hover result for soroban_sdk::require_auth")
	if hover != nil {
		assert.Contains(t, hover.Contents.Value, "require_auth")
	}

	// Hover over "storage_put" in "env.storage_put"
	hoverParams2 := &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 1, Character: 8},
		},
	}
	hover2, err := srv.Hover(context.Background(), hoverParams2)
	assert.NoError(t, err)
	assert.NotNil(t, hover2, "expected hover result for env.storage_put")
	if hover2 != nil {
		assert.Contains(t, hover2.Contents.Value, "storage_put")
	}
}

func TestServerDocumentLifecycleAndHover(t *testing.T) {
	srv := NewServer()
	uri := protocol.DocumentURI("file:///test.soroban")

	openParams := &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  uri,
			Text: "require_auth(account)\nstorage_put(key, value)",
		},
	}
	assert.NoError(t, srv.DidOpen(context.Background(), openParams))

	hoverParams := &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 2},
		},
	}
	hover, err := srv.Hover(context.Background(), hoverParams)
	assert.NoError(t, err)
	assert.NotNil(t, hover)
	assert.Contains(t, hover.Contents.Value, "require_auth")

	diagnostics, err := srv.DiagnosticsForDocument(context.Background(), uri)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(diagnostics), 1)
	assert.Contains(t, diagnostics[0].Message, "require_auth")

	changeParams := &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{Text: "storage_get(key)\n"},
		},
	}
	assert.NoError(t, srv.DidChange(context.Background(), changeParams))

	diagnostics, err = srv.DiagnosticsForDocument(context.Background(), uri)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(diagnostics), 1)
	assert.Contains(t, diagnostics[0].Message, "storage_get")

	closeParams := &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	}
	assert.NoError(t, srv.DidClose(context.Background(), closeParams))

	_, err = srv.DiagnosticsForDocument(context.Background(), uri)
	assert.Error(t, err)
}
