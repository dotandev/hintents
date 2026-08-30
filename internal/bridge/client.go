// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package bridge wires snapshot compression into the IPC request pipeline.
// CompressRequest replaces the plain ledger_entries map with a Zstd-compressed,
// base64-encoded blob in ledger_entries_zstd so the Rust simulator can detect
// and decompress it automatically.
package bridge

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// ipcRequest is a minimal view of the simulator.SimulationRequest used for
// compression surgery without importing the simulator package (avoids cycles).
type ipcRequest struct {
	LedgerEntries     map[string]string `json:"ledger_entries,omitempty"`
	LedgerEntriesZstd string            `json:"ledger_entries_zstd,omitempty"`
	ControlCommand    string            `json:"control_command,omitempty"`
	RewindStep        *int              `json:"rewind_step,omitempty"`
	ForkParams        map[string]string `json:"fork_params,omitempty"`
	HarnessReset      bool              `json:"harness_reset,omitempty"`
}

const (
	// CommandRollbackAndResume requests simulator rollback to rewind_step and
	// immediate resumed execution using optional fork parameters.
	CommandRollbackAndResume = "ROLLBACK_AND_RESUME"
)

// CompressRequest takes the raw JSON bytes of a SimulationRequest, compresses
// the ledger_entries map with Zstd, and returns updated JSON bytes.
// If ledger_entries is absent or empty the input is returned unchanged.
func CompressRequest(reqJSON []byte) ([]byte, error) {
	// Unmarshal only the fields we care about.
	var partial ipcRequest
	if err := json.Unmarshal(reqJSON, &partial); err != nil {
		return nil, fmt.Errorf("bridge: unmarshal for compression: %w", err)
	}

	if len(partial.LedgerEntries) == 0 {
		return reqJSON, nil
	}

	compressed, err := CompressLedgerEntries(partial.LedgerEntries)
	if err != nil {
		return nil, err
	}

	// Patch the raw JSON: remove ledger_entries, inject ledger_entries_zstd.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(reqJSON, &raw); err != nil {
		return nil, fmt.Errorf("bridge: unmarshal raw map: %w", err)
	}

	delete(raw, "ledger_entries")

	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString(compressed))
	if err != nil {
		return nil, fmt.Errorf("bridge: marshal zstd field: %w", err)
	}
	raw["ledger_entries_zstd"] = encoded

	out, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("bridge: re-marshal compressed request: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// FETCH_SNAPSHOT command — bidirectional IPC (Go bridge → simulator → Go)
// ---------------------------------------------------------------------------

// CommandOpcode identifies the command sent to the simulator over stdin.
type CommandOpcode string

const (
	// OpFetchSnapshot requests one or more snapshot frames by sequence ID.
	// The simulator responds with a "fetch_response" NDJSON frame on stdout.
	OpFetchSnapshot CommandOpcode = "FETCH_SNAPSHOT"
)

// fetchSnapshotRequest is the JSON payload written to the simulator's stdin.
//
// Wire format:
//
//	{"op":"FETCH_SNAPSHOT","id":3,"batch_size":5}
type fetchSnapshotRequest struct {
	Op        CommandOpcode `json:"op"`
	ID        uint32        `json:"id"`
	BatchSize uint32        `json:"batch_size,omitempty"`
}

// SnapshotEntry is one snapshot frame inside a FetchSnapshotResponse.
type SnapshotEntry struct {
	// Seq is the original sequence number of this snapshot.
	Seq uint32 `json:"seq"`
	// Data is the raw ledger snapshot payload.
	Data json.RawMessage `json:"data"`
}

// FetchSnapshotResponse is the parsed payload of a "fetch_response" frame
// emitted by the simulator on stdout.
//
// Wire format:
//
//	{"type":"fetch_response","seq":3,"data":{"snapshots":[...]}}
type FetchSnapshotResponse struct {
	FrameType string `json:"type"`
	// Seq echoes the requested starting sequence ID.
	Seq  uint32 `json:"seq"`
	Data struct {
		Snapshots []SnapshotEntry `json:"snapshots"`
	} `json:"data"`
}

// FetchSnapshot sends a FETCH_SNAPSHOT command for a single frame to the
// simulator's stdin pipe w.
//
// The caller is responsible for reading the corresponding "fetch_response"
// frame from the simulator's stdout stream.
func FetchSnapshot(w io.Writer, id uint32) error {
	return writeCommand(w, fetchSnapshotRequest{
		Op:        OpFetchSnapshot,
		ID:        id,
		BatchSize: 1,
	})
}

// FetchSnapshotBatch sends a FETCH_SNAPSHOT command requesting up to count
// consecutive frames starting at id.
//
// The simulator caps batch_size at 5; requesting more is safe but the response
// will contain at most 5 entries.
//
// The caller is responsible for reading the corresponding "fetch_response"
// frame from the simulator's stdout stream.
func FetchSnapshotBatch(w io.Writer, id uint32, count uint32) error {
	if count == 0 {
		count = 1
	}
	return writeCommand(w, fetchSnapshotRequest{
		Op:        OpFetchSnapshot,
		ID:        id,
		BatchSize: count,
	})
}

// writeCommand serialises cmd to a single NDJSON line and writes it to w.
func writeCommand(w io.Writer, cmd fetchSnapshotRequest) error {
	b, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("bridge: marshal command: %w", err)
	}
	b = append(b, '\n')
	if _, err := w.Write(b); err != nil {
		return fmt.Errorf("bridge: write command to simulator stdin: %w", err)
	}
	return nil
}

// WithRollbackAndResume injects a rollback-and-resume control command into a
// simulation request JSON payload.
func WithRollbackAndResume(reqJSON []byte, rewindStep int, forkParams map[string]string, harnessReset bool) ([]byte, error) {
	if rewindStep < 0 {
		return nil, fmt.Errorf("bridge: rewind step must be >= 0")
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(reqJSON, &raw); err != nil {
		return nil, fmt.Errorf("bridge: unmarshal raw map: %w", err)
	}

	commandJSON, err := json.Marshal(CommandRollbackAndResume)
	if err != nil {
		return nil, fmt.Errorf("bridge: marshal control_command: %w", err)
	}
	rewindJSON, err := json.Marshal(rewindStep)
	if err != nil {
		return nil, fmt.Errorf("bridge: marshal rewind_step: %w", err)
	}

	raw["control_command"] = commandJSON
	raw["rewind_step"] = rewindJSON

	if len(forkParams) > 0 {
		paramsJSON, marshalErr := json.Marshal(forkParams)
		if marshalErr != nil {
			return nil, fmt.Errorf("bridge: marshal fork_params: %w", marshalErr)
		}
		raw["fork_params"] = paramsJSON
	} else {
		delete(raw, "fork_params")
	}

	harnessJSON, err := json.Marshal(harnessReset)
	if err != nil {
		return nil, fmt.Errorf("bridge: marshal harness_reset: %w", err)
	}
	raw["harness_reset"] = harnessJSON

	out, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("bridge: re-marshal rollback request: %w", err)
	}

	return out, nil
}
