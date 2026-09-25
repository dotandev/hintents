// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSerializeMatchesSaveOutput(t *testing.T) {
	snap := FromMap(map[string]string{"key-b": "val-b", "key-a": "val-a"})

	data, err := Serialize(snap)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "serialized.json")
	if err := Save(outPath, snap); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	onDisk, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read saved snapshot: %v", err)
	}

	if string(onDisk) != string(data) {
		t.Fatalf("Save output differs from Serialize output:\n%s\n---\n%s", onDisk, data)
	}
}

func TestSaveLeavesNoTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "atomic.json")

	if err := Save(outPath, FromMap(map[string]string{"k": "v"})); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	leftovers, err := filepath.Glob(filepath.Join(dir, ".snapshot-*.tmp"))
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("expected no temporary files, found %v", leftovers)
	}
}

func TestAsyncWriterSaveFlushesToDisk(t *testing.T) {
	w := NewAsyncWriter(AsyncWriterOptions{QueueSize: 4, Workers: 2})
	defer func() { _ = w.Close() }()

	snap := FromMap(map[string]string{"key-c": "val-c", "key-a": "val-a", "key-b": "val-b"})
	outPath := filepath.Join(t.TempDir(), "async.json")

	if err := w.Save(outPath, snap); err != nil {
		t.Fatalf("async Save failed: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	loaded, err := Load(outPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Fingerprint != snap.Fingerprint {
		t.Fatalf("fingerprint mismatch: %s vs %s", loaded.Fingerprint, snap.Fingerprint)
	}
	if len(loaded.LedgerEntries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(loaded.LedgerEntries))
	}
	if loaded.LedgerEntries[0][0] != "key-a" {
		t.Fatalf("expected sorted entries, got first key %s", loaded.LedgerEntries[0][0])
	}
}

func TestAsyncWriterFlushesAllPendingWrites(t *testing.T) {
	dir := t.TempDir()
	w := NewAsyncWriter(AsyncWriterOptions{QueueSize: 8, Workers: 3})
	defer func() { _ = w.Close() }()

	const total = 25
	for i := 0; i < total; i++ {
		path := filepath.Join(dir, fmt.Sprintf("snap-%02d.json", i))
		if err := w.Save(path, FromMap(map[string]string{"i": fmt.Sprint(i)})); err != nil {
			t.Fatalf("async Save %d failed: %v", i, err)
		}
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	for i := 0; i < total; i++ {
		path := filepath.Join(dir, fmt.Sprintf("snap-%02d.json", i))
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("Load %s failed: %v", path, err)
		}
		if got := loaded.ToMap()["i"]; got != fmt.Sprint(i) {
			t.Fatalf("expected value %d in %s, got %q", i, path, got)
		}
	}

	stats := w.Stats()
	if stats.Submitted != total || stats.Written != total || stats.Failed != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if stats.BytesWritten == 0 {
		t.Fatal("expected BytesWritten > 0")
	}
}

func TestAsyncWriterCloseDrainsPendingWrites(t *testing.T) {
	dir := t.TempDir()
	w := NewAsyncWriter(AsyncWriterOptions{QueueSize: 4, Workers: 2})

	const total = 12
	for i := 0; i < total; i++ {
		path := filepath.Join(dir, fmt.Sprintf("close-%02d.json", i))
		if err := w.Write(path, []byte(fmt.Sprintf("{\"n\":%d}", i))); err != nil {
			t.Fatalf("Write %d failed: %v", i, err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	for i := 0; i < total; i++ {
		path := filepath.Join(dir, fmt.Sprintf("close-%02d.json", i))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected %s to be drained on Close: %v", path, err)
		}
		if !strings.Contains(string(data), fmt.Sprint(i)) {
			t.Fatalf("unexpected contents in %s: %s", path, data)
		}
	}

	// Close is idempotent.
	if err := w.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
}

func TestAsyncWriterRejectsWritesAfterClose(t *testing.T) {
	w := NewAsyncWriter(AsyncWriterOptions{Workers: 1})
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if err := w.Write(filepath.Join(t.TempDir(), "late.json"), []byte("{}")); !errors.Is(err, ErrAsyncWriterClosed) {
		t.Fatalf("expected ErrAsyncWriterClosed, got %v", err)
	}
	if err := w.Save(filepath.Join(t.TempDir(), "late.json"), FromMap(nil)); !errors.Is(err, ErrAsyncWriterClosed) {
		t.Fatalf("expected ErrAsyncWriterClosed from Save, got %v", err)
	}
	if err := w.Flush(); !errors.Is(err, ErrAsyncWriterClosed) {
		t.Fatalf("expected ErrAsyncWriterClosed from Flush, got %v", err)
	}
}

func TestAsyncWriterRequiresDestinationPath(t *testing.T) {
	w := NewAsyncWriter(AsyncWriterOptions{Workers: 1})
	defer func() { _ = w.Close() }()

	if err := w.Write("", []byte("{}")); err == nil {
		t.Fatal("expected an error for an empty destination path")
	}
}

func TestAsyncWriterReportsWriteErrors(t *testing.T) {
	w := NewAsyncWriter(AsyncWriterOptions{Workers: 2})
	defer func() { _ = w.Close() }()

	missingDir := filepath.Join(t.TempDir(), "does-not-exist", "snap.json")
	if err := w.Write(missingDir, []byte("{}")); err != nil {
		t.Fatalf("Write should enqueue without blocking: %v", err)
	}

	err := w.Flush()
	if err == nil {
		t.Fatal("expected Flush to surface the background write failure")
	}
	if !strings.Contains(err.Error(), filepath.Base(missingDir)) {
		t.Fatalf("expected error to mention the failing path, got %v", err)
	}

	stats := w.Stats()
	if stats.Failed != 1 || stats.Written != 0 {
		t.Fatalf("unexpected stats after failure: %+v", stats)
	}

	// Errors are cleared once reported.
	if err := w.Flush(); err != nil {
		t.Fatalf("expected Flush to be clean after reporting, got %v", err)
	}
}

func TestAsyncWriterOnErrorCallback(t *testing.T) {
	var mu sync.Mutex
	var reported []string

	w := NewAsyncWriter(AsyncWriterOptions{
		Workers: 1,
		OnError: func(path string, err error) {
			mu.Lock()
			defer mu.Unlock()
			reported = append(reported, path)
		},
	})
	defer func() { _ = w.Close() }()

	missingDir := filepath.Join(t.TempDir(), "nope", "snap.json")
	if err := w.Write(missingDir, []byte("{}")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Flush(); err == nil {
		t.Fatal("expected Flush to report the failure")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(reported) != 1 || reported[0] != missingDir {
		t.Fatalf("expected OnError to receive %s, got %v", missingDir, reported)
	}
}

func TestAsyncWriterConcurrentSaves(t *testing.T) {
	dir := t.TempDir()
	w := NewAsyncWriter(AsyncWriterOptions{QueueSize: 8, Workers: 4})
	defer func() { _ = w.Close() }()

	const total = 40
	var wg sync.WaitGroup
	errs := make(chan error, total)

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := filepath.Join(dir, fmt.Sprintf("concurrent-%02d.json", i))
			if err := w.Save(path, FromMap(map[string]string{"i": fmt.Sprint(i)})); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent Save failed: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	for i := 0; i < total; i++ {
		path := filepath.Join(dir, fmt.Sprintf("concurrent-%02d.json", i))
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("Load %s failed: %v", path, err)
		}
		if got := loaded.ToMap()["i"]; got != fmt.Sprint(i) {
			t.Fatalf("expected value %d in %s, got %q", i, path, got)
		}
	}

	stats := w.Stats()
	if stats.Submitted != total || stats.Written != total || stats.Failed != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestAsyncWriterUsesDefaults(t *testing.T) {
	w := NewAsyncWriter(AsyncWriterOptions{})
	defer func() { _ = w.Close() }()

	if cap(w.jobs) != DefaultAsyncQueueSize {
		t.Fatalf("expected default queue size %d, got %d", DefaultAsyncQueueSize, cap(w.jobs))
	}
	if w.workers != DefaultAsyncWorkers {
		t.Fatalf("expected default workers %d, got %d", DefaultAsyncWorkers, w.workers)
	}
}
