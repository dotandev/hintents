package simulator

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestProfilingSchema(t *testing.T) {
	req := SimulationRequest{
		EnvelopeXdr: "AAAA...",
		Profile:     true,
	}
	assert.True(t, req.Profile)

	resp := SimulationResponse{
		Status:     "success",
		Flamegraph: "<svg>...</svg>",
	}
	assert.Equal(t, "success", resp.Status)
	assert.NotEmpty(t, resp.Flamegraph)
}
