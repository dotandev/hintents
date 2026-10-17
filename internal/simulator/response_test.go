// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/json"
	"testing"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/stretchr/testify/assert"
)

func TestGetDiagnosticEventsByContractID(t *testing.T) {
	cid1 := "contract1"
	cid2 := "contract2"

	resp := &SimulationResponse{
		DiagnosticEvents: []DiagnosticEvent{
			{ContractID: &cid1, EventType: "test_event"},
			{ContractID: &cid2, EventType: "another_event"},
			{ContractID: nil, EventType: "nil_contract"},
			{ContractID: &cid1, EventType: "test_event_2"},
		},
	}

	results := resp.GetDiagnosticEventsByContractID("contract1")
	assert.Len(t, results, 2)
	assert.Equal(t, "test_event", results[0].EventType)
	assert.Equal(t, "test_event_2", results[1].EventType)

	results2 := resp.GetDiagnosticEventsByContractID("contract2")
	assert.Len(t, results2, 1)

	resultsMiss := resp.GetDiagnosticEventsByContractID("nonexistent")
	assert.Empty(t, resultsMiss)
}

func TestGetDiagnosticEventsByTopic(t *testing.T) {
	resp := &SimulationResponse{
		DiagnosticEvents: []DiagnosticEvent{
			{Topics: []string{"topicA", "topicB"}, EventType: "event1"},
			{Topics: []string{"topicC"}, EventType: "event2"},
			{Topics: []string{"topicA", "topicD"}, EventType: "event3"},
			{Topics: nil, EventType: "event4"},
		},
	}

	resultsA := resp.GetDiagnosticEventsByTopic("topicA")
	assert.Len(t, resultsA, 2)
	assert.Equal(t, "event1", resultsA[0].EventType)
	assert.Equal(t, "event3", resultsA[1].EventType)

	resultsC := resp.GetDiagnosticEventsByTopic("topicC")
	assert.Len(t, resultsC, 1)
	assert.Equal(t, "event2", resultsC[0].EventType)

	resultsMiss := resp.GetDiagnosticEventsByTopic("topicMissing")
	assert.Empty(t, resultsMiss)
}

func TestSimulationResponseUnmarshalSnapshots(t *testing.T) {
	payload := []byte(`{
		"status":"ok",
		"optimization_report": {
			"snapshots":{
				"inline":{
					"post-run":{
						"ledger_entries":[["a2V5","dmFsdWU="]],
						"linear_memory":"AQID"
					}
				},
				"ids":["snap-2"]
			}
		}
	}`)

	var resp SimulationResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		t.Fatalf("expected no error unmarshalling snapshots, got %v", err)
	}
	if resp.OptimizationReport.Snapshots == nil {
		t.Fatal("expected snapshots payload")
	}
	if _, ok := resp.OptimizationReport.Snapshots.Inline["post-run"]; !ok {
		t.Fatal("expected inline snapshot with key post-run")
	}
	if got := len(resp.OptimizationReport.Snapshots.IDs); got != 1 {
		t.Fatalf("expected 1 lazy snapshot id, got %d", got)
	}
}

func TestSimulationResponseClassifyError(t *testing.T) {
	// Success cases
	t.Run("nil response", func(t *testing.T) {
		var resp *SimulationResponse
		assert.Nil(t, resp.ClassifyError())
	})

	t.Run("success status", func(t *testing.T) {
		resp := &SimulationResponse{Status: "ok"}
		assert.Nil(t, resp.ClassifyError())

		respSuccess := &SimulationResponse{Status: "success"}
		assert.Nil(t, respSuccess.ClassifyError())
	})

	// Error classification test cases
	tests := []struct {
		name         string
		response     SimulationResponse
		expectedCode errors.ErstErrorCode
	}{
		{
			name: "trap by raw code",
			response: SimulationResponse{
				Status:    "error",
				ErrorCode: "WASM_TRAP",
				Error:     "contract execution halted",
			},
			expectedCode: errors.ErstSimCrash,
		},
		{
			name: "trap by message unreachable",
			response: SimulationResponse{
				Status: "error",
				Error:  "wasm trap: unreachable instruction executed",
			},
			expectedCode: errors.ErstSimCrash,
		},
		{
			name: "timeout by raw code",
			response: SimulationResponse{
				Status:    "error",
				ErrorCode: "TIMEOUT",
				Error:     "execution deadline exceeded",
			},
			expectedCode: errors.ErstRPCTimeout,
		},
		{
			name: "timeout by message",
			response: SimulationResponse{
				Status: "error",
				Error:  "simulator execution timed out after 5000ms",
			},
			expectedCode: errors.ErstRPCTimeout,
		},
		{
			name: "oom by raw code",
			response: SimulationResponse{
				Status:    "error",
				ErrorCode: "OUT_OF_MEMORY",
			},
			expectedCode: errors.ErstSimulationFailed,
		},
		{
			name: "oom by message",
			response: SimulationResponse{
				Status: "error",
				Error:  "wasm memory limit exceeded (allocating 1048576 bytes)",
			},
			expectedCode: errors.ErstSimulationFailed,
		},
		{
			name: "decode failure by raw code",
			response: SimulationResponse{
				Status:    "error",
				ErrorCode: "DECODE_FAILURE",
			},
			expectedCode: errors.ErstValidationFailed,
		},
		{
			name: "decode failure by message",
			response: SimulationResponse{
				Status: "error",
				Error:  "malformed XDR payload in transaction envelope",
			},
			expectedCode: errors.ErstValidationFailed,
		},
		{
			name: "generic error",
			response: SimulationResponse{
				Status: "error",
				Error:  "internal runtime assertion failed",
			},
			expectedCode: errors.ErstSimulationFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.response.ClassifyError()
			assert.NotNil(t, err)
			assert.Equal(t, tc.expectedCode, err.Code)
		})
	}
}

func TestParseSimulationResponse(t *testing.T) {
	t.Run("empty bytes error", func(t *testing.T) {
		_, err := ParseSimulationResponse(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty simulation response")
	})

	t.Run("invalid json error", func(t *testing.T) {
		_, err := ParseSimulationResponse([]byte("not valid json"))
		assert.Error(t, err)
	})

	t.Run("valid payload success", func(t *testing.T) {
		payload := []byte(`{"status":"ok","events":["e1","e2"]}`)
		resp, err := ParseSimulationResponse(payload)
		assert.NoError(t, err)
		assert.Equal(t, "ok", resp.Status)
		assert.Len(t, resp.Events, 2)
	})
}

