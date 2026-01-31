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
