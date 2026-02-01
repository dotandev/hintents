// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTimeTravelSchema(t *testing.T) {
	req := SimulationRequest{
		EnvelopeXdr:    "AAAA...",
		Timestamp:      1738077842,
		LedgerSequence: 1234,
	}
	assert.Equal(t, int64(1738077842), req.Timestamp)
	assert.Equal(t, uint32(1234), req.LedgerSequence)
}
