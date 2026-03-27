// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/ipc"
)

func TestDecodeSimulationResponseStreamPlainJSON(t *testing.T) {
	input := `{"status":"success","events":["a","b"]}` + "\n"

	resp, err := decodeSimulationResponseStream(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %s", resp.Status)
	}
	if len(resp.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(resp.Events))
	}
}

func TestDecodeSimulationResponseStreamChunked(t *testing.T) {
	original := SimulationResponse{
		Status: "success",
		Events: []string{strings.Repeat("event-", 40000)},
	}
	payload, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	chunkSize := 1024
	total := (len(payload) + chunkSize - 1) / chunkSize
	var builder strings.Builder
	writeFrame := func(frame ipc.SimulationStreamFrame) {
		line, err := json.Marshal(frame)
		if err != nil {
			t.Fatalf("marshal frame failed: %v", err)
		}
		builder.Write(line)
		builder.WriteByte('\n')
	}

	writeFrame(ipc.SimulationStreamFrame{
		Kind:       ipc.ChunkFrameStart,
		StreamID:   "stream-1",
		Total:      total,
		ChunkBytes: chunkSize,
		TimeoutMs:  100,
		RetryLimit: 1,
	})
	for index, offset := 0, 0; offset < len(payload); index, offset = index+1, offset+chunkSize {
		end := offset + chunkSize
		if end > len(payload) {
			end = len(payload)
		}
		writeFrame(ipc.SimulationStreamFrame{
			Kind:       ipc.ChunkFrameData,
			StreamID:   "stream-1",
			Index:      index,
			DataBase64: base64.StdEncoding.EncodeToString(payload[offset:end]),
		})
	}
	writeFrame(ipc.SimulationStreamFrame{
		Kind:       ipc.ChunkFrameEnd,
		StreamID:   "stream-1",
		Total:      total,
		TotalBytes: int64(len(payload)),
	})

	resp, err := decodeSimulationResponseStream(context.Background(), strings.NewReader(builder.String()))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if resp.Status != original.Status {
		t.Fatalf("expected status %s, got %s", original.Status, resp.Status)
	}
	if len(resp.Events) != 1 || resp.Events[0] != original.Events[0] {
		t.Fatalf("chunked payload was not reassembled correctly")
	}
}

func TestDecodeSimulationResponseStreamMissingChunkIsRetryable(t *testing.T) {
	start, _ := json.Marshal(ipc.SimulationStreamFrame{
		Kind:       ipc.ChunkFrameStart,
		StreamID:   "stream-2",
		Total:      2,
		ChunkBytes: 16,
		TimeoutMs:  100,
		RetryLimit: 2,
	})
	chunk, _ := json.Marshal(ipc.SimulationStreamFrame{
		Kind:       ipc.ChunkFrameData,
		StreamID:   "stream-2",
		Index:      1,
		DataBase64: base64.StdEncoding.EncodeToString([]byte(`{"status":"success"}`)),
	})

	_, err := decodeSimulationResponseStream(
		context.Background(),
		strings.NewReader(fmt.Sprintf("%s\n%s\n", start, chunk)),
	)
	if err == nil {
		t.Fatal("expected error")
	}
	chunkErr, ok := err.(*chunkStreamError)
	if !ok {
		t.Fatalf("expected chunkStreamError, got %T", err)
	}
	if chunkErr.retryLimit != 2 {
		t.Fatalf("expected retryLimit 2, got %d", chunkErr.retryLimit)
	}
}
