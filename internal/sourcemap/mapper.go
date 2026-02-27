// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/dotandev/hintents/internal/dwarf"
	"github.com/dotandev/hintents/internal/logger"
)

// WASMSourceMapper maps WASM offsets to source code locations using DWARF debug info
type WASMSourceMapper struct {
	parser      *dwarf.Parser
	wasmHash    string
	hasSymbols  bool
	mappings    map[uint64]*dwarf.SourceLocation
	mu          sync.RWMutex
	initialized bool
}

// NewWASMSourceMapper creates a new source mapper from WASM bytes
func NewWASMSourceMapper(wasmBytes []byte) (*WASMSourceMapper, error) {
	if len(wasmBytes) == 0 {
		return nil, fmt.Errorf("empty WASM bytes")
	}

	// Compute hash for caching
	hash := sha256.Sum256(wasmBytes)
	wasmHash := hex.EncodeToString(hash[:])

	// Try to parse DWARF info
	parser, err := dwarf.NewParser(wasmBytes)
	hasSymbols := err == nil && parser != nil

	if !hasSymbols {
		logger.Logger.Debug("No DWARF debug symbols found in WASM", "hash", wasmHash[:8])
	} else {
		logger.Logger.Info("DWARF debug symbols found in WASM", "hash", wasmHash[:8])
	}

	return &WASMSourceMapper{
		parser:      parser,
		wasmHash:    wasmHash,
		hasSymbols:  hasSymbols,
		mappings:    make(map[uint64]*dwarf.SourceLocation),
		initialized: false,
	}, nil
}

// HasDebugSymbols returns true if the WASM contains DWARF debug information
func (m *WASMSourceMapper) HasDebugSymbols() bool {
	return m.hasSymbols
}

// GetWASMHash returns the SHA256 hash of the WASM bytes
func (m *WASMSourceMapper) GetWASMHash() string {
	return m.wasmHash
}

// MapOffsetToSource maps a WASM offset to a source code location
func (m *WASMSourceMapper) MapOffsetToSource(offset uint64) (*dwarf.SourceLocation, error) {
	if !m.hasSymbols {
		return nil, fmt.Errorf("no debug symbols available")
	}

	// Check cache first
	m.mu.RLock()
	if loc, ok := m.mappings[offset]; ok {
		m.mu.RUnlock()
		return loc, nil
	}
	m.mu.RUnlock()

	// Parse from DWARF
	loc, err := m.parser.GetSourceLocation(offset)
	if err != nil {
		return nil, fmt.Errorf("failed to map offset 0x%x: %w", offset, err)
	}

	// Cache the result
	m.mu.Lock()
	m.mappings[offset] = loc
	m.mu.Unlock()

	return loc, nil
}

// GetSubprogramAt finds the function containing the given offset
func (m *WASMSourceMapper) GetSubprogramAt(offset uint64) (*dwarf.SubprogramInfo, error) {
	if !m.hasSymbols {
		return nil, fmt.Errorf("no debug symbols available")
	}

	return m.parser.FindSubprogramAt(offset)
}

// GetLocalVarsAt finds local variables visible at the given offset
func (m *WASMSourceMapper) GetLocalVarsAt(offset uint64) ([]dwarf.LocalVar, error) {
	if !m.hasSymbols {
		return nil, fmt.Errorf("no debug symbols available")
	}

	return m.parser.FindLocalVarsAt(offset)
}

// GetAllSubprograms returns all functions in the WASM
func (m *WASMSourceMapper) GetAllSubprograms() ([]dwarf.SubprogramInfo, error) {
	if !m.hasSymbols {
		return nil, fmt.Errorf("no debug symbols available")
	}

	return m.parser.GetSubprograms()
}

// PreloadMappings eagerly loads all source mappings into cache
// This is useful for performance when you know you'll need many mappings
func (m *WASMSourceMapper) PreloadMappings() error {
	if !m.hasSymbols {
		return fmt.Errorf("no debug symbols available")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil // Already preloaded
	}

	// Get all subprograms
	subprograms, err := m.parser.GetSubprograms()
	if err != nil {
		return fmt.Errorf("failed to get subprograms: %w", err)
	}

	// For each subprogram, try to get source locations
	for _, subprog := range subprograms {
		// Map the function entry point
		if subprog.LowPC > 0 {
			loc, err := m.parser.GetSourceLocation(subprog.LowPC)
			if err == nil && loc != nil {
				m.mappings[subprog.LowPC] = loc
			}
		}
	}

	m.initialized = true
	logger.Logger.Info("Preloaded source mappings", 
		"count", len(m.mappings),
		"hash", m.wasmHash[:8])

	return nil
}

// GetMappingCount returns the number of cached mappings
func (m *WASMSourceMapper) GetMappingCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.mappings)
}

// ClearCache clears all cached mappings
func (m *WASMSourceMapper) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mappings = make(map[uint64]*dwarf.SourceLocation)
	m.initialized = false
}
