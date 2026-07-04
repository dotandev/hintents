// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package testgen

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/dotandev/hintents/internal/fuzz"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
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
	var generateErr error
	switch lang {
	case "go":
		generateErr = g.GenerateGoTest(testData)
	case "rust":
		generateErr = g.GenerateRustTest(testData)
	case "both":
		if goErr := g.GenerateGoTest(testData); goErr != nil {
			return goErr
		}
		generateErr = g.GenerateRustTest(testData)
	default:
		return fmt.Errorf("unsupported language: %s", lang)
	}

	if generateErr != nil {
		return generateErr
	}

	if err := g.GenerateDynamicFuzzReport(ctx, testData); err != nil {
		return fmt.Errorf("failed to generate dynamic fuzz report: %w", err)
	}

	return nil
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

	ledgerEntries, err := extractLedgerEntries(resp.ResultMetaXdr)
	if err != nil {
		return nil, fmt.Errorf("failed to extract ledger entries from result metadata: %w", err)
	}

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

// GenerateDynamicFuzzReport creates a dynamic fuzzing summary report for the transaction
func (g *TestGenerator) GenerateDynamicFuzzReport(ctx context.Context, data *TestData) error {
	baseInput, err := g.buildFuzzerInput(data)
	if err != nil {
		return err
	}

	fuzzer := fuzz.NewCoverageGuidedFuzzer(simulator.NewDefaultMockRunner(), fuzz.FuzzerConfig{
		MaxIterations:  20,
		TimeoutMs:      3000,
		MaxCorpusSize:  20,
		EnableCoverage: false,
	})

	stats, err := fuzzer.Run(ctx, baseInput)
	if err != nil {
		return fmt.Errorf("dynamic fuzz run failed: %w", err)
	}

	return g.writeFuzzSummary(data.TestName, stats, fuzzer.GetCrashingInputs())
}

func (g *TestGenerator) buildFuzzerInput(data *TestData) (*simulator.FuzzerInput, error) {
	envelopeBytes, err := base64.StdEncoding.DecodeString(data.EnvelopeXdr)
	if err != nil {
		return nil, fmt.Errorf("invalid envelope XDR: %w", err)
	}

	ledgerEntries := make(map[string]string, len(data.LedgerEntries))
	for _, entry := range data.LedgerEntries {
		ledgerEntries[entry.Key] = entry.Value
	}

	return &simulator.FuzzerInput{
		EnvelopeXdr:   hex.EncodeToString(envelopeBytes),
		LedgerEntries: ledgerEntries,
		Timestamp:     time.Now().Unix(),
	}, nil
}

func (g *TestGenerator) writeFuzzSummary(testName string, stats *fuzz.FuzzingStats, crashes []*simulator.FuzzerInput) error {
	outputDir := filepath.Join(g.OutputDir, "internal", "testgen", "fuzz")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create fuzz output directory: %w", err)
	}

	filename := filepath.Join(outputDir, fmt.Sprintf("fuzz_%s_summary.txt", testName))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create fuzz summary file: %w", err)
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "Dynamic fuzz generation report for %s\n", testName)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(file, "Transactions: %s\n", testName)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(file, "Total executions: %d\n", stats.ExecutionCount)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(file, "Crashes found: %d\n", stats.CrashCount)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(file, "New coverage count: %d\n", stats.NewCoverageCount)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(file, "Corpus size: %d\n", stats.CorpusSize)
	if err != nil {
		return err
	}

	if len(crashes) > 0 {
		_, err = fmt.Fprintf(file, "\n-- Crash inputs --\n")
		if err != nil {
			return err
		}
		for i, input := range crashes {
			_, err = fmt.Fprintf(file, "%d: seed=%d envelope_hex=%s ledger_entries=%d\n", i+1, input.Seed, input.EnvelopeXdr, len(input.LedgerEntries))
			if err != nil {
				return err
			}
		}
	}

	fmt.Printf("Generated dynamic fuzz report: %s\n", filename)
	return nil
}

func extractLedgerEntries(resultMetaXdr string) ([]LedgerEntry, error) {
	if resultMetaXdr == "" {
		return []LedgerEntry{}, nil
	}

	entriesMap, err := rpc.ExtractLedgerEntriesFromMeta(resultMetaXdr)
	if err != nil {
		return nil, err
	}

	ledgerEntries := make([]LedgerEntry, 0, len(entriesMap))
	for key, value := range entriesMap {
		ledgerEntries = append(ledgerEntries, LedgerEntry{Key: key, Value: value})
	}

	sort.Slice(ledgerEntries, func(i, j int) bool {
		return ledgerEntries[i].Key < ledgerEntries[j].Key
	})

	return ledgerEntries, nil
}

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
