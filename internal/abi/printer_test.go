// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package abi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatTypeDef_Primitives(t *testing.T) {
	tests := []struct {
		typ      xdr.ScSpecType
		expected string
	}{
		{xdr.ScSpecTypeScSpecTypeVal, "Val"},
		{xdr.ScSpecTypeScSpecTypeBool, "Bool"},
		{xdr.ScSpecTypeScSpecTypeVoid, "Void"},
		{xdr.ScSpecTypeScSpecTypeError, "Error"},
		{xdr.ScSpecTypeScSpecTypeU32, "U32"},
		{xdr.ScSpecTypeScSpecTypeI32, "I32"},
		{xdr.ScSpecTypeScSpecTypeU64, "U64"},
		{xdr.ScSpecTypeScSpecTypeI64, "I64"},
		{xdr.ScSpecTypeScSpecTypeTimepoint, "Timepoint"},
		{xdr.ScSpecTypeScSpecTypeDuration, "Duration"},
		{xdr.ScSpecTypeScSpecTypeU128, "U128"},
		{xdr.ScSpecTypeScSpecTypeI128, "I128"},
		{xdr.ScSpecTypeScSpecTypeU256, "U256"},
		{xdr.ScSpecTypeScSpecTypeI256, "I256"},
		{xdr.ScSpecTypeScSpecTypeBytes, "Bytes"},
		{xdr.ScSpecTypeScSpecTypeString, "String"},
		{xdr.ScSpecTypeScSpecTypeSymbol, "Symbol"},
		{xdr.ScSpecTypeScSpecTypeAddress, "Address"},
		{xdr.ScSpecTypeScSpecTypeMuxedAddress, "MuxedAddress"},
	}

	for _, tc := range tests {
		td := xdr.ScSpecTypeDef{Type: tc.typ}
		assert.Equal(t, tc.expected, FormatTypeDef(td), "type %v", tc.typ)
	}
}

func TestFormatTypeDef_Option(t *testing.T) {
	td := xdr.ScSpecTypeDef{
		Type: xdr.ScSpecTypeScSpecTypeOption,
		Option: &xdr.ScSpecTypeOption{
			ValueType: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU128},
		},
	}
	assert.Equal(t, "Option<U128>", FormatTypeDef(td))
}

func TestFormatTypeDef_Vec(t *testing.T) {
	td := xdr.ScSpecTypeDef{
		Type: xdr.ScSpecTypeScSpecTypeVec,
		Vec: &xdr.ScSpecTypeVec{
			ElementType: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress},
		},
	}
	assert.Equal(t, "Vec<Address>", FormatTypeDef(td))
}

func TestFormatTypeDef_Map(t *testing.T) {
	td := xdr.ScSpecTypeDef{
		Type: xdr.ScSpecTypeScSpecTypeMap,
		Map: &xdr.ScSpecTypeMap{
			KeyType:   xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress},
			ValueType: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeI128},
		},
	}
	assert.Equal(t, "Map<Address, I128>", FormatTypeDef(td))
}

func TestFormatTypeDef_Result(t *testing.T) {
	td := xdr.ScSpecTypeDef{
		Type: xdr.ScSpecTypeScSpecTypeResult,
		Result: &xdr.ScSpecTypeResult{
			OkType:    xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeVoid},
			ErrorType: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeError},
		},
	}
	assert.Equal(t, "Result<Void, Error>", FormatTypeDef(td))
}

func TestFormatTypeDef_Tuple(t *testing.T) {
	td := xdr.ScSpecTypeDef{
		Type: xdr.ScSpecTypeScSpecTypeTuple,
		Tuple: &xdr.ScSpecTypeTuple{
			ValueTypes: []xdr.ScSpecTypeDef{
				{Type: xdr.ScSpecTypeScSpecTypeU32},
				{Type: xdr.ScSpecTypeScSpecTypeBool},
			},
		},
	}
	assert.Equal(t, "(U32, Bool)", FormatTypeDef(td))
}

func TestFormatTypeDef_BytesN(t *testing.T) {
	td := xdr.ScSpecTypeDef{
		Type:   xdr.ScSpecTypeScSpecTypeBytesN,
		BytesN: &xdr.ScSpecTypeBytesN{N: 32},
	}
	assert.Equal(t, "BytesN(32)", FormatTypeDef(td))
}

func TestFormatTypeDef_UDT(t *testing.T) {
	td := xdr.ScSpecTypeDef{
		Type: xdr.ScSpecTypeScSpecTypeUdt,
		Udt:  &xdr.ScSpecTypeUdt{Name: "MyStruct"},
	}
	assert.Equal(t, "MyStruct", FormatTypeDef(td))
}

func TestFormatTypeDef_NestedGenerics(t *testing.T) {
	// Option<Vec<U128>>
	td := xdr.ScSpecTypeDef{
		Type: xdr.ScSpecTypeScSpecTypeOption,
		Option: &xdr.ScSpecTypeOption{
			ValueType: xdr.ScSpecTypeDef{
				Type: xdr.ScSpecTypeScSpecTypeVec,
				Vec: &xdr.ScSpecTypeVec{
					ElementType: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU128},
				},
			},
		},
	}
	assert.Equal(t, "Option<Vec<U128>>", FormatTypeDef(td))
}

func TestFormatText_FullSpec(t *testing.T) {
	spec := &ContractSpec{
		Functions: []xdr.ScSpecFunctionV0{
			{
				Name: "initialize",
				Inputs: []xdr.ScSpecFunctionInputV0{
					{Name: "admin", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
				},
				Outputs: []xdr.ScSpecTypeDef{{Type: xdr.ScSpecTypeScSpecTypeVoid}},
			},
		},
		Structs: []xdr.ScSpecUdtStructV0{
			{
				Name: "Config",
				Fields: []xdr.ScSpecUdtStructFieldV0{
					{Name: "admin", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
				},
			},
		},
		Enums: []xdr.ScSpecUdtEnumV0{
			{
				Name: "Status",
				Cases: []xdr.ScSpecUdtEnumCaseV0{
					{Name: "Active", Value: 0},
				},
			},
		},
	}

	output := FormatText(spec)
	assert.Contains(t, output, "Functions (1):")
	assert.Contains(t, output, "initialize(admin: Address) -> Void")
	assert.Contains(t, output, "Structs (1):")
	assert.Contains(t, output, "Config")
	assert.Contains(t, output, "admin: Address")
	assert.Contains(t, output, "Enums (1):")
	assert.Contains(t, output, "Active = 0")
}

func TestFormatText_EmptySpec(t *testing.T) {
	spec := &ContractSpec{}
	output := FormatText(spec)
	assert.Equal(t, "", output)
}

func TestFormatText_FunctionsOnly(t *testing.T) {
	spec := &ContractSpec{
		Functions: []xdr.ScSpecFunctionV0{
			{
				Name:    "balance",
				Inputs:  []xdr.ScSpecFunctionInputV0{{Name: "addr", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}}},
				Outputs: []xdr.ScSpecTypeDef{{Type: xdr.ScSpecTypeScSpecTypeI128}},
			},
		},
	}

	output := FormatText(spec)
	// Should not have trailing sections
	assert.True(t, strings.HasPrefix(output, "Functions (1):"))
	assert.NotContains(t, output, "Structs")
	assert.NotContains(t, output, "Enums")
}

func TestFormatJSON_ValidOutput(t *testing.T) {
	spec := &ContractSpec{
		Functions: []xdr.ScSpecFunctionV0{
			{
				Name: "hello",
				Inputs: []xdr.ScSpecFunctionInputV0{
					{Name: "to", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
				},
				Outputs: []xdr.ScSpecTypeDef{{Type: xdr.ScSpecTypeScSpecTypeVoid}},
			},
		},
		ErrorEnums: []xdr.ScSpecUdtErrorEnumV0{
			{
				Name: "ContractError",
				Cases: []xdr.ScSpecUdtErrorEnumCaseV0{
					{Name: "NotFound", Value: 1},
				},
			},
		},
	}

	output, err := FormatJSON(spec)
	require.NoError(t, err)

	// Should be valid JSON
	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(output), &parsed))

	// Check structure
	fns, ok := parsed["functions"].([]interface{})
	require.True(t, ok)
	require.Len(t, fns, 1)

	fn := fns[0].(map[string]interface{})
	assert.Equal(t, "hello", fn["name"])

	errEnums, ok := parsed["error_enums"].([]interface{})
	require.True(t, ok)
	require.Len(t, errEnums, 1)
}
