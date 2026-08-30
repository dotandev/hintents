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
	"sort"
	"strings"
)

// chunkEntry is an internal helper for collecting and sorting chunk frames.
type chunkEntry struct {
	seq uint32
	raw json.RawMessage
}

// sortChunksBySeq sorts chunk entries by their seq number in ascending order.
func sortChunksBySeq(chunks []chunkEntry) {
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].seq < chunks[j].seq
	})
}

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

	// FrameTypeChunk is a partial-payload frame within a multi-frame large
	// response.  The consumer must concatenate all Chunk frames in seq order
	// to reconstruct the full JSON payload.  Each chunk frame carries a
	// "total" field indicating how many chunks to expect.
	FrameTypeChunk FrameType = "chunk"
)

// StreamFrame is one NDJSON line emitted by the simulator subprocess.
type StreamFrame struct {
	// Type discriminates snapshot, chunk, final, etc.
	Type FrameType `json:"type"`
	// Seq is a monotonically increasing sequence number (0-based) within
	// a single simulation run.  Out-of-order delivery is possible when the
	// simulator is extended to use concurrent goroutines; callers that care
	// about ordering should sort by Seq before processing.
	Seq uint32 `json:"seq"`
	// Total is the expected number of frames in this logical batch.
	// Only populated for chunk frames; callers should ignore it otherwise.
	Total uint32 `json:"total,omitempty"`
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
			Type  FrameType       `json:"type"`
			Seq   uint32          `json:"seq"`
			Total uint32          `json:"total,omitempty"`
			Data  json.RawMessage `json:"data"`
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

		case FrameTypeChunk:
			total := envelope.Total
			if total == 0 {
				total = 1
			}
			chunks := make([]chunkEntry, 0, total)
			chunks = append(chunks, chunkEntry{seq: envelope.Seq, raw: envelope.Data})

			for uint32(len(chunks)) < total {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
				}

				if !scanner.Scan() {
					if err := scanner.Err(); err != nil {
						return nil, fmt.Errorf("bridge: reading chunk: %w", err)
					}
					return nil, fmt.Errorf("bridge: unexpected EOF before all %d chunks arrived", total)
				}

				nextLine := scanner.Bytes()
				if len(nextLine) == 0 {
					continue
				}

				var nextEnv struct {
					Type FrameType       `json:"type"`
					Seq  uint32          `json:"seq"`
					Data json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal(nextLine, &nextEnv); err != nil {
					return nil, fmt.Errorf("bridge: unmarshal chunk frame: %w", err)
				}
				if nextEnv.Type != FrameTypeChunk {
					return nil, fmt.Errorf("bridge: expected chunk frame, got %q", nextEnv.Type)
				}
				chunks = append(chunks, chunkEntry{seq: nextEnv.Seq, raw: nextEnv.Data})
			}

			sortChunksBySeq(chunks)

			// Each chunk's data field is a JSON string containing a fragment
			// of the full payload. Decode each string and concatenate.
			var sb strings.Builder
			for _, c := range chunks {
				var fragment string
				if err := json.Unmarshal(c.raw, &fragment); err != nil {
					return nil, fmt.Errorf("bridge: decode chunk data: %w", err)
				}
				sb.WriteString(fragment)
			}
			return json.RawMessage(sb.String()), nil

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
