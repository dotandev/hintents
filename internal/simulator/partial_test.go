// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"testing"

	interrors "github.com/dotandev/hintents/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimulatePartial_HaltsOnMissingKey(t *testing.T) {
	req := &SimulationRequest{
		EnvelopeXdr:   "envelope-xdr",
		ResultMetaXdr: "result-meta-xdr",
		LedgerEntries: map[string]string{
			"key-a": "value-a",
			"key-b": "value-b",
		},
	}

	state := map[string]string{
		"key-a": "value-a",
		// "key-b" intentionally absent
	}

	result, err := SimulatePartial(req, state)

	require.Error(t, err)
	var missingErr *interrors.MissingLedgerKeyError
	require.ErrorAs(t, err, &missingErr)
	assert.Equal(t, "key-b", missingErr.Key)
	assert.False(t, result.Completed)
	assert.Equal(t, "key-b", result.HaltedAtKey)
	assert.True(t, interrors.Is(err, interrors.ErrMissingLedgerKey))
}

func TestSimulatePartial_CompletesWhenAllKeysPresent(t *testing.T) {
	req := &SimulationRequest{
		EnvelopeXdr:   "envelope-xdr",
		ResultMetaXdr: "result-meta-xdr",
		LedgerEntries: map[string]string{
			"key-a": "value-a",
			"key-b": "value-b",
		},
	}

	state := map[string]string{
		"key-a": "value-a",
		"key-b": "value-b",
	}

	result, err := SimulatePartial(req, state)

	require.NoError(t, err)
	assert.True(t, result.Completed)
	assert.Empty(t, result.HaltedAtKey)
	assert.Equal(t, 2, result.OpsApplied)
}

func TestSimulatePartial_EmptyLedgerEntriesCompletes(t *testing.T) {
	req := &SimulationRequest{
		EnvelopeXdr:   "envelope-xdr",
		ResultMetaXdr: "result-meta-xdr",
		LedgerEntries: map[string]string{},
	}

	result, err := SimulatePartial(req, map[string]string{})

	require.NoError(t, err)
	assert.True(t, result.Completed)
	assert.Equal(t, 0, result.OpsApplied)
}

func TestSimulatePartial_NilRequestReturnsValidationError(t *testing.T) {
	result, err := SimulatePartial(nil, map[string]string{})

	require.Error(t, err)
	assert.False(t, result.Completed)
	assert.True(t, interrors.Is(err, interrors.ErrValidationFailed))
}

func TestSimulatePartial_AllKeysMissingHaltsAtFirst(t *testing.T) {
	req := &SimulationRequest{
		EnvelopeXdr:   "envelope-xdr",
		ResultMetaXdr: "result-meta-xdr",
		LedgerEntries: map[string]string{
			"key-a": "value-a",
		},
	}

	result, err := SimulatePartial(req, map[string]string{})

	require.Error(t, err)
	var missingErr *interrors.MissingLedgerKeyError
	require.ErrorAs(t, err, &missingErr)
	assert.False(t, result.Completed)
	assert.Equal(t, 0, result.OpsApplied)
}
