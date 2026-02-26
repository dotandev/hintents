// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package abi

import (
	"bytes"
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// ContractSpec holds the decoded Soroban contract specification, grouped by
// entry kind.
type ContractSpec struct {
	Functions  []xdr.ScSpecFunctionV0
	Structs    []xdr.ScSpecUdtStructV0
	Unions     []xdr.ScSpecUdtUnionV0
	Enums      []xdr.ScSpecUdtEnumV0
	ErrorEnums []xdr.ScSpecUdtErrorEnumV0
	Events     []xdr.ScSpecEventV0
}

// DecodeContractSpec reads concatenated XDR-encoded ScSpecEntry values and
// returns them grouped by kind.
func DecodeContractSpec(data []byte) (*ContractSpec, error) {
	spec := &ContractSpec{}
	reader := bytes.NewReader(data)

	for reader.Len() > 0 {
		var entry xdr.ScSpecEntry
		_, err := xdr.Unmarshal(reader, &entry)
		if err != nil {
			return nil, fmt.Errorf("decoding spec entry: %w", err)
		}

		switch entry.Kind {
		case xdr.ScSpecEntryKindScSpecEntryFunctionV0:
			spec.Functions = append(spec.Functions, *entry.FunctionV0)
		case xdr.ScSpecEntryKindScSpecEntryUdtStructV0:
			spec.Structs = append(spec.Structs, *entry.UdtStructV0)
		case xdr.ScSpecEntryKindScSpecEntryUdtUnionV0:
			spec.Unions = append(spec.Unions, *entry.UdtUnionV0)
		case xdr.ScSpecEntryKindScSpecEntryUdtEnumV0:
			spec.Enums = append(spec.Enums, *entry.UdtEnumV0)
		case xdr.ScSpecEntryKindScSpecEntryUdtErrorEnumV0:
			spec.ErrorEnums = append(spec.ErrorEnums, *entry.UdtErrorEnumV0)
		case xdr.ScSpecEntryKindScSpecEntryEventV0:
			spec.Events = append(spec.Events, *entry.EventV0)
		default:
			return nil, fmt.Errorf("unknown spec entry kind: %d", entry.Kind)
		}
	}

	return spec, nil
}
