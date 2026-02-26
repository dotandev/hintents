// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package abi

import (
	"fmt"
	"io"

	"github.com/dotandev/hintents/internal/errors"
)

var wasmMagic = []byte{0x00, 0x61, 0x73, 0x6d} // \0asm

// ExtractCustomSection parses a WASM binary and returns the payload of the
// custom section with the given name. Returns (nil, nil) if the section is
// not present.
func ExtractCustomSection(wasm []byte, name string) ([]byte, error) {
	if len(wasm) < 8 {
		return nil, errors.WrapWasmInvalid("file too short")
	}
	if wasm[0] != wasmMagic[0] || wasm[1] != wasmMagic[1] ||
		wasm[2] != wasmMagic[2] || wasm[3] != wasmMagic[3] {
		return nil, errors.WrapWasmInvalid("bad magic bytes")
	}
	// bytes 4-7: version (we accept any version)

	offset := 8
	for offset < len(wasm) {
		sectionID := wasm[offset]
		offset++

		sectionLen, n, err := decodeLEB128(wasm, offset)
		if err != nil {
			return nil, errors.WrapWasmInvalid(fmt.Sprintf("bad section length at offset %d: %v", offset, err))
		}
		offset += n

		if offset+int(sectionLen) > len(wasm) {
			return nil, errors.WrapWasmInvalid("section extends past end of file")
		}

		sectionEnd := offset + int(sectionLen)

		if sectionID == 0 { // custom section
			nameLen, nn, err := decodeLEB128(wasm, offset)
			if err != nil {
				return nil, errors.WrapWasmInvalid(fmt.Sprintf("bad custom section name length: %v", err))
			}
			offset += nn

			if offset+int(nameLen) > sectionEnd {
				return nil, errors.WrapWasmInvalid("custom section name extends past section")
			}

			sectionName := string(wasm[offset : offset+int(nameLen)])
			offset += int(nameLen)

			if sectionName == name {
				payload := make([]byte, sectionEnd-offset)
				copy(payload, wasm[offset:sectionEnd])
				return payload, nil
			}
		}

		offset = sectionEnd
	}

	return nil, nil
}

// decodeLEB128 decodes an unsigned LEB128 integer from wasm at the given
// offset. Returns the value, the number of bytes consumed, and any error.
func decodeLEB128(data []byte, offset int) (uint32, int, error) {
	var result uint32
	var shift uint
	for i := 0; i < 5; i++ { // u32 needs at most 5 bytes
		if offset+i >= len(data) {
			return 0, 0, io.ErrUnexpectedEOF
		}
		b := data[offset+i]
		result |= uint32(b&0x7f) << shift
		if b&0x80 == 0 {
			return result, i + 1, nil
		}
		shift += 7
	}
	return 0, 0, fmt.Errorf("LEB128 integer too large")
}
