// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"errors"
	"testing"
)

func TestValidateEnvelope(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		expectedID any
		wantErr    bool
	}{
		{
			name:       "success result",
			response:   `{"jsonrpc":"2.0","id":1,"result":{"value":true}}`,
			expectedID: 1,
		},
		{
			name:       "null result",
			response:   `{"jsonrpc":"2.0","id":1,"result":null}`,
			expectedID: 1,
		},
		{
			name:       "valid error",
			response:   `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"failed"}}`,
			expectedID: 1,
		},
		{
			name:       "numeric ID formatting",
			response:   `{"jsonrpc":"2.0","id":1.0,"result":null}`,
			expectedID: 1,
		},
		{
			name:       "string ID",
			response:   `{"jsonrpc":"2.0","id":"request-1","result":null}`,
			expectedID: "request-1",
		},
		{
			name:       "wrong version",
			response:   `{"jsonrpc":"1.0","id":1,"result":null}`,
			expectedID: 1,
			wantErr:    true,
		},
		{
			name:       "missing version",
			response:   `{"id":1,"result":null}`,
			expectedID: 1,
			wantErr:    true,
		},
		{
			name:       "mismatched ID",
			response:   `{"jsonrpc":"2.0","id":2,"result":null}`,
			expectedID: 1,
			wantErr:    true,
		},
		{
			name:       "missing ID",
			response:   `{"jsonrpc":"2.0","result":null}`,
			expectedID: 1,
			wantErr:    true,
		},
		{
			name:       "missing result and error",
			response:   `{"jsonrpc":"2.0","id":1}`,
			expectedID: 1,
			wantErr:    true,
		},
		{
			name:       "both result and error",
			response:   `{"jsonrpc":"2.0","id":1,"result":null,"error":{"code":-1,"message":"failed"}}`,
			expectedID: 1,
			wantErr:    true,
		},
		{
			name:       "invalid error object",
			response:   `{"jsonrpc":"2.0","id":1,"error":{"code":"-1","message":"failed"}}`,
			expectedID: 1,
			wantErr:    true,
		},
		{
			name:       "fractional error code",
			response:   `{"jsonrpc":"2.0","id":1,"error":{"code":-1.5,"message":"failed"}}`,
			expectedID: 1,
			wantErr:    true,
		},
		{
			name:       "missing error message",
			response:   `{"jsonrpc":"2.0","id":1,"error":{"code":-1}}`,
			expectedID: 1,
			wantErr:    true,
		},
		{
			name:       "null ID",
			response:   `{"jsonrpc":"2.0","id":null,"result":null}`,
			expectedID: 1,
			wantErr:    true,
		},
		{
			name:       "non-object envelope",
			response:   `[]`,
			expectedID: 1,
			wantErr:    true,
		},
		{
			name:       "invalid JSON",
			response:   `{"jsonrpc":"2.0",`,
			expectedID: 1,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnvelope([]byte(tt.response), tt.expectedID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateEnvelope() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var validationErr *EnvelopeValidationError
				if !errors.As(err, &validationErr) {
					t.Fatalf("expected *EnvelopeValidationError, got %T", err)
				}
			}
		})
	}
}

func TestValidateTransactionHash(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		wantErr bool
	}{
		{
			name:    "valid lowercase hash",
			hash:    "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab",
			wantErr: false,
		},
		{
			name:    "valid uppercase hash",
			hash:    "5C0A1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890AB",
			wantErr: false,
		},
		{
			name:    "valid mixed case hash",
			hash:    "5c0a1234567890ABCDEF1234567890abcdef1234567890ABCDEF1234567890ab",
			wantErr: false,
		},
		{
			name:    "invalid length - too short",
			hash:    "123",
			wantErr: true,
		},
		{
			name:    "invalid length - too long",
			hash:    "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab12",
			wantErr: true,
		},
		{
			name:    "invalid characters",
			hash:    "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890gz", // 'g' and 'z' are invalid
			wantErr: true,
		},
		{
			name:    "empty string",
			hash:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransactionHash(tt.hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTransactionHash() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
