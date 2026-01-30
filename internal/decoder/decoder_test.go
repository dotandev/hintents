// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package decoder

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createEvent(t *testing.T, fnName string, isCall bool, isReturn bool) string {
	topics := []xdr.ScVal{}
	fnSym := xdr.ScSymbol(fnName)

	if isCall {
		callSym := xdr.ScSymbol("fn_call")
		topics = append(topics, xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &callSym})
		topics = append(topics, xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &fnSym})
	} else if isReturn {
		retSym := xdr.ScSymbol("fn_return")
		topics = append(topics, xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &retSym})
		topics = append(topics, xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &fnSym})
	} else {
		logSym := xdr.ScSymbol("log")
		topics = append(topics, xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &logSym})
		topics = append(topics, xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &fnSym})
	}

	diag := xdr.DiagnosticEvent{
		InSuccessfulContractCall: true,
		Event: xdr.ContractEvent{
			Type: xdr.ContractEventTypeDiagnostic,
			Body: xdr.ContractEventBody{
				V: 0,
				V0: &xdr.ContractEventV0{
					Topics: topics,
					Data:   xdr.ScVal{Type: xdr.ScValTypeScvVoid},
				},
			},
		},
	}

	bytes, err := diag.MarshalBinary()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(bytes)
}

func TestDecodeEvents(t *testing.T) {
	// A calls B, B returns, A returns
	events := []string{
		createEvent(t, "A", true, false),
		createEvent(t, "log_in_A", false, false),
		createEvent(t, "B", true, false),
		createEvent(t, "log_in_B", false, false),
		createEvent(t, "B", false, true),
		createEvent(t, "A", false, true),
	}

	root, err := DecodeEvents(events)
	require.NoError(t, err)

	assert.Equal(t, "TOP_LEVEL", root.Function)
	require.Len(t, root.SubCalls, 1)

	nodeA := root.SubCalls[0]
	assert.Equal(t, "A", nodeA.Function)
	// Expecting 3 events: fn_call A, log_in_A, fn_return A
	require.Len(t, nodeA.Events, 3)

	assert.Equal(t, "A", nodeA.Events[0].Topics[1])
	assert.Equal(t, "log_in_A", nodeA.Events[1].Topics[1])
	assert.Equal(t, "A", nodeA.Events[2].Topics[1])

	require.Len(t, nodeA.SubCalls, 1)
	nodeB := nodeA.SubCalls[0]
	assert.Equal(t, "B", nodeB.Function)
	// Expecting 3 events: fn_call B, log_in_B, fn_return B
	assert.Len(t, nodeB.Events, 3)
}

func TestUnbalanced(t *testing.T) {
	// A calls B, B crashes (no return), A returns
	events := []string{
		createEvent(t, "A", true, false),
		createEvent(t, "B", true, false),
		createEvent(t, "A", false, true),
	}

	root, err := DecodeEvents(events)
	require.NoError(t, err)

	nodeA := root.SubCalls[0]
	assert.Equal(t, "A", nodeA.Function)

	require.Len(t, nodeA.SubCalls, 1)
	nodeB := nodeA.SubCalls[0]
	assert.Equal(t, "B", nodeB.Function)
	// B has call event, but no return event
	assert.Len(t, nodeB.Events, 1)

	// A should have call + return (no log)
	assert.Len(t, nodeA.Events, 2)
}

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
