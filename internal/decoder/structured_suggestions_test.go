// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package decoder

import (
	"encoding/base64"
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStructuredSuggestionsIgnoreHostWording(t *testing.T) {
	cases := []struct {
		name string
		kind xdr.ScErrorType
		code xdr.ScErrorCode
		rule string
	}{
		{"storage", xdr.ScErrorTypeSceStorage, xdr.ScErrorCodeScecMissingValue, "uninitialized_contract"},
		{"auth", xdr.ScErrorTypeSceAuth, xdr.ScErrorCodeScecInvalidAction, "missing_authorization"},
		{"value", xdr.ScErrorTypeSceValue, xdr.ScErrorCodeScecUnexpectedType, "invalid_parameters"},
		{"object", xdr.ScErrorTypeSceObject, xdr.ScErrorCodeScecUnexpectedSize, "invalid_parameters"},
		{"budget", xdr.ScErrorTypeSceBudget, xdr.ScErrorCodeScecExceededLimit, "resource_limit_exceeded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, wording := range []string{"host wording changed", "invalid balance unauthorized missing"} {
				errorValue, err := xdr.NewScError(tc.kind, tc.code)
				require.NoError(t, err)
				suggestions := NewSuggestionEngine().AnalyzeEvents([]DecodedEvent{{
					ContractID: "contract", Error: &errorValue, Data: wording,
				}})
				require.Len(t, suggestions, 1)
				assert.Equal(t, tc.rule, suggestions[0].Rule)
				assert.Equal(t, confidenceHigh, suggestions[0].Confidence)
			}
		})
	}
}

func TestDiagnosticCriteriaRequiresAllPopulatedFields(t *testing.T) {
	code := xdr.ScErrorCodeScecUnexpectedType
	event := DecodedEvent{
		Error:            &xdr.ScError{Type: xdr.ScErrorTypeSceValue, Code: &code},
		ContractFunction: "transfer",
		ArgumentTypes:    []xdr.ScValType{xdr.ScValTypeScvAddress, xdr.ScValTypeScvI128},
	}
	criteria := DiagnosticCriteria{
		ErrorTypes:    []xdr.ScErrorType{xdr.ScErrorTypeSceObject, xdr.ScErrorTypeSceValue},
		ErrorCodes:    []xdr.ScErrorCode{code},
		Functions:     []string{"transfer", "approve"},
		ArgumentTypes: []xdr.ScValType{xdr.ScValTypeScvI128, xdr.ScValTypeScvAddress},
	}
	assert.True(t, criteria.matches(event))
	assert.False(t, (DiagnosticCriteria{}).matches(event))
	assert.False(t, (DiagnosticCriteria{ErrorTypes: []xdr.ScErrorType{xdr.ScErrorTypeSceAuth}}).matches(event))
	assert.False(t, (DiagnosticCriteria{ErrorCodes: []xdr.ScErrorCode{xdr.ScErrorCodeScecMissingValue}}).matches(event))
	for _, mismatch := range []DecodedEvent{
		{ContractFunction: event.ContractFunction, ArgumentTypes: event.ArgumentTypes},
		{Error: event.Error, ContractFunction: "balance", ArgumentTypes: event.ArgumentTypes},
		{Error: event.Error, ContractFunction: event.ContractFunction, ArgumentTypes: []xdr.ScValType{xdr.ScValTypeScvI128}},
	} {
		assert.False(t, criteria.matches(mismatch))
	}
	contractCode := xdr.Uint32(code)
	assert.False(t, criteria.matches(DecodedEvent{
		Error:            &xdr.ScError{Type: xdr.ScErrorTypeSceContract, ContractCode: &contractCode},
		ContractFunction: event.ContractFunction, ArgumentTypes: event.ArgumentTypes,
	}), "contract-specific numeric errors must not become host error codes")
}

func TestStructuredSuggestionsKeepFallbackOrderAndDeduplicate(t *testing.T) {
	code := xdr.ScErrorCodeScecExceededLimit
	events := []DecodedEvent{
		{ContractID: "contract", Data: "insufficient balance"},
		{ContractID: "contract", Data: "message changed", Error: &xdr.ScError{Type: xdr.ScErrorTypeSceBudget, Code: &code}},
		{ContractID: "contract", Data: "more text", Error: &xdr.ScError{Type: xdr.ScErrorTypeSceBudget, Code: &code}},
	}
	got := NewSuggestionEngine().AnalyzeEvents(events)
	require.Len(t, got, 2)
	assert.Equal(t, "insufficient_balance", got[0].Rule)
	assert.Equal(t, "resource_limit_exceeded", got[1].Rule)
	assert.Equal(t, confidenceHigh, got[1].Confidence)
	unknownCode := xdr.ScErrorCodeScecArithDomain
	got = NewSuggestionEngine().AnalyzeEvents([]DecodedEvent{{
		ContractID: "contract", Data: "insufficient balance",
		Error: &xdr.ScError{Type: xdr.ScErrorTypeSceWasmVm, Code: &unknownCode},
	}})
	require.Len(t, got, 1)
	assert.Equal(t, "insufficient_balance", got[0].Rule)
}

func TestStructuredFieldsSurviveBothDecoderPaths(t *testing.T) {
	contract := xdr.ScBytes(make([]byte, 32))
	contract[0] = 1
	amount := xdr.Int128Parts{Lo: 10}
	args := xdr.ScVec{{Type: xdr.ScValTypeScvI128, I128: &amount}}
	argsPtr := &args
	code := xdr.ScErrorCodeScecInvalidAction
	errorValue := xdr.ScError{Type: xdr.ScErrorTypeSceAuth, Code: &code}
	body := []xdr.ContractEventV0{
		{Topics: []xdr.ScVal{structuredSymbol("fn_call"), {Type: xdr.ScValTypeScvBytes, Bytes: &contract}, structuredSymbol("transfer")}, Data: xdr.ScVal{Type: xdr.ScValTypeScvVec, Vec: &argsPtr}},
		{Topics: []xdr.ScVal{structuredSymbol("error"), {Type: xdr.ScValTypeScvError, Error: &errorValue}}, Data: structuredSymbol("reworded")},
		{Topics: []xdr.ScVal{structuredSymbol("fn_return"), structuredSymbol("transfer")}, Data: xdr.ScVal{Type: xdr.ScValTypeScvVoid}},
	}
	var legacy []string
	var simulated []simulator.DiagnosticEvent
	for _, b := range body {
		diag := xdr.DiagnosticEvent{Event: xdr.ContractEvent{Type: xdr.ContractEventTypeDiagnostic, Body: xdr.ContractEventBody{V: 0, V0: &b}}}
		encoded, err := diag.MarshalBinary()
		require.NoError(t, err)
		legacy = append(legacy, base64.StdEncoding.EncodeToString(encoded))
		event := simulator.DiagnosticEvent{Data: structuredXDR(t, b.Data)}
		for _, topic := range b.Topics {
			event.Topics = append(event.Topics, structuredXDR(t, topic))
		}
		simulated = append(simulated, event)
	}
	for _, decode := range []struct {
		name string
		fn   func() (*CallNode, error)
	}{
		{"legacy", func() (*CallNode, error) { return DecodeEvents(legacy, 0) }},
		{"simulator", func() (*CallNode, error) { return DecodeDiagnosticEvents(simulated, 0) }},
	} {
		t.Run(decode.name, func(t *testing.T) {
			root, err := decode.fn()
			require.NoError(t, err)
			require.Len(t, root.SubCalls, 1)
			call := root.SubCalls[0]
			assert.Equal(t, "transfer", call.Function)
			require.Len(t, call.Events, 3)
			assert.Equal(t, []xdr.ScValType{xdr.ScValTypeScvI128}, call.Events[0].ArgumentTypes)
			custom := &SuggestionEngine{rules: []ErrorPattern{{
				Name: "typed_transfer", Suggestion: Suggestion{Rule: "typed_transfer"},
				Structured: []DiagnosticCriteria{{Functions: []string{"transfer"}, ArgumentTypes: []xdr.ScValType{xdr.ScValTypeScvI128}}},
			}}}
			require.Len(t, custom.AnalyzeEvents(call.Events), 1)
			assert.Equal(t, "transfer", call.Events[1].ContractFunction)
			require.NotNil(t, call.Events[1].Error)
			assert.Equal(t, errorValue, *call.Events[1].Error)
			got := NewSuggestionEngine().AnalyzeEvents(call.Events)
			require.NotEmpty(t, got)
			assert.Contains(t, got, Suggestion{
				Rule: "missing_authorization", Confidence: confidenceHigh,
				Description: "Potential Fix: Verify that all required signatures are present and the invoker has proper authorization.",
			})
		})
	}
}

func TestDecoderFindsErrorsInData(t *testing.T) {
	code := xdr.ScErrorCodeScecInvalidAction
	errorValue := xdr.ScError{Type: xdr.ScErrorTypeSceAuth, Code: &code}
	value := xdr.ScVal{Type: xdr.ScValTypeScvError, Error: &errorValue}
	vector := xdr.ScVec{structuredSymbol("host message changed"), value}
	vectorPtr := &vector
	for _, data := range []xdr.ScVal{value, {Type: xdr.ScValTypeScvVec, Vec: &vectorPtr}} {
		root, err := DecodeDiagnosticEvents([]simulator.DiagnosticEvent{{Data: structuredXDR(t, data)}}, 0)
		require.NoError(t, err)
		require.Len(t, root.Events, 1)
		require.NotNil(t, root.Events[0].Error)
		assert.Equal(t, errorValue, *root.Events[0].Error)
		got := NewSuggestionEngine().AnalyzeEvents(root.Events)
		require.Len(t, got, 1)
		assert.Equal(t, "missing_authorization", got[0].Rule)
	}
}

func TestDecoderPreservesLegacyTextAndGas(t *testing.T) {
	cpuStart, cpuEnd, memStart, memEnd := uint64(10), uint64(30), uint64(20), uint64(50)
	root, err := DecodeDiagnosticEvents([]simulator.DiagnosticEvent{
		{Topics: []string{"fn_call", "transfer"}, CPU: &cpuStart, Memory: &memStart},
		{Topics: []string{"auth_failed"}, Data: "signature missing"},
		{Topics: []string{"fn_return", "transfer"}, CPU: &cpuEnd, Memory: &memEnd},
	}, 0)
	require.NoError(t, err)
	require.Len(t, root.SubCalls, 1)
	call := root.SubCalls[0]
	assert.Equal(t, uint64(20), call.CPUInstructions)
	assert.Equal(t, uint64(30), call.MemoryBytes)
	assert.Equal(t, "signature missing", call.Events[1].Data)
	assert.Equal(t, "transfer", call.Events[1].ContractFunction)
}

func TestDepthLimitUnwindsBeforeTheNextCall(t *testing.T) {
	root, err := DecodeDiagnosticEvents([]simulator.DiagnosticEvent{
		{Topics: []string{"fn_call", "outer"}},
		{Data: "event without topics"},
		{Topics: []string{"fn_call", "nested"}},
		{Topics: []string{"fn_call", "deeper"}},
		{Topics: []string{"fn_return", "deeper"}},
		{Topics: []string{"fn_return", "nested"}},
		{Topics: []string{"fn_return", "outer"}},
		{Topics: []string{"fn_call", "sibling"}},
		{Topics: []string{"fn_return", "sibling"}},
	}, 1)
	require.NoError(t, err)
	require.Len(t, root.SubCalls, 2)
	assert.Equal(t, "outer", root.SubCalls[0].Function)
	assert.Equal(t, "sibling", root.SubCalls[1].Function)
	assert.Empty(t, root.SubCalls[0].SubCalls)
}

func structuredSymbol(value string) xdr.ScVal {
	symbol := xdr.ScSymbol(value)
	return xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &symbol}
}

func structuredXDR(t *testing.T, value xdr.ScVal) string {
	t.Helper()
	encoded, err := value.MarshalBinary()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(encoded)
}
