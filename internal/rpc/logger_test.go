// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// SanitizeLogString
// ---------------------------------------------------------------------------

func TestSanitizeLogString_MasksAuthorizationHeader(t *testing.T) {
	input := `request failed: Authorization: Bearer abc.def.ghi`
	got := SanitizeLogString(input)

	assert.Contains(t, got, "Authorization: "+RedactedPlaceholder)
	assert.NotContains(t, got, "abc.def.ghi")
}

func TestSanitizeLogString_MasksJSONEncodedAuthorization(t *testing.T) {
	input := `{"error":"unauthorized","authorization":"Bearer super-secret-token"}`
	got := SanitizeLogString(input)

	assert.Contains(t, got, RedactedPlaceholder)
	assert.NotContains(t, got, "super-secret-token")
}

func TestSanitizeLogString_MasksProxyAuthorizationHeader(t *testing.T) {
	input := `proxy handshake failed: Proxy-Authorization: Basic dXNlcjpwYXNz`
	got := SanitizeLogString(input)

	assert.Contains(t, got, RedactedPlaceholder)
	assert.NotContains(t, got, "dXNlcjpwYXNz")
}

func TestSanitizeLogString_MasksBearerTokenInFreeText(t *testing.T) {
	input := `upstream rejected token Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig`
	got := SanitizeLogString(input)

	assert.Contains(t, got, "Bearer "+RedactedPlaceholder)
	assert.NotContains(t, got, "eyJhbGciOiJIUzI1NiJ9")
}

func TestSanitizeLogString_MasksPEMPrivateKeyBlock(t *testing.T) {
	input := "decode failure near -----BEGIN PRIVATE KEY-----\nMIIBVAIBADANBgkq\nA9f8S7d6F5g4\n-----END PRIVATE KEY-----\nwhile parsing payload"
	got := SanitizeLogString(input)

	assert.Contains(t, got, RedactedPlaceholder)
	assert.NotContains(t, got, "MIIBVAIBADANBgkq")
	assert.NotContains(t, got, "A9f8S7d6F5g4")
	assert.Contains(t, got, "decode failure near ")
	assert.Contains(t, got, "while parsing payload")
}

func TestSanitizeLogString_MasksECPrivateKeyBlock(t *testing.T) {
	input := "-----BEGIN EC PRIVATE KEY-----\nMHQCAQEEII\n-----END EC PRIVATE KEY-----"
	got := SanitizeLogString(input)

	assert.Equal(t, RedactedPlaceholder, got)
}

func TestSanitizeLogString_MasksStellarSecretSeed(t *testing.T) {
	seed := "SBWK67QQGUTFU2P7465M7LLBRQKXTXNBVZ4V3LV2S6BRT2KQVVDV6GGD"
	input := "signer seed leaked: " + seed
	got := SanitizeLogString(input)

	assert.Contains(t, got, RedactedPlaceholder)
	assert.NotContains(t, got, seed)
}

func TestSanitizeLogString_KeepsPublicKeyAndContractID(t *testing.T) {
	input := "GCAZB67QQGUTFU2P7465M7LLBRQKXTXNBVZ4V3LV2S6BRT2KQVVDV6GGD and CASB7QQGUTFU2P7465M7LLBRQKXTXNBVZ4V3LV2S6BRT2KQVVDV6GGD3"
	got := SanitizeLogString(input)

	assert.Equal(t, input, got, "public keys and contract IDs must not be masked")
}

func TestSanitizeLogString_MasksLabeledSecrets(t *testing.T) {
	tests := []struct {
		name  string
		input string
		leak  string
	}{
		{
			name:  "private key assignment",
			input: "ERST_SOFTWARE_PRIVATE_KEY_HEX=6c69e2f4a1b2c3d4e5f60718293a4b5c6d7e8f9012345678abcdef0123456789",
			leak:  "6c69e2f4a1b2c3d4e5f60718293a4b5c6d7e8f9012345678abcdef0123456789",
		},
		{
			name:  "json secret field",
			input: `{"secret":"0123456789abcdef0123456789abcdef"}`,
			leak:  "0123456789abcdef0123456789abcdef",
		},
		{
			name:  "seed label",
			input: "seed: 4f3edf983ac636a65a842ce7c78d9aa706115327ac263e0dd4a1f9a7c88f2e5b",
			leak:  "4f3edf983ac636a65a842ce7c78d9aa706115327ac263e0dd4a1f9a7c88f2e5b",
		},
		{
			name:  "token assignment",
			input: "token=ghp_16C7e42F292c6912E7710c838347Ae178B4a",
			leak:  "ghp_16C7e42F292c6912E7710c838347Ae178B4a",
		},
		{
			name:  "pkcs11 pin",
			input: "ERST_PKCS11_PIN=123456",
			leak:  "123456",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeLogString(tc.input)

			assert.Contains(t, got, RedactedPlaceholder)
			assert.NotContains(t, got, tc.leak)
		})
	}
}

func TestSanitizeLogString_KeepsTransactionHashes(t *testing.T) {
	input := "tx 2a4c3d65b1f097e0c86a2b6ddba1a4335c3f0e781a2bce9f9c1d7f1a2b3c4d5e not found"
	got := SanitizeLogString(input)

	assert.Equal(t, input, got, "unlabelled hashes must stay visible for debugging")
}

func TestSanitizeLogString_Idempotent(t *testing.T) {
	input := "Authorization: Bearer abc.def.ghi seed SBWK67QQGUTFU2P7465M7LLBRQKXTXNBVZ4V3LV2S6BRT2KQVVDV6GGD"
	once := SanitizeLogString(input)
	twice := SanitizeLogString(once)

	assert.Equal(t, once, twice)
}

func TestSanitizeLogString_EmptyString(t *testing.T) {
	assert.Equal(t, "", SanitizeLogString(""))
}

func TestSanitizeLogString_PlainTextUnchanged(t *testing.T) {
	input := "Soroban getLedgerEntries request failed after 3 attempts"
	assert.Equal(t, input, SanitizeLogString(input))
}

// ---------------------------------------------------------------------------
// IsSensitiveHeader / SanitizeHeaders
// ---------------------------------------------------------------------------

func TestIsSensitiveHeader_Canonicalisation(t *testing.T) {
	for _, name := range []string{"Authorization", "authorization", "AUTHORIZATION"} {
		assert.True(t, IsSensitiveHeader(name), name)
	}
	assert.True(t, IsSensitiveHeader("X-API-Key"))
	assert.False(t, IsSensitiveHeader("Content-Type"))
	assert.False(t, IsSensitiveHeader("X-Custom-Header"))
}

func TestSanitizeHeaders_MasksSensitiveHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer super-secret-token")
	h.Set("X-Api-Key", "abcd1234")
	h.Set("Content-Type", "application/json")
	h.Set("X-Custom", "Bearer embedded-token-value")

	got := SanitizeHeaders(h)

	assert.Equal(t, []string{RedactedPlaceholder}, got.Values("Authorization"))
	assert.Equal(t, []string{RedactedPlaceholder}, got.Values("X-Api-Key"))
	assert.Equal(t, []string{"application/json"}, got.Values("Content-Type"))
	assert.Contains(t, got.Get("X-Custom"), RedactedPlaceholder)
	assert.NotContains(t, got.Get("X-Custom"), "embedded-token-value")

	// The original header must be untouched.
	assert.Equal(t, "Bearer super-secret-token", h.Get("Authorization"))
}

func TestSanitizeHeaders_NilHeader(t *testing.T) {
	assert.Nil(t, SanitizeHeaders(nil))
}

func TestSanitizeHeaders_EmptyHeader(t *testing.T) {
	got := SanitizeHeaders(http.Header{})
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

// ---------------------------------------------------------------------------
// Integration: the logging middleware must never emit raw credentials
// ---------------------------------------------------------------------------

// TestLoggingMiddleware_NeverLogsAuthorizationHeader drives the logging
// middleware with a client carrying a bearer token and asserts that the token
// value never appears in the emitted log stream.
func TestLoggingMiddleware_NeverLogsAuthorizationHeader(t *testing.T) {
	const token = "super-secret-bearer-token-42"

	server := captureServer(t, http.StatusOK, `{"jsonrpc":"2.0","result":{"status":"healthy"},"id":1}`)
	defer server.Close()

	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	client, err := NewClient(
		WithHorizonURL(server.URL),
		WithSorobanURL(server.URL),
		WithToken(token),
		WithLoggingEnabled(true),
	)
	require.NoError(t, err)

	_, _ = client.GetHealth(context.Background())

	logs := buf.String()
	require.NotEmpty(t, logs, "expected middleware to emit log lines")
	assert.NotContains(t, logs, token,
		"the bearer token must never appear in verbose RPC logs")
	assert.Contains(t, logs, RedactedPlaceholder,
		"the sanitized Authorization header should be visible in the log stream")
}

// TestLoggingMiddleware_SanitizesUpstreamErrorMessages ensures private key
// material quoted inside upstream error strings is masked before logging.
func TestLoggingMiddleware_SanitizesUpstreamErrorMessages(t *testing.T) {
	const seed = "SBWK67QQGUTFU2P7465M7LLBRQKXTXNBVZ4V3LV2S6BRT2KQVVDV6GGD"

	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	errTransport := RoundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("invalid signature for seed " + seed)
	})

	transport := NewLoggingMiddleware()(errTransport)
	req, err := http.NewRequest(http.MethodPost, "http://example.invalid/rpc", nil)
	require.NoError(t, err)

	_, err = transport.RoundTrip(req)
	require.Error(t, err)

	logs := buf.String()
	require.NotEmpty(t, logs)
	assert.NotContains(t, logs, seed,
		"private key material quoted in upstream errors must be masked")
	assert.Contains(t, logs, RedactedPlaceholder)
}

// ---------------------------------------------------------------------------
// Helper behaviour guards
// ---------------------------------------------------------------------------

func TestSanitizeLogString_URLEncodedAuthorizationNotLeaked(t *testing.T) {
	target := "https://node.example.com/rpc?authorization=Bearer%20abc123"
	got := SanitizeLogString(target)

	// The labelled query-parameter form is masked just like the header form.
	assert.NotContains(t, got, "abc123")
	assert.Contains(t, got, RedactedPlaceholder)
}

func TestSanitizeLogString_MultilinePEMInJSONPayload(t *testing.T) {
	// Simulates a verbose log of a JSON request body containing an escaped PEM block.
	input := `{"txenvelope":"-----BEGIN PRIVATE KEY-----\nMIIabc\nMIIdef\n-----END PRIVATE KEY-----"}`
	got := SanitizeLogString(input)

	assert.NotContains(t, got, "MIIabc")
	assert.Contains(t, got, RedactedPlaceholder)
}

func TestSanitizeHeaders_ValuesSanitizedInPlace(t *testing.T) {
	h := http.Header{}
	h.Set("X-Request-Trace", "seed SBWK67QQGUTFU2P7465M7LLBRQKXTXNBVZ4V3LV2S6BRT2KQVVDV6GGD")

	got := SanitizeHeaders(h)

	assert.NotContains(t, got.Get("X-Request-Trace"), "SBWK67")
	assert.Contains(t, got.Get("X-Request-Trace"), RedactedPlaceholder)
}

func TestSanitizeLogString_KeepsLongWordsUnrelatedToSecrets(t *testing.T) {
	input := "ledger hash 64HEXSTRINGVALUECOUNTEDTOEXACTLYFIFTYSIXCHARS ok"
	assert.Equal(t, input, SanitizeLogString(input))
}

// ---------------------------------------------------------------------------
// Regression: authHeaderRe / labeledSecretRe must not corrupt unrelated text
// ---------------------------------------------------------------------------

func TestSanitizeLogString_DoesNotMatchInsideUnrelatedWord(t *testing.T) {
	// "authorization" appearing as a substring of a larger word (not a
	// header/value pair) must not be treated as an Authorization header and
	// must not mask the rest of the line.
	input := "reauthorization: header ignored"
	assert.Equal(t, input, SanitizeLogString(input),
		"a word merely containing \"authorization\" must not trigger redaction")
}

func TestSanitizeLogString_PreservesTrailingParen(t *testing.T) {
	input := "config error (token=ghp_16C7e42F292c6912E7710c838347Ae178B4a)"
	got := SanitizeLogString(input)

	assert.Equal(t, "config error (token="+RedactedPlaceholder+")", got,
		"the closing paren must survive redaction")
	assert.NotContains(t, got, "ghp_16C7e42F292c6912E7710c838347Ae178B4a")
}

func TestSanitizeLogString_PreservesTrailingBracket(t *testing.T) {
	input := "pin check failed [pin=123456]"
	got := SanitizeLogString(input)

	assert.Equal(t, "pin check failed [pin="+RedactedPlaceholder+"]", got,
		"the closing bracket must survive redaction")
	assert.NotContains(t, got, "123456")
}
