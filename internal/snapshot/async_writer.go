// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

const (
	// DefaultAsyncQueueSize is the number of encoded snapshot buffers an
	// AsyncWriter keeps in memory before producers block.
	DefaultAsyncQueueSize = 32
	// DefaultAsyncWorkers is the number of background goroutines an AsyncWriter
	// uses to flush encoded snapshot buffers to disk.
	DefaultAsyncWorkers = 2
)

// ErrAsyncWriterClosed is returned when work is submitted to an AsyncWriter
// that has already been closed.
var ErrAsyncWriterClosed = errors.New("snapshot: async writer is closed")

// AsyncWriterOptions configures an AsyncWriter. The zero value is valid and
// selects the package defaults.
type AsyncWriterOptions struct {
	// QueueSize is the capacity of the in-memory job queue. Non-positive values
	// select DefaultAsyncQueueSize.
	QueueSize int
	// Workers is the number of background writer goroutines. Non-positive
	// values select DefaultAsyncWorkers.
	Workers int
	// OnError, when non-nil, is invoked for every failed background write. It is
	// called from a writer goroutine and must not block.
	OnError func(path string, err error)
}

// AsyncWriterStats reports cumulative AsyncWriter activity.
type AsyncWriterStats struct {
	// Submitted is the number of payloads accepted for background writing.
	Submitted uint64
	// Written is the number of payloads successfully flushed to disk.
	Written uint64
	// Failed is the number of payloads whose disk write failed.
	Failed uint64
	// BytesWritten is the total number of bytes successfully flushed.
	BytesWritten uint64
}

type writeJob struct {
	path string
	data []byte
}

// AsyncWriter buffers encoded snapshot payloads and flushes them to disk from a
// pool of background goroutines, so that dumping massive state snapshots no
// longer blocks the caller on disk I/O.
//
// Serialization stays synchronous: callers hand AsyncWriter an already encoded
// byte slice, which keeps output bytes and fingerprints deterministic and lets
// encoding errors surface immediately on the calling goroutine. Only the write
// syscalls move off-thread.
//
// An AsyncWriter is safe for concurrent use. Call Flush to wait for pending
// writes, and Close to drain, stop the background goroutines and release them.
type AsyncWriter struct {
	jobs    chan writeJob
	workers int
	wg      sync.WaitGroup

	mu     sync.RWMutex // guards closed and job submission
	closed bool

	inflightMu sync.Mutex
	inflight   int
	idle       *sync.Cond

	errMu   sync.Mutex
	errs    []error
	onError func(path string, err error)

	submitted    atomic.Uint64
	written      atomic.Uint64
	failed       atomic.Uint64
	bytesWritten atomic.Uint64
}

// NewAsyncWriter starts an AsyncWriter with the supplied options.
func NewAsyncWriter(opts AsyncWriterOptions) *AsyncWriter {
	workers := opts.Workers
	if workers <= 0 {
		workers = DefaultAsyncWorkers
	}
	queueSize := opts.QueueSize
	if queueSize <= 0 {
		queueSize = DefaultAsyncQueueSize
	}

	w := &AsyncWriter{
		jobs:    make(chan writeJob, queueSize),
		workers: workers,
		onError: opts.OnError,
	}
	w.idle = sync.NewCond(&w.inflightMu)

	for i := 0; i < workers; i++ {
		w.wg.Add(1)
		go w.run()
	}
	return w
}

func (w *AsyncWriter) run() {
	defer w.wg.Done()
	for job := range w.jobs {
		w.dispatch(job)
	}
}

func (w *AsyncWriter) dispatch(job writeJob) {
	defer w.finishJob()

	if err := writeFileAtomic(job.path, job.data); err != nil {
		w.failed.Add(1)
		wrapped := fmt.Errorf("snapshot: async write %s: %w", job.path, err)

		w.errMu.Lock()
		w.errs = append(w.errs, wrapped)
		onError := w.onError
		w.errMu.Unlock()

		if onError != nil {
			onError(job.path, err)
		}
		return
	}

	w.written.Add(1)
	w.bytesWritten.Add(uint64(len(job.data)))
}

// finishJob marks one submitted payload as processed and wakes any goroutine
// blocked in Flush or Close.
func (w *AsyncWriter) finishJob() {
	w.inflightMu.Lock()
	w.inflight--
	if w.inflight <= 0 {
		w.inflight = 0
		w.idle.Broadcast()
	}
	w.inflightMu.Unlock()
}

// Write enqueues an already encoded snapshot payload for background disk write.
// It blocks while the in-memory queue is full and returns ErrAsyncWriterClosed
// once the writer has been closed.
func (w *AsyncWriter) Write(path string, data []byte) error {
	if path == "" {
		return errors.New("snapshot: async write requires a destination path")
	}

	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.closed {
		return ErrAsyncWriterClosed
	}

	w.inflightMu.Lock()
	w.inflight++
	w.inflightMu.Unlock()

	// Sending while holding the read lock keeps Close from closing the queue
	// under us. Writer goroutines never take this lock, so a full queue always
	// drains and the send cannot deadlock.
	w.jobs <- writeJob{path: path, data: data}
	w.submitted.Add(1)
	return nil
}

// Save serializes snap on the calling goroutine and enqueues the encoded
// payload for background disk write.
func (w *AsyncWriter) Save(path string, snap *Snapshot) error {
	data, err := Serialize(snap)
	if err != nil {
		return err
	}
	return w.Write(path, data)
}

// Flush blocks until every payload submitted before it has been written to
// disk, then returns — and clears — the write errors observed so far.
func (w *AsyncWriter) Flush() error {
	w.mu.RLock()
	closed := w.closed
	w.mu.RUnlock()
	if closed {
		return ErrAsyncWriterClosed
	}

	w.waitForIdle()
	return w.takeErrors()
}

// Close drains all pending writes, stops the background goroutines and returns
// any write errors recorded during the writer's lifetime. Close is idempotent.
func (w *AsyncWriter) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	// Holding the write lock guarantees no producer is mid-send, so the queue
	// can be closed safely. Buffered payloads are still drained by the workers.
	w.closed = true
	close(w.jobs)
	w.mu.Unlock()

	w.waitForIdle()
	w.wg.Wait()

	return w.takeErrors()
}

// waitForIdle blocks until no payload is queued or being written.
func (w *AsyncWriter) waitForIdle() {
	w.inflightMu.Lock()
	for w.inflight > 0 {
		w.idle.Wait()
	}
	w.inflightMu.Unlock()
}

// Stats returns a snapshot of the writer's cumulative counters.
func (w *AsyncWriter) Stats() AsyncWriterStats {
	return AsyncWriterStats{
		Submitted:    w.submitted.Load(),
		Written:      w.written.Load(),
		Failed:       w.failed.Load(),
		BytesWritten: w.bytesWritten.Load(),
	}
}

func (w *AsyncWriter) takeErrors() error {
	w.errMu.Lock()
	defer w.errMu.Unlock()
	if len(w.errs) == 0 {
		return nil
	}
	err := errors.Join(w.errs...)
	w.errs = nil
	return err
}

// writeFileAtomic writes data to path through a temporary file in the same
// directory followed by a rename, so readers never observe a partially written
// snapshot and concurrent flushes cannot interleave their bytes.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".snapshot-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}
