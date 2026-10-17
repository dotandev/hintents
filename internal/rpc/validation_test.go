// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"testing"
)

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

func TestValidateEnvelope(t *testing.T) {
	tests := []struct {
		name        string
		resp        *RPCResponseEnvelope
		expectedID  any
		wantErr     bool
		errField    string
	}{
		{
			name: "valid response with result and matching id",
			resp: &RPCResponseEnvelope{
				Jsonrpc: "2.0",
				ID:      1,
				Result:  []byte(`{"status":"healthy"}`),
			},
			expectedID: 1,
			wantErr:    false,
		},
		{
			name: "valid response with error and matching string id",
			resp: &RPCResponseEnvelope{
				Jsonrpc: "2.0",
				ID:      "req-42",
				Error:   []byte(`{"code":-32600,"message":"invalid request"}`),
			},
			expectedID: "req-42",
			wantErr:    false,
		},
		{
			name:       "nil envelope",
			resp:       nil,
			expectedID: 1,
			wantErr:    true,
			errField:   "envelope",
		},
		{
			name: "invalid jsonrpc version",
			resp: &RPCResponseEnvelope{
				Jsonrpc: "1.0",
				ID:      1,
				Result:  []byte(`"ok"`),
			},
			expectedID: 1,
			wantErr:    true,
			errField:   "jsonrpc",
		},
		{
			name: "missing jsonrpc version",
			resp: &RPCResponseEnvelope{
				Jsonrpc: "",
				ID:      1,
				Result:  []byte(`"ok"`),
			},
			expectedID: 1,
			wantErr:    true,
			errField:   "jsonrpc",
		},
		{
			name: "mismatched id",
			resp: &RPCResponseEnvelope{
				Jsonrpc: "2.0",
				ID:      2,
				Result:  []byte(`"ok"`),
			},
			expectedID: 1,
			wantErr:    true,
			errField:   "id",
		},
		{
			name: "both result and error present",
			resp: &RPCResponseEnvelope{
				Jsonrpc: "2.0",
				ID:      1,
				Result:  []byte(`"ok"`),
				Error:   []byte(`{"code":-32000,"message":"err"}`),
			},
			expectedID: 1,
			wantErr:    true,
			errField:   "result/error",
		},
		{
			name: "neither result nor error present",
			resp: &RPCResponseEnvelope{
				Jsonrpc: "2.0",
				ID:      1,
			},
			expectedID: 1,
			wantErr:    true,
			errField:   "result/error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnvelope(tt.resp, tt.expectedID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateEnvelope() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				valErr, ok := err.(*RPCEnvelopeValidationError)
				if !ok {
					t.Fatalf("expected *RPCEnvelopeValidationError, got %T (%v)", err, err)
				}
				if tt.errField != "" && valErr.Field != tt.errField {
					t.Errorf("expected error field %q, got %q", tt.errField, valErr.Field)
				}
			}
		})
	}
}

func TestValidateEnvelopeBytes(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		expectedID any
		wantErr    bool
		errField   string
	}{
		{
			name:       "valid success envelope",
			payload:    `{"jsonrpc":"2.0","id":100,"result":{"sequence":12345}}`,
			expectedID: 100,
			wantErr:    false,
		},
		{
			name:       "valid error envelope",
			payload:    `{"jsonrpc":"2.0","id":"abc","error":{"code":-32001,"message":"not found"}}`,
			expectedID: "abc",
			wantErr:    false,
		},
		{
			name:       "empty bytes payload",
			payload:    "",
			expectedID: 1,
			wantErr:    true,
			errField:   "payload",
		},
		{
			name:       "malformed json payload",
			payload:    `{"jsonrpc": 2.0`,
			expectedID: 1,
			wantErr:    true,
			errField:   "payload",
		},
		{
			name:       "missing jsonrpc member",
			payload:    `{"id":1,"result":"ok"}`,
			expectedID: 1,
			wantErr:    true,
			errField:   "jsonrpc",
		},
		{
			name:       "wrong jsonrpc version string",
			payload:    `{"jsonrpc":"1.0","id":1,"result":"ok"}`,
			expectedID: 1,
			wantErr:    true,
			errField:   "jsonrpc",
		},
		{
			name:       "both result and error present in json",
			payload:    `{"jsonrpc":"2.0","id":1,"result":"data","error":{"code":-32000,"message":"err"}}`,
			expectedID: 1,
			wantErr:    true,
			errField:   "result/error",
		},
		{
			name:       "neither result nor error present in json",
			payload:    `{"jsonrpc":"2.0","id":1}`,
			expectedID: 1,
			wantErr:    true,
			errField:   "result/error",
		},
		{
			name:       "id mismatch in json",
			payload:    `{"jsonrpc":"2.0","id":999,"result":"ok"}`,
			expectedID: 1,
			wantErr:    true,
			errField:   "id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := ValidateEnvelopeBytes([]byte(tt.payload), tt.expectedID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateEnvelopeBytes() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				valErr, ok := err.(*RPCEnvelopeValidationError)
				if !ok {
					t.Fatalf("expected *RPCEnvelopeValidationError, got %T (%v)", err, err)
				}
				if tt.errField != "" && valErr.Field != tt.errField {
					t.Errorf("expected error field %q, got %q", tt.errField, valErr.Field)
				}
			} else {
				if env == nil {
					t.Fatal("expected non-nil envelope on success")
				}
			}
		})
	}
}
