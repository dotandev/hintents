// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package testgen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validTxHash = "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"

func TestTestData_Validate(t *testing.T) {
	validBase := func() TestData {
		return TestData{
			TestName:      "sample_tx_1",
			TxHash:        validTxHash,
			EnvelopeXdr:   "AAAAAQ==",
			ResultMetaXdr: "AAAAAAAAAAA=",
		}
	}

	tests := []struct {
		name        string
		mutate      func(*TestData)
		expectErr   bool
		errContains string
	}{
		{
			name:      "valid data",
			mutate:    func(_ *TestData) {},
			expectErr: false,
		},
		{
			name: "valid alphanumeric and underscore name",
			mutate: func(d *TestData) {
				d.TestName = "Valid_Test_Name_123"
			},
			expectErr: false,
		},
		{
			name: "empty name",
			mutate: func(d *TestData) {
				d.TestName = ""
			},
			expectErr:   true,
			errContains: "TestName is required",
		},
		{
			name: "name containing ../",
			mutate: func(d *TestData) {
				d.TestName = "../../evil"
			},
			expectErr:   true,
			errContains: "TestName",
		},
		{
			name: "name containing slash",
			mutate: func(d *TestData) {
				d.TestName = "dir/name"
			},
			expectErr:   true,
			errContains: "TestName",
		},
		{
			name: "name containing hyphen",
			mutate: func(d *TestData) {
				d.TestName = "invalid-name"
			},
			expectErr:   true,
			errContains: "TestName",
		},
		{
			name: "63-character hash",
			mutate: func(d *TestData) {
				d.TxHash = strings.Repeat("a", 63)
			},
			expectErr:   true,
			errContains: "TxHash",
		},
		{
			name: "65-character hash",
			mutate: func(d *TestData) {
				d.TxHash = strings.Repeat("a", 65)
			},
			expectErr:   true,
			errContains: "TxHash",
		},
		{
			name: "non-hex hash",
			mutate: func(d *TestData) {
				d.TxHash = "g" + strings.Repeat("a", 63)
			},
			expectErr:   true,
			errContains: "TxHash",
		},
		{
			name: "invalid envelope XDR",
			mutate: func(d *TestData) {
				d.EnvelopeXdr = "not!valid@xdr"
			},
			expectErr:   true,
			errContains: "XDR",
		},
		{
			name: "empty envelope XDR",
			mutate: func(d *TestData) {
				d.EnvelopeXdr = ""
			},
			expectErr:   true,
			errContains: "XDR",
		},
		{
			name: "invalid result meta XDR",
			mutate: func(d *TestData) {
				d.ResultMetaXdr = "invalid xdr!"
			},
			expectErr:   true,
			errContains: "XDR",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := validBase()
			tc.mutate(&data)

			err := data.Validate()
			if tc.expectErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGenerateGoTest(t *testing.T) {
	outDir := t.TempDir()
	gen := NewTestGenerator(nil, outDir)

	data := &TestData{
		TestName:      "sample_go_regression",
		TxHash:        validTxHash,
		EnvelopeXdr:   "AAAAAQ==",
		ResultMetaXdr: "AAAAAAAAAAA=",
		LedgerEntries: []LedgerEntry{
			{Key: "ledgerKey1", Value: "ledgerVal1"},
		},
	}

	err := gen.GenerateGoTest(data)
	require.NoError(t, err)

	expectedPath := filepath.Join(outDir, "internal", "simulator", "regression_tests", "regression_sample_go_regression_test.go")
	require.FileExists(t, expectedPath)

	contentBytes, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	content := string(contentBytes)

	assert.Contains(t, content, "TestRegression_sample_go_regression")
	assert.Contains(t, content, validTxHash)
	assert.Contains(t, content, "AAAAAQ==")
	assert.Contains(t, content, "AAAAAAAAAAA=")
	assert.Contains(t, content, `"ledgerKey1": "ledgerVal1"`)
}

func TestGenerateRustTest(t *testing.T) {
	outDir := t.TempDir()
	gen := NewTestGenerator(nil, outDir)

	data := &TestData{
		TestName:      "sample_rust_regression",
		TxHash:        validTxHash,
		EnvelopeXdr:   "AAAAAQ==",
		ResultMetaXdr: "AAAAAAAAAAA=",
		LedgerEntries: []LedgerEntry{
			{Key: "ledgerKey1", Value: "ledgerVal1"},
		},
	}

	err := gen.GenerateRustTest(data)
	require.NoError(t, err)

	expectedPath := filepath.Join(outDir, "simulator", "tests", "regression", "regression_sample_rust_regression.rs")
	require.FileExists(t, expectedPath)

	contentBytes, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	content := string(contentBytes)

	assert.Contains(t, content, "test_regression_sample_rust_regression")
	assert.Contains(t, content, validTxHash)
	assert.Contains(t, content, "AAAAAQ==")
	assert.Contains(t, content, "AAAAAAAAAAA=")
	assert.Contains(t, content, `ledger_entries.insert("ledgerKey1".to_string(), "ledgerVal1".to_string());`)
}

func TestSanitizeTestName(t *testing.T) {
	assert.Equal(t, "a1b2c3d4", sanitizeTestName(validTxHash))
	assert.Equal(t, "ab_cd_12", sanitizeTestName("AB-CD/123456"))
	assert.Equal(t, "short", sanitizeTestName("SHORT"))
}

func TestGenerateTests_WithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":              validTxHash,
			"hash":            validTxHash,
			"envelope_xdr":    "AAAAAQ==",
			"result_xdr":      "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA=",
			"result_meta_xdr": "AAAAAAAAAAA=",
		})
	}))
	defer server.Close()

	client, err := rpc.NewClient(rpc.WithHorizonURL(server.URL))
	require.NoError(t, err)

	t.Run("rejects unsafe test name before generation", func(t *testing.T) {
		outDir := t.TempDir()
		gen := NewTestGenerator(client, outDir)

		err := gen.GenerateTests(context.Background(), validTxHash, "go", "../escape")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pre-processing validation failed")
	})

	t.Run("rejects unsupported language", func(t *testing.T) {
		outDir := t.TempDir()
		gen := NewTestGenerator(client, outDir)

		err := gen.GenerateTests(context.Background(), validTxHash, "python", "valid_name")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported language: python")
	})

	t.Run("generates both tests and fuzz report", func(t *testing.T) {
		outDir := t.TempDir()
		gen := NewTestGenerator(client, outDir)

		err := gen.GenerateTests(context.Background(), validTxHash, "both", "full_flow")
		require.NoError(t, err)

		require.FileExists(t, filepath.Join(outDir, "internal", "simulator", "regression_tests", "regression_full_flow_test.go"))
		require.FileExists(t, filepath.Join(outDir, "simulator", "tests", "regression", "regression_full_flow.rs"))
		require.FileExists(t, filepath.Join(outDir, "internal", "testgen", "fuzz", "fuzz_full_flow_summary.txt"))
	})
}
