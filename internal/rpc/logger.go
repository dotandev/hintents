// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/dotandev/hintents/internal/logger"
)

const (
	// RedactedValue is the replacement token for sensitive header values and generic secrets.
	RedactedValue = "[REDACTED]"

	// RedactedSecretKey is the replacement token for Stellar secret seeds/keys.
	RedactedSecretKey = "[REDACTED_SECRET_KEY]"

	// RedactedSignature is the replacement token for payload signatures.
	RedactedSignature = "[REDACTED_SIGNATURE]"
)

// Pre-compiled regular expressions for stripping sensitive credentials and tokens.
var (
	// Authorization header pattern: matches "Authorization: ...", "authorization=..." with optional Bearer prefix
	reAuthHeader = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*(?:bearer\s+)?)([^\s,"';]+)`)

	// Standalone Bearer token pattern
	reBearerToken = regexp.MustCompile(`(?i)(\bbearer\s+)([a-zA-Z0-9_\-\.]{10,})`)

	// Stellar secret seed pattern: begins with 'S' followed by 55 base32 chars
	reStellarSecret = regexp.MustCompile(`\bS[A-Z2-7]{55}\b`)

	// JSON-style private key and secret fields
	reJSONPrivateKey = regexp.MustCompile(`(?i)("(?:private_key|secret_key|secret|seed|privkey|privateKey|api_key|apiKey)"\s*:\s*")([^"]+)(")`)

	// Key-value style private key and secret fields
	reKVPrivateKey = regexp.MustCompile(`(?i)(\b(?:private_key|secret_key|secret|seed|privkey|privateKey|api_key|apiKey)\s*[=:]\s*)([^\s,"';]+)`)

	// JSON-style payload signature fields
	reJSONSignature = regexp.MustCompile(`(?i)("(?:signature|signatures|sig|payload_signature)"\s*:\s*")([^"]+)(")`)

	// Key-value style signature fields (targeting tokens or signatures >= 20 chars)
	reKVSignature = regexp.MustCompile(`(?i)(\b(?:signature|sig|payload_signature)\s*[=:]\s*)([a-zA-Z0-9+/=_-]{20,})`)
)

// sensitiveHeaders defines the set of HTTP header names whose values must be redacted.
var sensitiveHeaders = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"x-api-key":           {},
	"x-auth-token":        {},
	"cookie":              {},
	"set-cookie":          {},
	"token":               {},
}

// IsSensitiveHeader returns true if the specified header name contains sensitive credentials.
func IsSensitiveHeader(name string) bool {
	_, found := sensitiveHeaders[strings.ToLower(strings.TrimSpace(name))]
	return found
}

// StripSensitiveData strips sensitive credentials (such as Authorization headers,
// Bearer tokens, Stellar secret keys, private keys, and payload signatures) from arbitrary text.
func StripSensitiveData(s string) string {
	if s == "" {
		return ""
	}

	// 1. Redact Authorization headers and Bearer tokens
	s = reAuthHeader.ReplaceAllString(s, `${1}`+RedactedValue)
	s = reBearerToken.ReplaceAllString(s, `${1}`+RedactedValue)

	// 2. Redact Stellar secret keys
	s = reStellarSecret.ReplaceAllString(s, RedactedSecretKey)

	// 3. Redact private keys and secrets in JSON or key-value structures
	s = reJSONPrivateKey.ReplaceAllString(s, `${1}`+RedactedValue+`${3}`)
	s = reKVPrivateKey.ReplaceAllString(s, `${1}`+RedactedValue)

	// 4. Redact payload signatures in JSON or key-value structures
	s = reJSONSignature.ReplaceAllString(s, `${1}`+RedactedSignature+`${3}`)
	s = reKVSignature.ReplaceAllString(s, `${1}`+RedactedSignature)

	return s
}

// SanitizeHeader returns a deep copy of the given http.Header with sensitive headers redacted.
func SanitizeHeader(h http.Header) http.Header {
	if h == nil {
		return nil
	}

	sanitized := make(http.Header, len(h))
	for k, v := range h {
		if IsSensitiveHeader(k) {
			redactedVals := make([]string, len(v))
			for i := range v {
				redactedVals[i] = RedactedValue
			}
			sanitized[k] = redactedVals
		} else {
			copiedVals := make([]string, len(v))
			for i, val := range v {
				copiedVals[i] = StripSensitiveData(val)
			}
			sanitized[k] = copiedVals
		}
	}
	return sanitized
}

// SanitizeURL scrubs user credentials and sensitive query parameters (token, secret, key, auth) from a URL.
func SanitizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return StripSensitiveData(rawURL)
	}

	// Scrub password in user info
	if parsed.User != nil {
		if _, hasPassword := parsed.User.Password(); hasPassword {
			parsed.User = url.UserPassword(parsed.User.Username(), RedactedValue)
		}
	}

	// Scrub sensitive query parameters
	if parsed.RawQuery != "" {
		q := parsed.Query()
		modified := false
		for k := range q {
			lowerK := strings.ToLower(k)
			if strings.Contains(lowerK, "token") ||
				strings.Contains(lowerK, "secret") ||
				strings.Contains(lowerK, "key") ||
				strings.Contains(lowerK, "auth") ||
				strings.Contains(lowerK, "pass") {
				q.Set(k, RedactedValue)
				modified = true
			}
		}
		if modified {
			parsed.RawQuery = q.Encode()
		}
	}

	return parsed.String()
}

// sanitizedError wraps an error and redacts sensitive data when Error() is called.
type sanitizedError struct {
	err error
}

func (e *sanitizedError) Error() string {
	if e.err == nil {
		return ""
	}
	return StripSensitiveData(e.err.Error())
}

func (e *sanitizedError) Unwrap() error {
	return e.err
}

// SanitizeError wraps an error so that its message is stripped of sensitive credentials upon formatting.
func SanitizeError(err error) error {
	if err == nil {
		return nil
	}
	return &sanitizedError{err: err}
}

// SanitizingWriter wraps an io.Writer and strips sensitive data from all byte chunks before writing.
type SanitizingWriter struct {
	w io.Writer
}

// NewSanitizingWriter creates an io.Writer that strips sensitive information from written bytes.
func NewSanitizingWriter(w io.Writer) *SanitizingWriter {
	return &SanitizingWriter{w: w}
}

func (sw *SanitizingWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	sanitized := StripSensitiveData(string(p))
	_, err = sw.w.Write([]byte(sanitized))
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// SanitizingHandler wraps an slog.Handler to ensure all log records, messages, and attributes are sanitized.
type SanitizingHandler struct {
	next slog.Handler
}

// NewSanitizingHandler creates a new slog.Handler that strips sensitive data.
func NewSanitizingHandler(h slog.Handler) *SanitizingHandler {
	return &SanitizingHandler{next: h}
}

func (sh *SanitizingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return sh.next.Enabled(ctx, level)
}

func (sh *SanitizingHandler) Handle(ctx context.Context, r slog.Record) error {
	// Strip message
	newRecord := slog.NewRecord(r.Time, r.Level, StripSensitiveData(r.Message), r.PC)

	// Strip attributes
	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(sanitizeAttr(a))
		return true
	})

	return sh.next.Handle(ctx, newRecord)
}

func (sh *SanitizingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	sanitized := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		sanitized[i] = sanitizeAttr(a)
	}
	return &SanitizingHandler{next: sh.next.WithAttrs(sanitized)}
}

func (sh *SanitizingHandler) WithGroup(name string) slog.Handler {
	return &SanitizingHandler{next: sh.next.WithGroup(name)}
}

func sanitizeAttr(a slog.Attr) slog.Attr {
	// If the key is known to be sensitive
	if IsSensitiveHeader(a.Key) || strings.EqualFold(a.Key, "signature") || strings.EqualFold(a.Key, "secret") {
		return slog.String(a.Key, RedactedValue)
	}

	switch a.Value.Kind() {
	case slog.KindString:
		return slog.String(a.Key, StripSensitiveData(a.Value.String()))
	case slog.KindAny:
		switch v := a.Value.Any().(type) {
		case error:
			return slog.String(a.Key, StripSensitiveData(v.Error()))
		case http.Header:
			return slog.Any(a.Key, SanitizeHeader(v))
		case fmt.Stringer:
			return slog.String(a.Key, StripSensitiveData(v.String()))
		default:
			str := fmt.Sprint(v)
			return slog.String(a.Key, StripSensitiveData(str))
		}
	default:
		return a
	}
}

// LogVerboseRPCError formats and logs a verbose RPC error with all sensitive headers, URLs, and errors sanitized.
func LogVerboseRPCError(endpoint string, reqHeader http.Header, err error, extra ...any) {
	attrs := []any{
		"endpoint", SanitizeURL(endpoint),
		"headers", SanitizeHeader(reqHeader),
		"error", SanitizeError(err),
	}
	attrs = append(attrs, extra...)
	logger.Logger.Error("RPC verbose error", attrs...)
}
