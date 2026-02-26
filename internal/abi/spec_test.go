// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package abi

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func marshalEntries(t *testing.T, entries ...xdr.ScSpecEntry) []byte {
	t.Helper()
	var out []byte
	for _, e := range entries {
		b, err := e.MarshalBinary()
		require.NoError(t, err)
		out = append(out, b...)
	}
	return out
}

func TestDecodeContractSpec_SingleFunction(t *testing.T) {
	fn := xdr.ScSpecFunctionV0{
		Name: "hello",
		Inputs: []xdr.ScSpecFunctionInputV0{
			{Name: "to", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
		},
		Outputs: []xdr.ScSpecTypeDef{
			{Type: xdr.ScSpecTypeScSpecTypeVoid},
		},
	}
	entry := xdr.ScSpecEntry{
		Kind:       xdr.ScSpecEntryKindScSpecEntryFunctionV0,
		FunctionV0: &fn,
	}

	data := marshalEntries(t, entry)
	spec, err := DecodeContractSpec(data)
	require.NoError(t, err)
	require.Len(t, spec.Functions, 1)
	assert.Equal(t, xdr.ScSymbol("hello"), spec.Functions[0].Name)
	assert.Len(t, spec.Functions[0].Inputs, 1)
	assert.Equal(t, "to", spec.Functions[0].Inputs[0].Name)
}

func TestDecodeContractSpec_MultipleMixedEntries(t *testing.T) {
	fnEntry := xdr.ScSpecEntry{
		Kind: xdr.ScSpecEntryKindScSpecEntryFunctionV0,
		FunctionV0: &xdr.ScSpecFunctionV0{
			Name:    "deposit",
			Outputs: []xdr.ScSpecTypeDef{{Type: xdr.ScSpecTypeScSpecTypeVoid}},
		},
	}

	structEntry := xdr.ScSpecEntry{
		Kind: xdr.ScSpecEntryKindScSpecEntryUdtStructV0,
		UdtStructV0: &xdr.ScSpecUdtStructV0{
			Name: "Config",
			Fields: []xdr.ScSpecUdtStructFieldV0{
				{Name: "admin", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
			},
		},
	}

	enumEntry := xdr.ScSpecEntry{
		Kind: xdr.ScSpecEntryKindScSpecEntryUdtEnumV0,
		UdtEnumV0: &xdr.ScSpecUdtEnumV0{
			Name: "Status",
			Cases: []xdr.ScSpecUdtEnumCaseV0{
				{Name: "Active", Value: 0},
				{Name: "Paused", Value: 1},
			},
		},
	}

	data := marshalEntries(t, fnEntry, structEntry, enumEntry)
	spec, err := DecodeContractSpec(data)
	require.NoError(t, err)
	assert.Len(t, spec.Functions, 1)
	assert.Len(t, spec.Structs, 1)
	assert.Len(t, spec.Enums, 1)
	assert.Equal(t, "Config", string(spec.Structs[0].Name))
	assert.Equal(t, "Status", string(spec.Enums[0].Name))
}

func TestDecodeContractSpec_Empty(t *testing.T) {
	spec, err := DecodeContractSpec([]byte{})
	require.NoError(t, err)
	assert.Empty(t, spec.Functions)
	assert.Empty(t, spec.Structs)
	assert.Empty(t, spec.Enums)
	assert.Empty(t, spec.Unions)
	assert.Empty(t, spec.ErrorEnums)
	assert.Empty(t, spec.Events)
}

func TestDecodeContractSpec_CorruptData(t *testing.T) {
	_, err := DecodeContractSpec([]byte{0xFF, 0xFF, 0xFF})
	require.Error(t, err)
}

func TestDecodeContractSpec_ErrorEnum(t *testing.T) {
	entry := xdr.ScSpecEntry{
		Kind: xdr.ScSpecEntryKindScSpecEntryUdtErrorEnumV0,
		UdtErrorEnumV0: &xdr.ScSpecUdtErrorEnumV0{
			Name: "ContractError",
			Cases: []xdr.ScSpecUdtErrorEnumCaseV0{
				{Name: "NotInitialized", Value: 1},
				{Name: "Unauthorized", Value: 2},
			},
		},
	}

	data := marshalEntries(t, entry)
	spec, err := DecodeContractSpec(data)
	require.NoError(t, err)
	require.Len(t, spec.ErrorEnums, 1)
	assert.Equal(t, "ContractError", string(spec.ErrorEnums[0].Name))
	assert.Len(t, spec.ErrorEnums[0].Cases, 2)
}

func TestDecodeContractSpec_Union(t *testing.T) {
	entry := xdr.ScSpecEntry{
		Kind: xdr.ScSpecEntryKindScSpecEntryUdtUnionV0,
		UdtUnionV0: &xdr.ScSpecUdtUnionV0{
			Name: "Outcome",
			Cases: []xdr.ScSpecUdtUnionCaseV0{
				{
					Kind:     xdr.ScSpecUdtUnionCaseV0KindScSpecUdtUnionCaseVoidV0,
					VoidCase: &xdr.ScSpecUdtUnionCaseVoidV0{Name: "None"},
				},
			},
		},
	}

	data := marshalEntries(t, entry)
	spec, err := DecodeContractSpec(data)
	require.NoError(t, err)
	require.Len(t, spec.Unions, 1)
	assert.Equal(t, "Outcome", string(spec.Unions[0].Name))
}
