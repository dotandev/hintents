// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// StreamingRunner spawns the Rust simulator subprocess and delivers snapshot
// frames over a channel as they are produced by the simulator, without waiting
// for the simulation to complete.  This reduces Time-to-First-Interactive for
// the CLI UI.
type StreamingRunner struct {
	// BinaryPath is the absolute path to the erst-sim binary.
	BinaryPath string
}

// NewStreamingRunner returns a StreamingRunner that invokes the binary at
// binaryPath.  Use simulator.Runner to resolve the binary path; see
// internal/simulator/runner.go.
func NewStreamingRunner(binaryPath string) *StreamingRunner {
	return &StreamingRunner{BinaryPath: binaryPath}
}

// StreamResult bundles the snapshot-frame channel and the outcome of a
// streaming simulation run.
//
// Usage pattern:
//
//	result, err := runner.RunStreaming(ctx, reqJSON)
//	for frame := range result.Frames {
//	    // render intermediate snapshot
//	}
//	<-result.Done
//	if result.Err() != nil { ... }
//	finalJSON := result.FinalData() // unmarshal into SimulationResponse
type StreamResult struct {
	// Frames is closed by the background goroutine once all snapshot frames
	// have been delivered or an error has occurred.  Callers should range
	// over it to receive intermediate snapshots.
	Frames <-chan StreamFrame

	// Done is closed after Frames has been closed and the final payload (or
	// error) has been stored.  Always wait for Done before calling FinalData()
	// or Err().
	Done <-chan struct{}

	// finalData and err are written by the background goroutine before Done
	// is closed; reads are safe only after Done is closed.
	finalData json.RawMessage
	err       error
}

// FinalData returns the raw JSON payload of the terminal frame emitted by the
// simulator.  Callers should unmarshal it into simulator.SimulationResponse.
// Safe to call only after Done is closed.
func (sr *StreamResult) FinalData() json.RawMessage { return sr.finalData }

// Err returns any error that occurred during the streaming run.  Safe to call
// only after Done is closed.
func (sr *StreamResult) Err() error { return sr.err }

// RunStreaming starts the simulator subprocess and immediately returns a
// *StreamResult.  reqJSON must be the JSON-encoded simulation request.
// Snapshot frames are delivered asynchronously over result.Frames as the
// simulator produces them.  The subprocess is terminated when ctx is cancelled.
//
// Callers must always drain result.Frames to avoid blocking the background
// goroutine.
func (s *StreamingRunner) RunStreaming(ctx context.Context, reqJSON []byte) (*StreamResult, error) {
	if len(reqJSON) == 0 {
		return nil, fmt.Errorf("bridge: reqJSON must not be empty")
	}

	// exec.CommandContext cancels the process when ctx expires.
	cmd := exec.CommandContext(ctx, s.BinaryPath) //nolint:gosec // path is caller-supplied trusted binary
	cmd.Stdin = bytes.NewReader(reqJSON)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("bridge: create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("bridge: start simulator: %w", err)
	}

	// Buffer up to 32 snapshot frames so that fast simulators do not block
	// waiting for the caller to consume.
	frames := make(chan StreamFrame, 32)
	done := make(chan struct{})

	result := &StreamResult{
		Frames: frames,
		Done:   done,
	}

	go func() {
		// Ensure Frames is always closed before Done so that a range-over-Frames
		// loop terminates before the caller reads result.Err().
		defer close(done)
		defer close(frames)

		reader := NewFrameReader(stdoutPipe)
		finalData, readErr := reader.ReadFrames(ctx, frames)

		// Always wait for the process to avoid zombie processes, regardless of
		// whether ReadFrames succeeded.
		waitErr := cmd.Wait()

		switch {
		case readErr != nil:
			result.err = readErr
		case ctx.Err() != nil:
			result.err = ctx.Err()
		case waitErr != nil:
			result.err = fmt.Errorf("bridge: simulator exited with error: %w", waitErr)
		default:
			result.finalData = finalData
		}
	}()

	return result, nil
}
