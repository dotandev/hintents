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
	} {
		if string(tc.ft) != tc.want {
			t.Errorf("FrameType %q: expected %q", tc.ft, tc.want)
		}
	}
}
