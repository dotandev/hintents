// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package daemon

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAcceptsGzip(t *testing.T) {
	cases := []struct {
		name    string
		header  string
		accepts bool
	}{
		{"absent", "", false},
		{"plain gzip", "gzip", true},
		{"uppercase", "GZIP", true},
		{"with q value", "gzip;q=1.0", true},
		{"in list", "br, gzip, deflate", true},
		{"other encodings only", "br, deflate", false},
		{"wildcard is not gzip", "*", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Accept-Encoding", tc.header)
			}
			if got := acceptsGzip(req); got != tc.accepts {
				t.Fatalf("acceptsGzip(%q) = %v, want %v", tc.header, got, tc.accepts)
			}
		})
	}
}

func TestGzipHandlerCompressesWhenAccepted(t *testing.T) {
	payload := strings.Repeat(`{"result":"ledger-state"}`, 128)
	handler := gzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, payload)
	}))

	req := httptest.NewRequest(http.MethodPost, "/rpc", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	if got := resp.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if !strings.Contains(resp.Header.Get("Vary"), "Accept-Encoding") {
		t.Fatalf("Vary = %q, want it to contain Accept-Encoding", resp.Header.Get("Vary"))
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		t.Fatalf("Content-Length = %q, want it removed for compressed responses", cl)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("response is not valid gzip: %v", err)
	}
	defer func() { _ = gz.Close() }()
	decoded, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("failed to decompress response: %v", err)
	}
	if string(decoded) != payload {
		t.Fatalf("decoded body mismatch: got %d bytes, want %d", len(decoded), len(payload))
	}
}

func TestGzipHandlerPassesThroughWithoutAcceptEncoding(t *testing.T) {
	handler := gzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "plain-body")
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	if got := resp.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty for non-gzip client", got)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "plain-body" {
		t.Fatalf("body = %q, want plain-body", body)
	}
}

func TestGzipHandlerPreservesStatusCode(t *testing.T) {
	handler := gzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = io.WriteString(w, "teapot")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
	gz, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("response is not valid gzip: %v", err)
	}
	defer func() { _ = gz.Close() }()
	decoded, _ := io.ReadAll(gz)
	if string(decoded) != "teapot" {
		t.Fatalf("decoded body = %q, want teapot", decoded)
	}
}

func TestGzipHandlerSkipsBodylessResponses(t *testing.T) {
	handler := gzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Result().Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty for 204", got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("204 response wrote %d body bytes, want 0", rec.Body.Len())
	}
}

func TestGzipHandlerEmptyBodyIsDecodable(t *testing.T) {
	handler := gzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Result().Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	gz, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("empty response is not valid gzip: %v", err)
	}
	defer func() { _ = gz.Close() }()
	decoded, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("failed to decompress empty response: %v", err)
	}
	if len(decoded) != 0 {
		t.Fatalf("decoded %d bytes, want 0", len(decoded))
	}
}
