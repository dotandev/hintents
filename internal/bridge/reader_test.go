// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package bridge

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func TestReadFrames_SnapshotsAndFinal(t *testing.T) {
	ndjson := strings.Join([]string{
		`{"type":"snapshot","seq":0,"data":{"entries":1}}`,
		`{"type":"snapshot","seq":1,"data":{"entries":2}}`,
		`{"type":"final","seq":2,"data":{"status":"success","events":[]}}`,
	}, "\n") + "\n"

	reader := NewFrameReader(strings.NewReader(ndjson))
	frames := make(chan StreamFrame, 10)

	finalData, err := reader.ReadFrames(context.Background(), frames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	close(frames)

	var snapshots []StreamFrame
	for f := range frames {
		snapshots = append(snapshots, f)
	}

	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshot frames, got %d", len(snapshots))
	}
	if snapshots[0].Seq != 0 {
		t.Errorf("expected seq 0, got %d", snapshots[0].Seq)
	}
	if snapshots[1].Seq != 1 {
		t.Errorf("expected seq 1, got %d", snapshots[1].Seq)
	}

	var finalResp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(finalData, &finalResp); err != nil {
		t.Fatalf("unmarshal final data: %v", err)
	}
	if finalResp.Status != "success" {
		t.Errorf("expected status success, got %q", finalResp.Status)
	}
}

func TestReadFrames_FinalOnly(t *testing.T) {
	ndjson := `{"type":"final","seq":0,"data":{"status":"success","events":[]}}` + "\n"

	reader := NewFrameReader(strings.NewReader(ndjson))
	frames := make(chan StreamFrame, 10)

	finalData, err := reader.ReadFrames(context.Background(), frames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	close(frames)

	var snapshots []StreamFrame
	for f := range frames {
		snapshots = append(snapshots, f)
	}
	if len(snapshots) != 0 {
		t.Errorf("expected no snapshot frames, got %d", len(snapshots))
	}

	if len(finalData) == 0 {
		t.Error("expected non-empty final data")
	}
}

func TestReadFrames_LegacyNonStreamingResponse(t *testing.T) {
	// Simulator that hasn't been updated yet emits a plain JSON object.
	legacy := `{"status":"success","events":["e1"]}` + "\n"

	reader := NewFrameReader(strings.NewReader(legacy))
	frames := make(chan StreamFrame, 10)

	finalData, err := reader.ReadFrames(context.Background(), frames)
	if err != nil {
		t.Fatalf("unexpected error for legacy response: %v", err)
	}
	close(frames)

	var resp struct {
		Status string   `json:"status"`
		Events []string `json:"events"`
	}
	if err := json.Unmarshal(finalData, &resp); err != nil {
		t.Fatalf("unmarshal legacy response: %v", err)
	}
	if resp.Status != "success" {
		t.Errorf("expected success, got %q", resp.Status)
	}
	if len(resp.Events) != 1 || resp.Events[0] != "e1" {
		t.Errorf("unexpected events: %v", resp.Events)
	}
}

func TestReadFrames_EOFWithoutFinal(t *testing.T) {
	ndjson := `{"type":"snapshot","seq":0,"data":{}}` + "\n"
	// No final frame.

	reader := NewFrameReader(strings.NewReader(ndjson))
	frames := make(chan StreamFrame, 10)

	_, err := reader.ReadFrames(context.Background(), frames)
	if err == nil {
		t.Fatal("expected error when final frame is missing")
	}
}

func TestReadFrames_ContextCancellation(t *testing.T) {
	// Simulate context cancellation by closing the pipe with the context error,
	// which is what exec.CommandContext does when it kills the subprocess.
	pr, pw := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Close the write end with the context error once the deadline passes.
	// This mimics the subprocess being killed, which EOF-closes its stdout pipe.
	go func() {
		<-ctx.Done()
		pw.CloseWithError(ctx.Err())
	}()

	reader := NewFrameReader(pr)
	frames := make(chan StreamFrame, 10)

	_, err := reader.ReadFrames(ctx, frames)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestReadFrames_EmptyLinesSkipped(t *testing.T) {
	ndjson := "\n\n" +
		`{"type":"snapshot","seq":0,"data":{}}` + "\n\n" +
		`{"type":"final","seq":1,"data":{"status":"ok"}}` + "\n"

	reader := NewFrameReader(strings.NewReader(ndjson))
	frames := make(chan StreamFrame, 10)

	finalData, err := reader.ReadFrames(context.Background(), frames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	close(frames)

	var snapshots []StreamFrame
	for f := range frames {
		snapshots = append(snapshots, f)
	}
	if len(snapshots) != 1 {
		t.Errorf("expected 1 snapshot frame, got %d", len(snapshots))
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(finalData, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected ok, got %q", resp.Status)
	}
}

func TestStreamFrame_Types(t *testing.T) {
	for _, tc := range []struct {
		ft   FrameType
		want string
	}{
		{FrameTypeSnapshot, "snapshot"},
		{FrameTypeFinal, "final"},
		{FrameTypeRollbackDiff, "rollbackdiff"},
	} {
		if string(tc.ft) != tc.want {
			t.Errorf("FrameType %q: expected %q", tc.ft, tc.want)
		}
	}
}

func TestReadFrames_RollbackDiffForwardedToChannel(t *testing.T) {
	// A stream with one rollback-diff frame followed by the final frame.
	ndjson := strings.Join([]string{
		`{"type":"rollbackdiff","seq":1,"data":{"rollback_to_seq":0,"entries":[{"address":"0x00001000","old_value":99,"new_value":42}]}}`,
		`{"type":"final","seq":2,"data":{"status":"success"}}`,
	}, "\n") + "\n"

	reader := NewFrameReader(strings.NewReader(ndjson))
	frames := make(chan StreamFrame, 10)

	finalData, err := reader.ReadFrames(context.Background(), frames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	close(frames)

	var collected []StreamFrame
	for f := range frames {
		collected = append(collected, f)
	}

	if len(collected) != 1 {
		t.Fatalf("expected 1 rollbackdiff frame, got %d", len(collected))
	}

	frame := collected[0]
	if frame.Type != FrameTypeRollbackDiff {
		t.Errorf("expected rollbackdiff frame type, got %q", frame.Type)
	}
	if frame.Seq != 1 {
		t.Errorf("expected seq 1, got %d", frame.Seq)
	}

	var diff MemoryDiff
	if err := json.Unmarshal(frame.Data, &diff); err != nil {
		t.Fatalf("unmarshal MemoryDiff: %v", err)
	}
	if diff.RollbackToSeq != 0 {
		t.Errorf("expected rollback_to_seq 0, got %d", diff.RollbackToSeq)
	}
	if len(diff.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(diff.Entries))
	}
	e := diff.Entries[0]
	if e.Address != "0x00001000" {
		t.Errorf("unexpected address %q", e.Address)
	}
	if e.OldValue != 99 {
		t.Errorf("expected old_value 99, got %d", e.OldValue)
	}
	if e.NewValue != 42 {
		t.Errorf("expected new_value 42, got %d", e.NewValue)
	}

	// Final frame payload should still be returned correctly.
	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(finalData, &resp); err != nil {
		t.Fatalf("unmarshal final: %v", err)
	}
	if resp.Status != "success" {
		t.Errorf("expected success, got %q", resp.Status)
	}
}

func TestReadFrames_RollbackDiffEmptyEntries(t *testing.T) {
	// A no-op rollback (no memory changed) should still be forwarded.
	ndjson := strings.Join([]string{
		`{"type":"rollbackdiff","seq":0,"data":{"rollback_to_seq":3,"entries":[]}}`,
		`{"type":"final","seq":1,"data":{"status":"ok"}}`,
	}, "\n") + "\n"

	reader := NewFrameReader(strings.NewReader(ndjson))
	frames := make(chan StreamFrame, 10)

	_, err := reader.ReadFrames(context.Background(), frames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	close(frames)

	var collected []StreamFrame
	for f := range frames {
		collected = append(collected, f)
	}
	if len(collected) != 1 {
		t.Fatalf("expected 1 rollbackdiff frame, got %d", len(collected))
	}

	var diff MemoryDiff
	if err := json.Unmarshal(collected[0].Data, &diff); err != nil {
		t.Fatalf("unmarshal empty MemoryDiff: %v", err)
	}
	if diff.RollbackToSeq != 3 {
		t.Errorf("expected rollback_to_seq 3, got %d", diff.RollbackToSeq)
	}
	if len(diff.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(diff.Entries))
	}
}

func TestReadFrames_RollbackDiffMalformedPayloadReturnsError(t *testing.T) {
	// A rollbackdiff frame with invalid data should return a decode error.
	ndjson := `{"type":"rollbackdiff","seq":0,"data":"not-an-object"}` + "\n"

	reader := NewFrameReader(strings.NewReader(ndjson))
	frames := make(chan StreamFrame, 10)

	_, err := reader.ReadFrames(context.Background(), frames)
	if err == nil {
		t.Fatal("expected error for malformed rollbackdiff payload")
	}
}

func TestReadFrames_MixedSnapshotAndRollbackDiff(t *testing.T) {
	// Verify that snapshot and rollbackdiff frames can be interleaved and are
	// both forwarded on the frames channel in order.
	ndjson := strings.Join([]string{
		`{"type":"snapshot","seq":0,"data":{"ledger":0}}`,
		`{"type":"rollbackdiff","seq":1,"data":{"rollback_to_seq":0,"entries":[{"address":"0x0000","old_value":1,"new_value":0}]}}`,
		`{"type":"snapshot","seq":2,"data":{"ledger":1}}`,
		`{"type":"final","seq":3,"data":{"status":"success"}}`,
	}, "\n") + "\n"

	reader := NewFrameReader(strings.NewReader(ndjson))
	frames := make(chan StreamFrame, 10)

	_, err := reader.ReadFrames(context.Background(), frames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	close(frames)

	var collected []StreamFrame
	for f := range frames {
		collected = append(collected, f)
	}
	if len(collected) != 3 {
		t.Fatalf("expected 3 frames (2 snapshots + 1 rollbackdiff), got %d", len(collected))
	}
	if collected[0].Type != FrameTypeSnapshot {
		t.Errorf("frame 0: expected snapshot, got %q", collected[0].Type)
	}
	if collected[1].Type != FrameTypeRollbackDiff {
		t.Errorf("frame 1: expected rollbackdiff, got %q", collected[1].Type)
	}
	if collected[2].Type != FrameTypeSnapshot {
		t.Errorf("frame 2: expected snapshot, got %q", collected[2].Type)
	}
}
