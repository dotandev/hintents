// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package bridge provides a streaming interface between the Go CLI and the
// Rust simulator subprocess.  The simulator emits newline-delimited JSON
// (NDJSON) frames to stdout; this package reads those frames in a background
// goroutine so that the UI can start rendering snapshot data before the full
// simulation has completed, reducing Time-to-First-Interactive.
package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// FrameType is the discriminator field written by the Rust simulator into every
// NDJSON line it emits.
type FrameType string

const (
	// FrameTypeSnapshot is an intermediate ledger-snapshot frame produced
	// while the simulation is still running.
	FrameTypeSnapshot FrameType = "snapshot"

	// FrameTypeFinal is the terminal frame whose Data field contains the
	// complete SimulationResponse JSON object.
	FrameTypeFinal FrameType = "final"
)

// StreamFrame is one NDJSON line emitted by the simulator subprocess.
type StreamFrame struct {
	// Type discriminates snapshot frames from the terminal final frame.
	Type FrameType `json:"type"`
	// Seq is a monotonically increasing sequence number (0-based) within
	// a single simulation run.  Out-of-order delivery is possible when the
	// simulator is extended to use concurrent goroutines; callers that care
	// about ordering should sort by Seq before processing.
	Seq uint32 `json:"seq"`
	// Data holds the frame payload as raw JSON so that callers can decode
	// it into the appropriate concrete type without a second allocation.
	Data json.RawMessage `json:"data"`
}

// FrameReader reads NDJSON StreamFrames from an io.Reader.
type FrameReader struct {
	r io.Reader
}

// NewFrameReader constructs a FrameReader that reads from r.
func NewFrameReader(r io.Reader) *FrameReader {
	return &FrameReader{r: r}
}

// ReadFrames scans r line by line until either the final frame arrives, ctx is
// cancelled, or r is exhausted.
//
// Each FrameTypeSnapshot frame is forwarded to frames. When FrameTypeFinal is
// encountered the raw JSON payload is returned so the caller can decode it into
// a *simulator.SimulationResponse.
//
// For backward compatibility, a line that does not carry a recognised "type"
// field but does contain a top-level "status" key is treated as a legacy
// (non-streaming) final response and returned as-is.
//
// The caller is responsible for closing frames after ReadFrames returns.
func (fr *FrameReader) ReadFrames(ctx context.Context, frames chan<- StreamFrame) (json.RawMessage, error) {
	scanner := bufio.NewScanner(fr.r)
	// Allow individual lines up to 16 MiB; large simulation responses can
	// easily exceed the 64 KiB bufio default.
	const maxLineBuf = 16 * 1024 * 1024
	scanner.Buffer(make([]byte, 64*1024), maxLineBuf)

	for {
		// Check for cancellation before blocking on the next line.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return nil, fmt.Errorf("bridge: reading simulator stdout: %w", err)
			}
			// EOF – could be caused by context cancellation (subprocess killed)
			// or by a simulator that exited without emitting a final frame.
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("bridge: simulator stdout closed without a final frame")
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Fast-path: peek at the "type" field without a full unmarshal.
		var envelope struct {
			Type FrameType       `json:"type"`
			Seq  uint32          `json:"seq"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			return nil, fmt.Errorf("bridge: unmarshal frame: %w", err)
		}

		switch envelope.Type {
		case FrameTypeFinal:
			return envelope.Data, nil

		case FrameTypeSnapshot:
			frame := StreamFrame{
				Type: FrameTypeSnapshot,
				Seq:  envelope.Seq,
				Data: envelope.Data,
			}
			select {
			case frames <- frame:
			case <-ctx.Done():
				return nil, ctx.Err()
			}

		default:
			// The "type" field is absent or unknown.  Check for a legacy
			// single-shot response (simulator not yet upgraded to streaming).
			var probe struct {
				Status string `json:"status"`
			}
			if jsonErr := json.Unmarshal(line, &probe); jsonErr == nil && probe.Status != "" {
				// Legacy non-streaming response — return the whole line as the
				// final payload so existing callers keep working.
				return json.RawMessage(line), nil
			}
			// Truly unknown frame; skip for forward compatibility.
		}
	}
}
