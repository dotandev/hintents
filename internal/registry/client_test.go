// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePackage(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    PackageRef
		expectErr   bool
		errContains string
	}{
		{
			name:      "valid package reference",
			input:     "@oz/token@v1.0.0",
			expected:  PackageRef{Org: "oz", Repo: "token", Version: "v1.0.0"},
			expectErr: false,
		},
		{
			name:        "missing @",
			input:       "oz/token@v1.0.0",
			expectErr:   true,
			errContains: "invalid package format, expected @org/repo@version",
		},
		{
			name:        "missing version",
			input:       "@oz/token",
			expectErr:   true,
			errContains: "version is required",
		},
		{
			name:        "org/repo/extra",
			input:       "@oz/token/extra@v1.0.0",
			expectErr:   true,
			errContains: "invalid package path, expected org/repo",
		},
		{
			name:        "empty version",
			input:       "@oz/token@",
			expectErr:   true,
			errContains: "version is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := ParsePackage(tc.input)
			if tc.expectErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, ref)
			}
		})
	}
}
