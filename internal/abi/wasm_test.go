// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package abi

import (
	"testing"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildWasm constructs a minimal WASM binary with the given custom sections.
// Each section is a (name, payload) pair.
func buildWasm(sections ...struct {
	name    string
	payload []byte
}) []byte {
	// WASM header: magic + version 1
	buf := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	for _, sec := range sections {
		nameBytes := []byte(sec.name)
		// Section content = name_len (LEB128) + name + payload
		content := appendLEB128(nil, uint32(len(nameBytes)))
		content = append(content, nameBytes...)
		content = append(content, sec.payload...)

		// Section header: id=0 (custom) + content_len (LEB128)
		buf = append(buf, 0x00) // section ID
		buf = appendLEB128(buf, uint32(len(content)))
		buf = append(buf, content...)
	}

	return buf
}

// buildWasmWithNonCustomSection constructs a WASM binary with a non-custom
// section (type section, id=1) followed by a custom section.
func buildWasmWithNonCustomSection(customName string, customPayload []byte) []byte {
	buf := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	// Type section (id=1) with empty content
	typeSectionContent := []byte{0x00} // 0 types
	buf = append(buf, 0x01)            // section ID = 1 (type)
	buf = appendLEB128(buf, uint32(len(typeSectionContent)))
	buf = append(buf, typeSectionContent...)

	// Custom section
	nameBytes := []byte(customName)
	content := appendLEB128(nil, uint32(len(nameBytes)))
	content = append(content, nameBytes...)
	content = append(content, customPayload...)
	buf = append(buf, 0x00) // section ID = 0 (custom)
	buf = appendLEB128(buf, uint32(len(content)))
	buf = append(buf, content...)

	return buf
}

func appendLEB128(buf []byte, val uint32) []byte {
	for {
		b := byte(val & 0x7f)
		val >>= 7
		if val != 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if val == 0 {
			break
		}
	}
	return buf
}

func TestExtractCustomSection_Found(t *testing.T) {
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	wasm := buildWasm(struct {
		name    string
		payload []byte
	}{"contractspecv0", payload})

	result, err := ExtractCustomSection(wasm, "contractspecv0")
	require.NoError(t, err)
	assert.Equal(t, payload, result)
}

func TestExtractCustomSection_NotFound(t *testing.T) {
	wasm := buildWasm(struct {
		name    string
		payload []byte
	}{"other_section", []byte{0x01}})

	result, err := ExtractCustomSection(wasm, "contractspecv0")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestExtractCustomSection_InvalidMagic(t *testing.T) {
	wasm := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x01, 0x00, 0x00, 0x00}

	_, err := ExtractCustomSection(wasm, "contractspecv0")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrWasmInvalid))
}

func TestExtractCustomSection_TooShort(t *testing.T) {
	wasm := []byte{0x00, 0x61}

	_, err := ExtractCustomSection(wasm, "contractspecv0")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrWasmInvalid))
}

func TestExtractCustomSection_EmptyPayload(t *testing.T) {
	wasm := buildWasm(struct {
		name    string
		payload []byte
	}{"contractspecv0", nil})

	result, err := ExtractCustomSection(wasm, "contractspecv0")
	require.NoError(t, err)
	assert.Equal(t, []byte{}, result)
}

func TestExtractCustomSection_MultipleCustomSections(t *testing.T) {
	payload1 := []byte{0x01, 0x02}
	payload2 := []byte{0x03, 0x04}
	wasm := buildWasm(
		struct {
			name    string
			payload []byte
		}{"other", payload1},
		struct {
			name    string
			payload []byte
		}{"contractspecv0", payload2},
	)

	result, err := ExtractCustomSection(wasm, "contractspecv0")
	require.NoError(t, err)
	assert.Equal(t, payload2, result)
}

func TestExtractCustomSection_SkipsNonCustomSections(t *testing.T) {
	payload := []byte{0xCA, 0xFE}
	wasm := buildWasmWithNonCustomSection("contractspecv0", payload)

	result, err := ExtractCustomSection(wasm, "contractspecv0")
	require.NoError(t, err)
	assert.Equal(t, payload, result)
}

func TestExtractCustomSection_TruncatedSection(t *testing.T) {
	// WASM header + section ID 0 + length that extends past EOF
	wasm := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	wasm = append(wasm, 0x00) // section ID = 0
	wasm = append(wasm, 0xFF) // LEB128 length = 127 (way past end)

	_, err := ExtractCustomSection(wasm, "contractspecv0")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrWasmInvalid))
}
