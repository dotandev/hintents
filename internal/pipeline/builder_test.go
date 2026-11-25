// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuilder_RoundTrip(t *testing.T) {
	builder := NewBuilder()
	builder.AddCommand("MoveCall", "0x2::coin::mint", []string{"1000", "0xUser"})
	builder.AddCommand("TransferObjects", "0xUser", []string{"obj1", "obj2"})

	jsonStr, err := builder.ToJSON()
	require.NoError(t, err)
	require.NotEmpty(t, jsonStr)

	parsed, err := FromJSON([]byte(jsonStr))
	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.Equal(t, builder, parsed)
}

func TestFromJSON_Invalid(t *testing.T) {
	invalidJSON := []byte(`{ invalid json `)
	parsed, err := FromJSON(invalidJSON)
	require.Error(t, err)
	assert.Nil(t, parsed)
	assert.Contains(t, err.Error(), "failed to parse pipeline JSON")
}
