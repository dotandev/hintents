// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/dotandev/hintents/internal/visualizer"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// Server provides a minimal LSP backend for Soroban hinting.
type Server struct {
	mu        sync.RWMutex
	documents map[protocol.DocumentURI]string

	connMu sync.Mutex
	conn   jsonrpc2.Conn
}

// NewServer creates a new LSP backend server.
func NewServer() *Server {
	return &Server{
		documents: make(map[protocol.DocumentURI]string),
	}
}

// Run serves LSP requests over the provided JSON-RPC stream until the context
// is cancelled or the client disconnects. The main event loop listens for
// context cancellation so JSON-RPC connections and internal channels close
// cleanly instead of leaving orphan processes behind.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
	stream := jsonrpc2.NewStream(&readWriteCloser{Reader: r, Writer: w})
	conn := jsonrpc2.NewConn(stream)

	s.connMu.Lock()
	s.conn = conn
	s.connMu.Unlock()

	conn.Go(ctx, s.handler())

	for {
		select {
		case <-ctx.Done():
			s.closeConnection()
			<-conn.Done()
			if err := conn.Err(); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
				return err
			}
			return ctx.Err()
		case <-conn.Done():
			if err := conn.Err(); err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			return nil
		}
	}
}

func (s *Server) handler() jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		if ctx.Err() != nil {
			return reply(ctx, nil, protocol.ErrRequestCancelled)
		}

		dec := json.NewDecoder(bytes.NewReader(req.Params()))

		switch req.Method() {
		case protocol.MethodInitialize:
			var params protocol.InitializeParams
			if err := dec.Decode(&params); err != nil {
				return reply(ctx, nil, fmt.Errorf("%w: %v", jsonrpc2.ErrParse, err))
			}
			resp, err := s.Initialize(ctx, &params)
			return reply(ctx, resp, err)

		case protocol.MethodInitialized:
			var params protocol.InitializedParams
			if err := dec.Decode(&params); err != nil {
				return reply(ctx, nil, fmt.Errorf("%w: %v", jsonrpc2.ErrParse, err))
			}
			return reply(ctx, nil, s.Initialized(ctx, &params))

		case protocol.MethodShutdown:
			if len(req.Params()) > 0 {
				return reply(ctx, nil, fmt.Errorf("expected no params: %w", jsonrpc2.ErrInvalidParams))
			}
			return reply(ctx, nil, s.Shutdown(ctx))

		case protocol.MethodExit:
			if len(req.Params()) > 0 {
				return reply(ctx, nil, fmt.Errorf("expected no params: %w", jsonrpc2.ErrInvalidParams))
			}
			err := s.Exit(ctx)
			_ = reply(ctx, nil, err)
			return err

		case protocol.MethodTextDocumentDidOpen:
			var params protocol.DidOpenTextDocumentParams
			if err := dec.Decode(&params); err != nil {
				return reply(ctx, nil, fmt.Errorf("%w: %v", jsonrpc2.ErrParse, err))
			}
			return reply(ctx, nil, s.DidOpen(ctx, &params))

		case protocol.MethodTextDocumentDidChange:
			var params protocol.DidChangeTextDocumentParams
			if err := dec.Decode(&params); err != nil {
				return reply(ctx, nil, fmt.Errorf("%w: %v", jsonrpc2.ErrParse, err))
			}
			return reply(ctx, nil, s.DidChange(ctx, &params))

		case protocol.MethodTextDocumentDidClose:
			var params protocol.DidCloseTextDocumentParams
			if err := dec.Decode(&params); err != nil {
				return reply(ctx, nil, fmt.Errorf("%w: %v", jsonrpc2.ErrParse, err))
			}
			return reply(ctx, nil, s.DidClose(ctx, &params))

		case protocol.MethodTextDocumentHover:
			var params protocol.HoverParams
			if err := dec.Decode(&params); err != nil {
				return reply(ctx, nil, fmt.Errorf("%w: %v", jsonrpc2.ErrParse, err))
			}
			resp, err := s.Hover(ctx, &params)
			return reply(ctx, resp, err)

		default:
			return reply(ctx, nil, jsonrpc2.ErrMethodNotFound)
		}
	}
}

func (s *Server) closeConnection() {
	s.connMu.Lock()
	defer s.connMu.Unlock()

	if s.conn == nil {
		return
	}

	_ = s.conn.Close()
	s.conn = nil
}

func (s *Server) clearDocuments() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.documents = make(map[protocol.DocumentURI]string)
}

type readWriteCloser struct {
	io.Reader
	io.Writer
}

func (r *readWriteCloser) Close() error {
	return nil
}

// Initialize validates the LSP initialization request and advertises capabilities.
func (s *Server) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			HoverProvider: true,
			TextDocumentSync: protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindFull,
			},
		},
	}, nil
}

// Initialized is called after initialize completes.
func (s *Server) Initialized(ctx context.Context, params *protocol.InitializedParams) error {
	return nil
}

// Shutdown ends the current LSP session.
func (s *Server) Shutdown(ctx context.Context) error {
	s.clearDocuments()
	return nil
}

// Exit is called when the LSP client exits.
func (s *Server) Exit(ctx context.Context) error {
	s.closeConnection()
	return nil
}

// DidOpen handles textDocument/didOpen.
func (s *Server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	if params.TextDocument.URI == "" {
		return fmt.Errorf("document URI is empty")
	}

	s.mu.Lock()
	s.documents[params.TextDocument.URI] = params.TextDocument.Text
	s.mu.Unlock()
	return nil
}

// DidChange handles textDocument/didChange.
func (s *Server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	if params.TextDocument.URI == "" {
		return fmt.Errorf("document URI is empty")
	}

	if len(params.ContentChanges) == 0 {
		return nil
	}

	text := params.ContentChanges[0].Text
	s.mu.Lock()
	s.documents[params.TextDocument.URI] = text
	s.mu.Unlock()
	return nil
}

// DidClose handles textDocument/didClose.
func (s *Server) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	if params.TextDocument.URI == "" {
		return fmt.Errorf("document URI is empty")
	}

	s.mu.Lock()
	delete(s.documents, params.TextDocument.URI)
	s.mu.Unlock()
	return nil
}

// Hover returns inline hover content for known host functions.
func (s *Server) Hover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	text, found := s.getDocument(params.TextDocument.URI)
	if !found {
		return nil, fmt.Errorf("document not found: %s", params.TextDocument.URI)
	}

	lineText := lineAtPosition(text, params.Position)
	if lineText == "" {
		return nil, nil
	}

	functionName, start, end := hostFunctionAtPosition(lineText, params.Position)
	if functionName == "" {
		return nil, nil
	}

	content := visualizer.HostFunctionHoverContent(functionName)
	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: content,
		},
		Range: &protocol.Range{
			Start: protocol.Position{Line: params.Position.Line, Character: start},
			End:   protocol.Position{Line: params.Position.Line, Character: end},
		},
	}, nil
}

// DiagnosticsForDocument returns diagnostics for the given document URI.
func (s *Server) DiagnosticsForDocument(ctx context.Context, uri protocol.DocumentURI) ([]protocol.Diagnostic, error) {
	text, found := s.getDocument(uri)
	if !found {
		return nil, fmt.Errorf("document not found: %s", uri)
	}

	hints := visualizer.DiagnosticsForSource(text)
	diagnostics := make([]protocol.Diagnostic, 0, len(hints))
	for _, hint := range hints {
		diagnostics = append(diagnostics, protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(hint.Line), Character: hint.Start},
				End:   protocol.Position{Line: uint32(hint.Line), Character: hint.End},
			},
			Severity: protocol.DiagnosticSeverityInformation,
			Source:   "erst-lsp",
			Message:  hint.Message,
		})
	}

	return diagnostics, nil
}

func (s *Server) getDocument(uri protocol.DocumentURI) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	text, ok := s.documents[uri]
	return text, ok
}

// lineAtPosition returns the text of the line at the given LSP position without
// splitting the entire document. It scans forward using strings.IndexByte to
// find newline boundaries, which avoids allocating a slice of all lines and
// copying every sub-string — important for large documents where
// strings.Split("\n") would produce O(n) heap allocations on every hover.
func lineAtPosition(text string, position protocol.Position) string {
	if text == "" {
		return ""
	}

	lineIndex := int(position.Line)
	if lineIndex < 0 {
		return ""
	}

	start := 0
	for i := 0; i < lineIndex; i++ {
		idx := strings.IndexByte(text[start:], '\n')
		if idx < 0 {
			// Requested line is beyond the last line in the document.
			return ""
		}
		start += idx + 1
	}

	// start now points at the beginning of the target line.
	rest := text[start:]
	if end := strings.IndexByte(rest, '\n'); end >= 0 {
		return rest[:end]
	}
	return rest
}

func hostFunctionAtPosition(line string, position protocol.Position) (string, uint32, uint32) {
	if line == "" {
		return "", 0, 0
	}

	cursor := int(position.Character)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(line) {
		cursor = len(line)
	}

	start := cursor
	for start > 0 && isWordCharacter(line[start-1]) {
		start--
	}

	end := cursor
	for end < len(line) && isWordCharacter(line[end]) {
		end++
	}

	word := line[start:end]
	if word == "" {
		return "", 0, 0
	}

	candidates := visualizer.KnownHostFunctions()

	// First try to match the full token as-is (e.g. a simple name like "require_auth").
	for _, candidate := range candidates {
		if word == candidate {
			return word, uint32(start), uint32(end)
		}
	}

	// For fully-qualified Rust paths (e.g. "soroban_sdk::require_auth" or
	// "env.require_auth"), extract the final segment after the last "::" or "."
	// and try to match that against known host functions. The hover range still
	// covers the full qualified token so the editor highlights it correctly.
	segment := word
	if idx := strings.LastIndex(word, "::"); idx >= 0 {
		segment = word[idx+2:]
	} else if idx := strings.LastIndex(word, "."); idx >= 0 {
		segment = word[idx+1:]
	}

	if segment != word && segment != "" {
		for _, candidate := range candidates {
			if segment == candidate {
				return segment, uint32(start), uint32(end)
			}
		}
	}

	return "", 0, 0
}

// isWordCharacter reports whether the byte b is part of a word token in a
// Rust source context. In addition to alphanumeric characters and underscores
// it recognises the ':' used in Rust path separators (::) and the '.' used
// for method-chaining (e.g. env.ledger()), so that fully-qualified names such
// as "soroban_sdk::require_auth" or "env.require_auth" are treated as a single
// token during hover-range scanning.
func isWordCharacter(r byte) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == ':' || r == '.'
}
