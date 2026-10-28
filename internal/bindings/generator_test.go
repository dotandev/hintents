// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package bindings

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/abi"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// typeCheckGoSource parses and type-checks generated Go source, failing the
// test on any syntax or type error (for example duplicate switch cases).
func typeCheckGoSource(t *testing.T, src string) {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "errors.go", src, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated Go source does not parse: %v\n%s", err, src)
	}

	conf := types.Config{Importer: importer.Default()}
	if _, err := conf.Check("generated", fset, []*ast.File{file}, nil); err != nil {
		t.Fatalf("generated Go source does not type-check: %v\n%s", err, src)
	}
}

func TestMapTypeDefToTS(t *testing.T) {
	g := &Generator{}

	tests := []struct {
		name     string
		typeDef  xdr.ScSpecTypeDef
		expected string
	}{
		{
			name:     "Bool",
			typeDef:  xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeBool},
			expected: "boolean",
		},
		{
			name:     "U64",
			typeDef:  xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU64},
			expected: "bigint",
		},
		{
			name:     "String",
			typeDef:  xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeString},
			expected: "string",
		},
		{
			name:     "Address",
			typeDef:  xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress},
			expected: "Address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.mapTypeDefToTS(tt.typeDef)
			if result != tt.expected {
				t.Errorf("mapTypeDefToTS() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello-world", "HelloWorld"},
		{"my_contract", "MyContract"},
		{"simple", "Simple"},
		{"multi-word-test", "MultiWordTest"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateGoErrors(t *testing.T) {
	g := &Generator{
		config: GeneratorConfig{PackageName: "my-contract"},
		spec: &abi.ContractSpec{
			ErrorEnums: []xdr.ScSpecUdtErrorEnumV0{
				{
					Name: "ContractError",
					Doc:  "Errors returned by the\ncontract.",
					Cases: []xdr.ScSpecUdtErrorEnumCaseV0{
						{Name: "NotInitialized", Value: 1, Doc: "contract is not initialized"},
						{Name: "Unauthorized", Value: 2},
						{Name: "TransferFailed", Value: 1},
					},
				},
			},
		},
	}

	src := g.generateGoErrors()

	// The generated file must be valid, type-correct Go source.
	typeCheckGoSource(t, src)

	checks := []string{
		"package mycontract",
		"type ContractError uint32",
		"ContractErrorNotInitialized ContractError = 1",
		"ContractErrorUnauthorized ContractError = 2",
		"ContractErrorTransferFailed ContractError = 1",
		"func (e ContractError) Error() string {",
		`return "NotInitialized: contract is not initialized"`,
		`return "Unauthorized"`,
		"// ContractError Errors returned by the contract.",
	}

	for _, want := range checks {
		if !strings.Contains(src, want) {
			t.Errorf("generated Go errors missing %q\n%s", want, src)
		}
	}

	// Duplicate error codes must not produce duplicate switch cases.
	if strings.Contains(src, "case ContractErrorTransferFailed:") {
		t.Errorf("duplicate code should be skipped in switch:\n%s", src)
	}
}

func TestGenerateGoErrors_NoErrorEnums(t *testing.T) {
	g := &Generator{
		config: GeneratorConfig{PackageName: "empty"},
		spec:   &abi.ContractSpec{},
	}

	src := g.generateGoErrors()
	if !strings.Contains(src, "package empty") {
		t.Errorf("expected package declaration, got %q", src)
	}
	if strings.Contains(src, "import") {
		t.Errorf("no import expected without error enums, got %q", src)
	}
}

func TestGenerateGoErrorEnum_NormalizesIdentifiers(t *testing.T) {
	g := &Generator{
		config: GeneratorConfig{PackageName: "123-contract"},
		spec: &abi.ContractSpec{
			ErrorEnums: []xdr.ScSpecUdtErrorEnumV0{
				{
					Name: "1Bad-Name",
					Cases: []xdr.ScSpecUdtErrorEnumCaseV0{
						{Name: "duplicate", Value: 5},
						{Name: "duplicate", Value: 6},
					},
				},
			},
		},
	}

	src := g.generateGoErrors()
	typeCheckGoSource(t, src)

	if !strings.Contains(src, "package contract123contract") {
		t.Errorf("package name not normalized:\n%s", src)
	}
	// Colliding constant names must be disambiguated.
	if !strings.Contains(src, "Error1BadNameDuplicate Error1BadName = 5") {
		t.Errorf("first collision not handled:\n%s", src)
	}
	if !strings.Contains(src, "Error1BadNameDuplicate2 Error1BadName = 6") {
		t.Errorf("second collision not handled:\n%s", src)
	}
}

func TestGenerateGoErrorEnum_UniqueAcrossEnums(t *testing.T) {
	g := &Generator{
		config: GeneratorConfig{PackageName: "collide"},
		spec: &abi.ContractSpec{
			ErrorEnums: []xdr.ScSpecUdtErrorEnumV0{
				{Name: "Error", Cases: []xdr.ScSpecUdtErrorEnumCaseV0{{Name: "Code", Value: 1}}},
				{Name: "Error", Cases: []xdr.ScSpecUdtErrorEnumCaseV0{{Name: "Code", Value: 2}}},
			},
		},
	}

	src := g.generateGoErrors()
	typeCheckGoSource(t, src)

	if !strings.Contains(src, "type Error uint32") {
		t.Errorf("first error enum type missing:\n%s", src)
	}
	if !strings.Contains(src, "type Error2 uint32") {
		t.Errorf("second error enum type not disambiguated:\n%s", src)
	}
}
