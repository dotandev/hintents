package simulator

import (
	"testing"
	"github.com/stretchr/testify/assert"
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
