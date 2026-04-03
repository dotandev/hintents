package simulator

import (
	"testing"

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
// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/json"
	"testing"
)

func TestSimulationResponseUnmarshalSnapshots(t *testing.T) {
	payload := []byte(`{
		"status":"ok",
		"snapshots":{
			"inline":{
				"post-run":{
					"ledger_entries":[["a2V5","dmFsdWU="]],
					"linear_memory":"AQID"
				}
			},
			"ids":["snap-2"]
		}
	}`)

	var resp SimulationResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		t.Fatalf("expected no error unmarshalling snapshots, got %v", err)
	}
	if resp.Snapshots == nil {
		t.Fatal("expected snapshots payload")
	}
	if _, ok := resp.Snapshots.Inline["post-run"]; !ok {
		t.Fatal("expected inline snapshot with key post-run")
	}
	if got := len(resp.Snapshots.IDs); got != 1 {
		t.Fatalf("expected 1 lazy snapshot id, got %d", got)
	}
}
