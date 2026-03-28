// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalJSON_KeyOrdering(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected string
	}{
		{
			name: "keys sorted alphabetically",
			input: map[string]interface{}{
				"zebra":  "last",
				"apple":  "first",
				"middle": "middle",
				"banana": "second",
			},
			expected: `{"apple":"first","banana":"second","middle":"middle","zebra":"last"}`,
		},
		{
			name: "nested objects with sorted keys",
			input: map[string]interface{}{
				"outer_z": map[string]interface{}{
					"inner_z": "value1",
					"inner_a": "value2",
				},
				"outer_a": "simple",
			},
			expected: `{"outer_a":"simple","outer_z":{"inner_a":"value2","inner_z":"value1"}}`,
		},
		{
			name:     "empty object",
			input:    map[string]interface{}{},
			expected: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := canonicalJSON(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestCanonicalJSON_Arrays(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "simple array",
			input:    []interface{}{"a", "b", "c"},
			expected: `["a","b","c"]`,
		},
		{
			name: "array with objects",
			input: []interface{}{
				map[string]interface{}{"z": 1, "a": 2},
				map[string]interface{}{"b": 3},
			},
			expected: `[{"a":2,"z":1},{"b":3}]`,
		},
		{
			name:     "empty array",
			input:    []interface{}{},
			expected: `[]`,
		},
		{
			name:     "nested arrays",
			input:    []interface{}{[]interface{}{1, 2}, []interface{}{3, 4}},
			expected: `[[1,2],[3,4]]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := canonicalJSON(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestCanonicalJSON_DataTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string",
			input:    "hello",
			expected: `"hello"`,
		},
		{
			name:     "number",
			input:    42.5,
			expected: `42.5`,
		},
		{
			name:     "boolean true",
			input:    true,
			expected: `true`,
		},
		{
			name:     "boolean false",
			input:    false,
			expected: `false`,
		},
		{
			name:     "null",
			input:    nil,
			expected: `null`,
		},
		{
			name:     "string with special characters",
			input:    "hello\nworld\t\"quoted\"",
			expected: `"hello\nworld\t\"quoted\""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := canonicalJSON(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestMarshalCanonical_Struct(t *testing.T) {
	type TestStruct struct {
		Zebra  string   `json:"zebra"`
		Apple  string   `json:"apple"`
		Events []string `json:"events"`
		Count  int      `json:"count"`
	}

	input := TestStruct{
		Zebra:  "last",
		Apple:  "first",
		Events: []string{"event1", "event2"},
		Count:  42,
	}

	result, err := marshalCanonical(input)
	require.NoError(t, err)

	expected := `{"apple":"first","count":42,"events":["event1","event2"],"zebra":"last"}`
	assert.Equal(t, expected, string(result))
}

func TestMarshalCanonical_Payload(t *testing.T) {
	payload := Payload{
		EnvelopeXdr:   "envelope_data",
		ResultMetaXdr: "result_data",
		Events:        []string{"event1", "event2"},
		Logs:          []string{"log1"},
	}

	result, err := marshalCanonical(payload)
	require.NoError(t, err)

	// Verify it's valid JSON
	var decoded map[string]interface{}
	err = json.Unmarshal(result, &decoded)
	require.NoError(t, err)

	// Verify keys are in alphabetical order
	expected := `{"envelope_xdr":"envelope_data","events":["event1","event2"],"logs":["log1"],"result_meta_xdr":"result_data"}`
	assert.Equal(t, expected, string(result))
}

func TestCanonicalJSON_Deterministic(t *testing.T) {
	// Test that multiple serializations produce identical output
	input := map[string]interface{}{
		"z_field": "value1",
		"a_field": "value2",
		"m_field": map[string]interface{}{
			"nested_z": 1,
			"nested_a": 2,
		},
		"array": []interface{}{3, 2, 1},
	}

	// Serialize multiple times
	results := make([][]byte, 10)
	for i := 0; i < 10; i++ {
		result, err := canonicalJSON(input)
		require.NoError(t, err)
		results[i] = result
	}

	// All results should be identical
	for i := 1; i < len(results); i++ {
		assert.Equal(t, results[0], results[i], "serialization %d differs from first", i)
	}
}

func TestCanonicalJSON_ComplexNesting(t *testing.T) {
	input := map[string]interface{}{
		"level1_z": map[string]interface{}{
			"level2_z": map[string]interface{}{
				"level3_z": "deep_value",
				"level3_a": "another_value",
			},
			"level2_a": []interface{}{
				map[string]interface{}{
					"item_z": 1,
					"item_a": 2,
				},
			},
		},
		"level1_a": "simple",
	}

	result, err := canonicalJSON(input)
	require.NoError(t, err)

	expected := `{"level1_a":"simple","level1_z":{"level2_a":[{"item_a":2,"item_z":1}],"level2_z":{"level3_a":"another_value","level3_z":"deep_value"}}}`
	assert.Equal(t, expected, string(result))
}

func TestCanonicalJSON_EmptyValues(t *testing.T) {
	input := map[string]interface{}{
		"empty_string": "",
		"empty_array":  []interface{}{},
		"empty_object": map[string]interface{}{},
		"zero_number":  0,
		"null_value":   nil,
	}

	result, err := canonicalJSON(input)
	require.NoError(t, err)

	expected := `{"empty_array":[],"empty_object":{},"empty_string":"","null_value":null,"zero_number":0}`
	assert.Equal(t, expected, string(result))
}
