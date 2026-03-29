// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"strings"
	"testing"
)

func TestExtractLinearMemoryBase64(t *testing.T) {
	enc := base64.StdEncoding.EncodeToString([]byte("abc"))

	got, err := extractLinearMemoryBase64(`{"linear_memory_base64":"` + enc + `"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != enc {
		t.Fatalf("expected %q, got %q", enc, got)
	}

	got, err = extractLinearMemoryBase64(`{"linear_memory_dump":"` + enc + `"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != enc {
		t.Fatalf("expected dump field %q, got %q", enc, got)
	}

	got, err = extractLinearMemoryBase64(`{"linear_memory":"` + enc + `"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != enc {
		t.Fatalf("expected fallback memory field %q, got %q", enc, got)
	}
}

func TestPrintMemorySegment(t *testing.T) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	os.Stdout = w

	printMemorySegment([]byte("ABCD\x00EFGH"), 32)

	w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "00000020") {
		t.Fatalf("expected offset in output, got %q", out)
	}
	if !strings.Contains(out, "|ABCD.EFGH|") {
		t.Fatalf("expected ascii segment in output, got %q", out)
	}
}
