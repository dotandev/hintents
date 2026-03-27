// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dotandev/hintents/internal/ipc"
)

type chunkStreamError struct {
	retryLimit int
	err        error
}

func (e *chunkStreamError) Error() string {
	return e.err.Error()
}

func (e *chunkStreamError) Unwrap() error {
	return e.err
}

type lineReadResult struct {
	line []byte
	err  error
}

func decodeSimulationResponseStream(
	ctx context.Context,
	reader io.Reader,
) (*SimulationResponse, error) {
	lineCh := make(chan lineReadResult, 4)
	go readChunkLines(reader, lineCh)

	first, err := waitForChunkLine(ctx, lineCh, time.Duration(ipc.DefaultChunkTimeoutMs)*time.Millisecond)
	if err != nil {
		return nil, err
	}

	var frame ipc.SimulationStreamFrame
	if err := json.Unmarshal(first, &frame); err == nil && frame.IsChunkFrame() {
		return decodeChunkedSimulationResponse(ctx, lineCh, frame)
	}

	var resp SimulationResponse
	if err := json.Unmarshal(first, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func decodeChunkedSimulationResponse(
	ctx context.Context,
	lineCh <-chan lineReadResult,
	start ipc.SimulationStreamFrame,
) (*SimulationResponse, error) {
	if start.Kind != ipc.ChunkFrameStart {
		return nil, fmt.Errorf("unexpected first chunk frame %q", start.Kind)
	}

	timeoutMs := start.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = ipc.DefaultChunkTimeoutMs
	}
	retryLimit := start.RetryLimit
	if retryLimit < 0 {
		retryLimit = 0
	}

	tempFile, err := os.CreateTemp("", "erst-sim-response-*.json")
	if err != nil {
		return nil, err
	}
	defer func() {
		tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	expectedIndex := 0
	for {
		line, err := waitForChunkLine(ctx, lineCh, time.Duration(timeoutMs)*time.Millisecond)
		if err != nil {
			return nil, &chunkStreamError{
				retryLimit: retryLimit,
				err:        fmt.Errorf("chunk delivery failed for stream %s: %w", start.StreamID, err),
			}
		}

		var frame ipc.SimulationStreamFrame
		if err := json.Unmarshal(line, &frame); err != nil {
			return nil, &chunkStreamError{
				retryLimit: retryLimit,
				err:        fmt.Errorf("invalid chunk frame for stream %s: %w", start.StreamID, err),
			}
		}

		if frame.StreamID != start.StreamID {
			return nil, &chunkStreamError{
				retryLimit: retryLimit,
				err:        fmt.Errorf("unexpected stream id %q while reading %q", frame.StreamID, start.StreamID),
			}
		}

		switch frame.Kind {
		case ipc.ChunkFrameData:
			if frame.Index != expectedIndex {
				return nil, &chunkStreamError{
					retryLimit: retryLimit,
					err:        fmt.Errorf("missing chunk: expected %d, received %d", expectedIndex, frame.Index),
				}
			}
			if _, err := io.Copy(tempFile, base64.NewDecoder(base64.StdEncoding, strings.NewReader(frame.DataBase64))); err != nil {
				return nil, &chunkStreamError{
					retryLimit: retryLimit,
					err:        fmt.Errorf("failed to decode chunk %d: %w", frame.Index, err),
				}
			}
			expectedIndex++
		case ipc.ChunkFrameEnd:
			if frame.Total != start.Total {
				return nil, &chunkStreamError{
					retryLimit: retryLimit,
					err:        fmt.Errorf("chunk total mismatch: start=%d end=%d", start.Total, frame.Total),
				}
			}
			if expectedIndex != start.Total {
				return nil, &chunkStreamError{
					retryLimit: retryLimit,
					err:        fmt.Errorf("incomplete chunk stream: received %d of %d", expectedIndex, start.Total),
				}
			}
			if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
				return nil, err
			}
			var resp SimulationResponse
			if err := json.NewDecoder(tempFile).Decode(&resp); err != nil {
				return nil, err
			}
			return &resp, nil
		default:
			return nil, &chunkStreamError{
				retryLimit: retryLimit,
				err:        fmt.Errorf("unexpected chunk frame kind %q", frame.Kind),
			}
		}
	}
}

func readChunkLines(reader io.Reader, out chan<- lineReadResult) {
	defer close(out)

	bufReader := bufio.NewReaderSize(reader, ipc.DefaultChunkMaxLineSize)
	for {
		line, err := bufReader.ReadBytes('\n')
		if len(line) > 0 {
			out <- lineReadResult{line: trimLineEndings(line)}
		}
		if err != nil {
			if err != io.EOF {
				out <- lineReadResult{err: err}
			}
			return
		}
	}
}

func waitForChunkLine(
	ctx context.Context,
	lineCh <-chan lineReadResult,
	timeout time.Duration,
) ([]byte, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, fmt.Errorf("timed out waiting for next chunk after %v", timeout)
		case res, ok := <-lineCh:
			if !ok {
				return nil, io.ErrUnexpectedEOF
			}
			if res.err != nil {
				return nil, res.err
			}
			if len(res.line) == 0 {
				continue
			}
			return res.line, nil
		}
	}
}

func trimLineEndings(line []byte) []byte {
	return []byte(strings.TrimRight(string(line), "\r\n"))
}
