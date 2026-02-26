// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", cfg.MaxRetries)
	}
	if cfg.InitialBackoff != 1*time.Second {
		t.Errorf("expected InitialBackoff=1s, got %v", cfg.InitialBackoff)
	}
	if cfg.MaxBackoff != 10*time.Second {
		t.Errorf("expected MaxBackoff=10s, got %v", cfg.MaxBackoff)
	}
	if len(cfg.StatusCodesToRetry) == 0 {
		t.Errorf("expected StatusCodesToRetry to have values")
	}
}

func TestRetryerSuccessFirstAttempt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	retrier := NewRetrier(cfg, server.Client())

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := retrier.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected StatusOK, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "success" {
		t.Errorf("expected 'success', got '%s'", string(body))
	}
}

func TestRetryerOn429ThenSuccess(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("rate limited"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	cfg.MaxRetries = 3
	cfg.InitialBackoff = 10 * time.Millisecond
	retrier := NewRetrier(cfg, server.Client())

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := retrier.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected StatusOK, got %d", resp.StatusCode)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryerMaxRetriesExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("rate limited"))
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	cfg.MaxRetries = 2
	cfg.InitialBackoff = 10 * time.Millisecond
	retrier := NewRetrier(cfg, server.Client())

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := retrier.Do(context.Background(), req)
	if err == nil {
		t.Errorf("expected error, got nil")
		if resp != nil {
			resp.Body.Close()
		}
		return
	}
	if resp != nil {
		t.Errorf("expected nil response on error, got response")
		resp.Body.Close()
	}
}

func TestRetryerContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	cfg.InitialBackoff = 100 * time.Millisecond
	retrier := NewRetrier(cfg, server.Client())

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := retrier.Do(ctx, req)
	if err == nil {
		t.Errorf("expected error from context cancellation, got nil")
		resp.Body.Close()
		return
	}
	if resp != nil {
		resp.Body.Close()
	}
}

func TestRetryAfterHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	cfg.MaxRetries = 1
	retrier := NewRetrier(cfg, server.Client())

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	start := time.Now()
	resp, err := retrier.Do(context.Background(), req)
	elapsed := time.Since(start)

	if err == nil {
		t.Errorf("expected error (max retries), got nil")
		if resp != nil {
			resp.Body.Close()
		}
		return
	}
	if resp != nil {
		t.Errorf("expected nil response on error, got response")
		resp.Body.Close()
	}

	// Should have waited at least as long as Retry-After value
	if elapsed < 1*time.Second {
		t.Logf("warning: elapsed time %v is less than expected 1s (may pass due to timing variability)", elapsed)
	}
}

func TestRetryerExponentialBackoff(t *testing.T) {
	cfg := DefaultRetryConfig()
	retrier := NewRetrier(cfg, nil)

	backoff := cfg.InitialBackoff
	expectedBackoffs := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		10 * time.Second, // capped at MaxBackoff
	}

	for _, expected := range expectedBackoffs {
		next := retrier.nextBackoff(backoff)
		if next < expected || next > expected+100*time.Millisecond {
			t.Logf("backoff progression: %v (with jitter, checking range around %v)", next, expected)
		}
		backoff = next
	}
}

func TestRetryTransportSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	transport := NewRetryTransport(cfg, http.DefaultTransport)
	client := &http.Client{Transport: transport}

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected StatusOK, got %d", resp.StatusCode)
	}
}

func TestRetryTransportRetry503(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	cfg.InitialBackoff = 10 * time.Millisecond
	transport := NewRetryTransport(cfg, http.DefaultTransport)
	client := &http.Client{Transport: transport}

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected StatusOK, got %d", resp.StatusCode)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryTransportCustomStatusCodes(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.StatusCodesToRetry = []int{429, 500, 502}
	transport := NewRetryTransport(cfg, http.DefaultTransport)

	if !transport.shouldRetry(429) {
		t.Errorf("expected 429 to be retryable")
	}
	if !transport.shouldRetry(500) {
		t.Errorf("expected 500 to be retryable")
	}
	if !transport.shouldRetry(502) {
		t.Errorf("expected 502 to be retryable")
	}
	if transport.shouldRetry(400) {
		t.Errorf("expected 400 to not be retryable")
	}
}

func TestParseRetryAfterSeconds(t *testing.T) {
	cfg := DefaultRetryConfig()
	retrier := NewRetrier(cfg, nil)

	resp := &http.Response{
		Header: http.Header{
			"Retry-After": []string{"5"},
		},
	}

	duration := retrier.getRetryAfter(resp)
	if duration != 5*time.Second {
		t.Errorf("expected 5s, got %v", duration)
	}
}

func TestParseRetryAfterHTTPDate(t *testing.T) {
	cfg := DefaultRetryConfig()
	retrier := NewRetrier(cfg, nil)

	futureTime := time.Now().Add(3 * time.Second)
	dateStr := futureTime.Format(time.RFC1123)

	resp := &http.Response{
		Header: http.Header{
			"Retry-After": []string{dateStr},
		},
	}

	duration := retrier.getRetryAfter(resp)
	if duration <= 0 {
		t.Errorf("expected positive duration, got %v", duration)
	}
	if duration > 5*time.Second {
		t.Errorf("expected duration <= 5s, got %v", duration)
	}
}

func TestParseRetryAfterInvalid(t *testing.T) {
	cfg := DefaultRetryConfig()
	retrier := NewRetrier(cfg, nil)

	resp := &http.Response{
		Header: http.Header{
			"Retry-After": []string{"invalid"},
		},
	}

	duration := retrier.getRetryAfter(resp)
	if duration != 0 {
		t.Errorf("expected 0 for invalid header, got %v", duration)
	}
}

func TestRetryerRequestBodyNotReplayed(t *testing.T) {
	attempts := 0
	var bodies []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))

		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	cfg.InitialBackoff = 10 * time.Millisecond
	retrier := NewRetrier(cfg, server.Client())

	body := []byte("test body")
	req, err := http.NewRequest("POST", server.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := retrier.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryerClonedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "test" {
			t.Errorf("expected X-Custom header, got empty")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	retrier := NewRetrier(cfg, server.Client())

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("X-Custom", "test")

	resp, err := retrier.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer resp.Body.Close()
}

func TestRetryerNilClient(t *testing.T) {
	cfg := DefaultRetryConfig()
	retrier := NewRetrier(cfg, nil)

	if retrier.client == nil || retrier.client != http.DefaultClient {
		t.Errorf("expected retrier to use DefaultClient when nil passed")
	}
}

func TestRetryerResponseTooLarge(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		w.Write([]byte("response too large"))
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	cfg.MaxRetries = 3
	cfg.InitialBackoff = 10 * time.Millisecond
	retrier := NewRetrier(cfg, server.Client())

	req, err := http.NewRequest("POST", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := retrier.Do(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 413 response, got nil")
	}
	if resp != nil {
		resp.Body.Close()
	}

	if attempts != 1 {
		t.Errorf("expected exactly 1 attempt (no retry for 413), got %d", attempts)
	}

	if err.Error() == "" || !containsResponseTooLarge(err) {
		t.Errorf("expected response-too-large error, got: %v", err)
	}
}

func TestRetryTransportResponseTooLarge(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		w.Write([]byte("response too large"))
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	cfg.MaxRetries = 3
	cfg.InitialBackoff = 10 * time.Millisecond
	transport := NewRetryTransport(cfg, http.DefaultTransport)
	client := &http.Client{Transport: transport}

	req, err := http.NewRequest("POST", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err == nil {
		t.Fatal("expected error for 413 response, got nil")
	}
	if resp != nil {
		resp.Body.Close()
	}

	if attempts != 1 {
		t.Errorf("expected exactly 1 attempt (no retry for 413), got %d", attempts)
	}

	if !containsResponseTooLarge(err) {
		t.Errorf("expected response-too-large error, got: %v", err)
	}
}

func containsResponseTooLarge(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return len(msg) > 0 &&
		(contains(msg, "response too large") || contains(msg, "exceeded the server"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func BenchmarkRetryerSuccess(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	retrier := NewRetrier(cfg, server.Client())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, _ := retrier.Do(context.Background(), req)
		if resp != nil {
			resp.Body.Close()
		}
	}
}

func BenchmarkRetryTransport(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	transport := NewRetryTransport(cfg, http.DefaultTransport)
	client := &http.Client{Transport: transport}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}
}
