// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package decoder

import (
	"encoding/base64"
	"strings"
	"testing"
)

// TestDecodeEnvelope tests basic functionality and error cases
func TestDecodeEnvelope(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty string",
			input:       "",
			expectError: true,
			errorMsg:    "envelope XDR is empty",
		},
		{
			name:        "invalid base64",
			input:       "invalid base64!",
			expectError: true,
			errorMsg:    "failed to decode base64",
		},
		{
			name:        "valid base64 but invalid XDR",
			input:       base64.StdEncoding.EncodeToString([]byte("not xdr")),
			expectError: true,
			errorMsg:    "failed to unmarshal XDR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeEnvelope(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// FuzzDecodeEnvelope tests the XDR decoder with random input to ensure it doesn't crash
func FuzzDecodeEnvelope(f *testing.F) {
	// Add some seed inputs - valid base64 strings and edge cases
	f.Add("")
	f.Add("invalid")
	f.Add("YWJjZA==") // "abcd" in base64
	f.Add("AAAA")     // Short valid base64

	f.Fuzz(func(t *testing.T, data string) {
		// The decoder should never panic, regardless of input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("DecodeEnvelope panicked with input %q: %v", data, r)
			}
		}()

		// Call the decoder - it should return an error for invalid input, not panic
		_, err := DecodeEnvelope(data)

		// We expect most random inputs to fail, but they should fail gracefully
		if err != nil {
			// Verify the error is descriptive and not just a panic
			if err.Error() == "" {
				t.Errorf("DecodeEnvelope returned empty error message for input %q", data)
			}
		}
	})
}

// FuzzDecodeEnvelopeBytes tests with raw byte input converted to base64
func FuzzDecodeEnvelopeBytes(f *testing.F) {
	// Add some seed byte inputs
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x01, 0x02, 0x03})
	f.Add([]byte{0xFF, 0xFE, 0xFD})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Convert bytes to base64 string
		b64Data := base64.StdEncoding.EncodeToString(data)

		// The decoder should never panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("DecodeEnvelope panicked with byte input (len=%d): %v", len(data), r)
			}
		}()

		// Call the decoder
		_, err := DecodeEnvelope(b64Data)

		// Most random byte sequences won't be valid XDR, but should fail gracefully
		if err != nil && err.Error() == "" {
			t.Errorf("DecodeEnvelope returned empty error message for byte input (len=%d)", len(data))
		}
	})
}
