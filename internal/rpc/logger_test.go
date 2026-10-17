// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripSensitiveData_AuthorizationHeader(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard Authorization Bearer header",
			input:    "failed with Authorization: Bearer secret-token-xyz-12345",
			expected: "failed with Authorization: Bearer [REDACTED]",
		},
		{
			name:     "case-insensitive authorization header",
			input:    "request header: authorization: my-custom-api-token-9999",
			expected: "request header: authorization: [REDACTED]",
		},
		{
			name:     "authorization with equals sign",
			input:    "auth context: authorization=secret-token-key-abcdef",
			expected: "auth context: authorization=[REDACTED]",
		},
		{
			name:     "standalone bearer token",
			input:    "connecting with Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.token payload",
			expected: "connecting with Bearer [REDACTED] payload",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripSensitiveData(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestStripSensitiveData_StellarSecretKey(t *testing.T) {
	// A valid 56-character base32 secret seed (starting with 'S')
	secretSeed := "SBCAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	// Public key (starting with 'G') must NOT be redacted
	publicKey := "GBCAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	input := fmt.Sprintf("Account %s failed transaction with signer secret %s", publicKey, secretSeed)
	output := StripSensitiveData(input)

	assert.NotContains(t, output, secretSeed)
	assert.Contains(t, output, RedactedSecretKey)
	assert.Contains(t, output, publicKey)
}

func TestStripSensitiveData_JSONAndKVPrivateKeys(t *testing.T) {
	jsonPayload := `{"user": "alice", "private_key": "topsecret-key-12345", "token": "xyz"}`
	sanitizedJSON := StripSensitiveData(jsonPayload)

	assert.NotContains(t, sanitizedJSON, "topsecret-key-12345")
	assert.Contains(t, sanitizedJSON, `"private_key":"`+RedactedValue+`"`)

	kvPayload := `config option private_key=very-secret-seed-987654321`
	sanitizedKV := StripSensitiveData(kvPayload)

	assert.NotContains(t, sanitizedKV, "very-secret-seed-987654321")
	assert.Contains(t, sanitizedKV, "private_key="+RedactedValue)
}

func TestStripSensitiveData_Signatures(t *testing.T) {
	sig := "3045022100abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	jsonPayload := fmt.Sprintf(`{"tx_hash": "abc", "signature": "%s"}`, sig)
	sanitizedJSON := StripSensitiveData(jsonPayload)

	assert.NotContains(t, sanitizedJSON, sig)
	assert.Contains(t, sanitizedJSON, `"signature":"`+RedactedSignature+`"`)

	kvPayload := fmt.Sprintf("debug response: payload_signature=%s end", sig)
	sanitizedKV := StripSensitiveData(kvPayload)

	assert.NotContains(t, sanitizedKV, sig)
	assert.Contains(t, sanitizedKV, "payload_signature="+RedactedSignature)
}

func TestSanitizeHeader(t *testing.T) {
	headers := http.Header{
		"Authorization": []string{"Bearer secret-token-111", "Basic dXNlcjpwYXNz"},
		"Cookie":        []string{"session_id=abcdef123456"},
		"X-Api-Key":     []string{"api-key-998877"},
		"Content-Type":  []string{"application/json"},
		"User-Agent":    []string{"Erst-RPC-Client/1.0"},
	}

	sanitized := SanitizeHeader(headers)

	require.NotNil(t, sanitized)
	// Sensitive headers are redacted
	for _, val := range sanitized["Authorization"] {
		assert.Equal(t, RedactedValue, val)
	}
	for _, val := range sanitized["Cookie"] {
		assert.Equal(t, RedactedValue, val)
	}
	for _, val := range sanitized["X-Api-Key"] {
		assert.Equal(t, RedactedValue, val)
	}

	// Non-sensitive headers remain intact
	assert.Equal(t, []string{"application/json"}, sanitized["Content-Type"])
	assert.Equal(t, []string{"Erst-RPC-Client/1.0"}, sanitized["User-Agent"])

	// Test nil input
	assert.Nil(t, SanitizeHeader(nil))
}

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		expected string
	}{
		{
			name:     "URL with basic auth password",
			rawURL:   "https://alice:supersecretpass@soroban.example.com/rpc",
			expected: "https://alice:" + RedactedValue + "@soroban.example.com/rpc",
		},
		{
			name:     "URL with sensitive query parameters",
			rawURL:   "https://soroban.example.com/rpc?api_key=secret-key-123&network=testnet",
			expected: "https://soroban.example.com/rpc?api_key=" + url.QueryEscape(RedactedValue) + "&network=testnet",
		},
		{
			name:     "clean URL unchanged",
			rawURL:   "https://soroban.example.com/rpc",
			expected: "https://soroban.example.com/rpc",
		},
		{
			name:     "empty URL",
			rawURL:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeURL(tt.rawURL)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestSanitizeError(t *testing.T) {
	origErr := errors.New("rpc call failed: Authorization: Bearer secret-auth-tok-789")
	wrappedErr := SanitizeError(origErr)

	require.NotNil(t, wrappedErr)
	assert.NotContains(t, wrappedErr.Error(), "secret-auth-tok-789")
	assert.Contains(t, wrappedErr.Error(), RedactedValue)
	assert.True(t, errors.Is(wrappedErr, origErr))

	assert.Nil(t, SanitizeError(nil))
}

func TestSanitizingWriter(t *testing.T) {
	var buf bytes.Buffer
	writer := NewSanitizingWriter(&buf)

	msg := "Verbose log: Authorization: Bearer my-very-secret-token-12345 occurred"
	n, err := writer.Write([]byte(msg))

	require.NoError(t, err)
	assert.Equal(t, len(msg), n)

	output := buf.String()
	assert.NotContains(t, output, "my-very-secret-token-12345")
	assert.Contains(t, output, RedactedValue)
}

func TestSanitizingHandler(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	sanitizingHandler := NewSanitizingHandler(baseHandler)
	testLogger := slog.New(sanitizingHandler)

	testLogger.Error("Verbose error: Authorization: Bearer topsecret12345",
		"auth_header", "Bearer secrettoken999",
		"signature", "sig-value-0123456789",
		"normal_field", "normal_value",
	)

	logOutput := buf.String()
	assert.NotContains(t, logOutput, "topsecret12345")
	assert.NotContains(t, logOutput, "secrettoken999")
	assert.NotContains(t, logOutput, "sig-value-0123456789")
	assert.Contains(t, logOutput, RedactedValue)
	assert.Contains(t, logOutput, "normal_value")
}

func TestLogVerboseRPCError(t *testing.T) {
	var buf bytes.Buffer
	origLogger := logger.Logger
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	defer func() { logger.Logger = origLogger }()

	reqHeaders := http.Header{
		"Authorization": []string{"Bearer secret-verbose-token"},
		"X-Request-Id":  []string{"req-42"},
	}
	err := errors.New("dial tcp: Authorization: Bearer leaked-tok-999")

	LogVerboseRPCError("https://user:password123@stellar.rpc.org/v1", reqHeaders, err, "attempt", 3)

	output := buf.String()
	assert.NotContains(t, output, "secret-verbose-token")
	assert.NotContains(t, output, "password123")
	assert.NotContains(t, output, "leaked-tok-999")
	assert.Contains(t, output, "req-42")
	assert.Contains(t, output, RedactedValue)
}

func TestLoggingMiddleware_RedactsSensitiveDataInErrors(t *testing.T) {
	var buf bytes.Buffer
	origLogger := logger.Logger
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	defer func() { logger.Logger = origLogger }()

	// Server that fails and returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Faulty transport that produces an error containing Authorization header
	faultyTransport := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("connection aborted with header Authorization: Bearer bad-leak-token-123")
	})

	mw := NewLoggingMiddleware()
	wrapped := mw(faultyTransport)

	req, err := http.NewRequest("GET", server.URL+"?token=sensitive-query-param", nil)
	require.NoError(t, err)

	_, tripErr := wrapped.RoundTrip(req)
	require.Error(t, tripErr)

	logOutput := buf.String()
	assert.NotContains(t, logOutput, "bad-leak-token-123")
	assert.NotContains(t, logOutput, "sensitive-query-param")
	assert.Contains(t, logOutput, RedactedValue)
}
