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

package testgen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/dotandev/hintents/internal/rpc"
)

// TestGenerator handles the generation of regression tests
type TestGenerator struct {
	RPCClient *rpc.Client
	OutputDir string
}

// TestData contains the data needed to generate a test
type TestData struct {
	TestName      string
	TxHash        string
	EnvelopeXdr   string
	ResultMetaXdr string
	LedgerEntries []LedgerEntry
}

// LedgerEntry represents a key-value pair for ledger state
type LedgerEntry struct {
	Key   string
	Value string
}

// NewTestGenerator creates a new test generator
func NewTestGenerator(client *rpc.Client, outputDir string) *TestGenerator {
	return &TestGenerator{
		RPCClient: client,
		OutputDir: outputDir,
	}
}

// GenerateTests generates both Go and Rust tests for a transaction
func (g *TestGenerator) GenerateTests(ctx context.Context, txHash string, lang string, testName string) error {
	// Fetch transaction data
	testData, err := g.fetchTransactionData(ctx, txHash, testName)
	if err != nil {
		return fmt.Errorf("failed to fetch transaction data: %w", err)
	}

	// Generate tests based on language flag
	switch lang {
	case "go":
		return g.GenerateGoTest(testData)
	case "rust":
		return g.GenerateRustTest(testData)
	case "both":
		if err := g.GenerateGoTest(testData); err != nil {
			return err
		}
		return g.GenerateRustTest(testData)
	default:
		return fmt.Errorf("unsupported language: %s (must be 'go', 'rust', or 'both')", lang)
	}
}

// fetchTransactionData fetches transaction data from the RPC client
func (g *TestGenerator) fetchTransactionData(ctx context.Context, txHash string, testName string) (*TestData, error) {
	resp, err := g.RPCClient.GetTransaction(ctx, txHash)
	if err != nil {
		return nil, err
	}

	// Use provided test name or generate from hash
	if testName == "" {
		testName = sanitizeTestName(txHash)
	}

	// TODO: Fetch ledger entries from transaction footprint
	// For now, we'll use an empty map
	ledgerEntries := []LedgerEntry{}

	return &TestData{
		TestName:      testName,
		TxHash:        txHash,
		EnvelopeXdr:   resp.EnvelopeXdr,
		ResultMetaXdr: resp.ResultMetaXdr,
		LedgerEntries: ledgerEntries,
	}, nil
}

// GenerateGoTest generates a Go test file
func (g *TestGenerator) GenerateGoTest(data *TestData) error {
	tmpl, err := template.New("go_test").Parse(goTestTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse Go template: %w", err)
	}

	// Create output directory
	outputDir := filepath.Join(g.OutputDir, "internal", "simulator", "regression_tests")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	filename := filepath.Join(outputDir, fmt.Sprintf("regression_%s_test.go", data.TestName))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create Go test file: %w", err)
	}
	defer file.Close()

	// Execute template
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute Go template: %w", err)
	}

	fmt.Printf("Generated Go test: %s\n", filename)
	return nil
}

// GenerateRustTest generates a Rust test file
func (g *TestGenerator) GenerateRustTest(data *TestData) error {
	tmpl, err := template.New("rust_test").Parse(rustTestTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse Rust template: %w", err)
	}

	// Create output directory
	outputDir := filepath.Join(g.OutputDir, "simulator", "tests", "regression")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	filename := filepath.Join(outputDir, fmt.Sprintf("regression_%s.rs", data.TestName))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create Rust test file: %w", err)
	}
	defer file.Close()

	// Execute template
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute Rust template: %w", err)
	}

	fmt.Printf("Generated Rust test: %s\n", filename)
	return nil
}

// sanitizeTestName converts a transaction hash to a valid test name
func sanitizeTestName(txHash string) string {
	// Take first 8 characters of hash
	name := txHash
	if len(name) > 8 {
		name = name[:8]
	}
	// Replace any non-alphanumeric characters
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, name)
	return strings.ToLower(name)
}
