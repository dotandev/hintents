// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package testgen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/dotandev/hintents/internal/rpc"
)

// Formal schema validation regex
var (
	txHashRegex = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
	xdrRegex    = regexp.MustCompile(`^[A-Za-z0-9+/=]+$`) // Basic Base64 validation
)

// TestGenerator handles the generation of regression tests
type TestGenerator struct {
	RPCClient *rpc.Client
	OutputDir string
}

// TestData contains the data needed to generate a test.
// Struct tags added to reflect formal schema for internal documentation.
type TestData struct {
	TestName      string        `validate:"required,alphanumeric"`
	TxHash        string        `validate:"required,hex,len=64"`
	EnvelopeXdr   string        `validate:"required,base64"`
	ResultMetaXdr string        `validate:"required,base64"`
	LedgerEntries []LedgerEntry `validate:"min=0"`
}

// Validate audits the input data against formal schemas before processing [Issue #606]
func (d *TestData) Validate() error {
	if d.TestName == "" {
		return fmt.Errorf("formal schema error: TestName is required")
	}
	if !txHashRegex.MatchString(d.TxHash) {
		return fmt.Errorf("formal schema error: TxHash must be a valid 64-character hex string")
	}
	if !xdrRegex.MatchString(d.EnvelopeXdr) || !xdrRegex.MatchString(d.ResultMetaXdr) {
		return fmt.Errorf("formal schema error: Envelope and ResultMeta must be valid XDR strings")
	}
	return nil
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

	// 1. Formal Schema Validation before processing [Issue #606]
	if err := testData.Validate(); err != nil {
		return fmt.Errorf("pre-processing validation failed: %w", err)
	}

	// 2. Proceed with generation
	switch lang {
	case "go":
		return g.GenerateGoTest(testData)
	case "rust":
		return g.GenerateRustTest(testData)
	case "both":
		if goErr := g.GenerateGoTest(testData); goErr != nil {
			return goErr
		}
		return g.GenerateRustTest(testData)
	default:
		return fmt.Errorf("unsupported language: %s", lang)
	}
}

// fetchTransactionData fetches transaction data from the RPC client
func (g *TestGenerator) fetchTransactionData(ctx context.Context, txHash string, testName string) (*TestData, error) {
	resp, err := g.RPCClient.GetTransaction(ctx, txHash)
	if err != nil {
		return nil, err
	}

	if testName == "" {
		testName = sanitizeTestName(txHash)
	}

	// TODO: Fetch ledger entries from transaction footprint
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

	outputDir := filepath.Join(g.OutputDir, "internal", "simulator", "regression_tests")
	if mkdirErr := os.MkdirAll(outputDir, 0755); mkdirErr != nil {
		return fmt.Errorf("failed to create output directory: %w", mkdirErr)
	}

	filename := filepath.Join(outputDir, fmt.Sprintf("regression_%s_test.go", data.TestName))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create Go test file: %w", err)
	}
	defer file.Close()

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

	outputDir := filepath.Join(g.OutputDir, "simulator", "tests", "regression")
	if mkdirErr := os.MkdirAll(outputDir, 0755); mkdirErr != nil {
		return fmt.Errorf("failed to create output directory: %w", mkdirErr)
	}

	filename := filepath.Join(outputDir, fmt.Sprintf("regression_%s.rs", data.TestName))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create Rust test file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute Rust template: %w", err)
	}

	fmt.Printf("Generated Rust test: %s\n", filename)
	return nil
}

// sanitizeTestName converts a transaction hash to a valid test name
func sanitizeTestName(txHash string) string {
	name := txHash
	if len(name) > 8 {
		name = name[:8]
	}
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, name)
	return strings.ToLower(name)
}
