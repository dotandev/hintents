// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package bindings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/abi"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestGenerateBindings(t *testing.T) {
	// Create a mock contract spec
	spec := &abi.ContractSpec{
		Functions: []xdr.ScSpecFunctionV0{
			{
				Name: "transfer",
				Inputs: []xdr.ScSpecFunctionInputV0{
					{Name: "from", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
					{Name: "to", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
					{Name: "amount", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU128}},
				},
				Outputs: []xdr.ScSpecTypeDef{
					{Type: xdr.ScSpecTypeScSpecTypeVoid},
				},
			},
		},
		Structs: []xdr.ScSpecUdtStructV0{
			{
				Name: "TokenInfo",
				Fields: []xdr.ScSpecUdtStructFieldV0{
					{Name: "name", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeString}},
					{Name: "symbol", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeString}},
					{Name: "decimals", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU32}},
				},
			},
		},
		Enums: []xdr.ScSpecUdtEnumV0{
			{
				Name: "Status",
				Cases: []xdr.ScSpecUdtEnumCaseV0{
					{Name: "Active", Value: 0},
					{Name: "Inactive", Value: 1},
				},
			},
		},
	}

	generator := &Generator{
		config: GeneratorConfig{
			PackageName: "test-contract",
			ContractID:  "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
			Network:     "testnet",
		},
		spec: spec,
	}

	// Test types generation
	t.Run("GenerateTypes", func(t *testing.T) {
		types := generator.generateTypes()

		// Check for base types
		if !strings.Contains(types, "export type Address = string;") {
			t.Error("Missing Address type definition")
		}

		// Check for struct
		if !strings.Contains(types, "export interface TokenInfo") {
			t.Error("Missing TokenInfo struct")
		}

		// Check for enum
		if !strings.Contains(types, "export enum Status") {
			t.Error("Missing Status enum")
		}
	})

	// Test client generation
	t.Run("GenerateClient", func(t *testing.T) {
		client := generator.generateClient()

		// Check for client class
		if !strings.Contains(client, "export class TestContractClient") {
			t.Error("Missing client class")
		}

		// Check for transfer method
		if !strings.Contains(client, "async transfer(") {
			t.Error("Missing transfer method")
		}

		// Check for erst integration
		if !strings.Contains(client, "ErstSimulator") {
			t.Error("Missing erst integration")
		}
	})

	// Test erst integration generation
	t.Run("GenerateErstIntegration", func(t *testing.T) {
		erst := generator.generateErstIntegration()

		// Check for simulator class
		if !strings.Contains(erst, "export class ErstSimulator") {
			t.Error("Missing ErstSimulator class")
		}

		// Check for simulate method
		if !strings.Contains(erst, "async simulate(") {
			t.Error("Missing simulate method")
		}

		// Check for debug method
		if !strings.Contains(erst, "async debugTransaction(") {
			t.Error("Missing debugTransaction method")
		}
	})

	// Test package.json generation
	t.Run("GeneratePackageJSON", func(t *testing.T) {
		pkg := generator.generatePackageJSON()

		// Check for package name
		if !strings.Contains(pkg, `"name": "test-contract"`) {
			t.Error("Missing package name")
		}

		// Check for stellar-sdk dependency
		if !strings.Contains(pkg, "@stellar/stellar-sdk") {
			t.Error("Missing stellar-sdk dependency")
		}
	})

	// Test README generation
	t.Run("GenerateReadme", func(t *testing.T) {
		readme := generator.generateReadme()

		// Check for title
		if !strings.Contains(readme, "# test-contract") {
			t.Error("Missing title")
		}

		// Check for usage examples
		if !strings.Contains(readme, "## Usage") {
			t.Error("Missing usage section")
		}

		// Check for contract methods
		if !strings.Contains(readme, "## Contract Methods") {
			t.Error("Missing contract methods section")
		}
	})
}

func TestGenerateBindingsEndToEnd(t *testing.T) {
	// Skip if no test WASM file available
	testWasmPath := os.Getenv("TEST_WASM_PATH")
	if testWasmPath == "" {
		t.Skip("TEST_WASM_PATH not set, skipping end-to-end test")
	}

	wasmBytes, err := os.ReadFile(testWasmPath)
	if err != nil {
		t.Fatalf("Failed to read test WASM: %v", err)
	}

	// Create temp directory for output
	tmpDir := t.TempDir()

	config := GeneratorConfig{
		WasmBytes:   wasmBytes,
		OutputDir:   tmpDir,
		PackageName: "test-contract",
		ContractID:  "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
		Network:     "testnet",
	}

	generator := NewGenerator(config)
	files, err := generator.Generate()
	if err != nil {
		t.Fatalf("Failed to generate bindings: %v", err)
	}

	// Verify all expected files are generated
	expectedFiles := []string{
		"types.ts",
		"client.ts",
		"erst-integration.ts",
		"index.ts",
		"package.json",
		"README.md",
	}

	for _, expectedFile := range expectedFiles {
		found := false
		for _, file := range files {
			if file.Path == expectedFile {
				found = true

				// Verify file has content
				if len(file.Content) == 0 {
					t.Errorf("File %s has no content", expectedFile)
				}

				// Write file to temp dir
				fullPath := filepath.Join(tmpDir, file.Path)
				if err := os.WriteFile(fullPath, []byte(file.Content), 0644); err != nil {
					t.Errorf("Failed to write file %s: %v", expectedFile, err)
				}

				break
			}
		}

		if !found {
			t.Errorf("Expected file %s not generated", expectedFile)
		}
	}

	// Verify files exist on disk
	for _, expectedFile := range expectedFiles {
		fullPath := filepath.Join(tmpDir, expectedFile)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("File %s not found on disk", expectedFile)
		}
	}
}
