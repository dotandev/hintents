// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"net/http"
	"net/textproto"
	"regexp"
)

// RedactedPlaceholder replaces any sensitive value detected by the sanitizers
// below before it is handed to the structured logger.
const RedactedPlaceholder = "[REDACTED]"

// sensitiveHeaderNames is the set of HTTP header names whose values must never
// reach the logs, even at TRACE/DEBUG verbosity. Lookups use the canonical
// MIME form so "authorization", "AUTHORIZATION" and "Authorization" are all
// treated identically, matching how net/http canonicalises header keys.
var sensitiveHeaderNames = map[string]struct{}{
	"Authorization":       {},
	"Proxy-Authorization": {},
	"Www-Authenticate":    {},
	"X-Api-Key":           {},
	"X-Auth-Token":        {},
	"X-Private-Key":       {},
	"Private-Key":         {},
	"Cookie":              {},
	"Set-Cookie":          {},
	"Signature":           {},
	"X-Signature":         {},
}

// IsSensitiveHeader reports whether the named HTTP header carries credentials
// or other authorization data that must not be logged verbatim.
func IsSensitiveHeader(name string) bool {
	_, ok := sensitiveHeaderNames[textproto.CanonicalMIMEHeaderKey(name)]
	return ok
}

// SanitizeHeaders returns a deep copy of h with every sensitive header value
// replaced by RedactedPlaceholder. Remaining values are passed through
// SanitizeLogString so credentials embedded in free-form header values (e.g.
// a bearer token inside X-Custom-Header) are masked as well. The input header
// is never modified, making the result safe to hand to any logger.
func SanitizeHeaders(h http.Header) http.Header {
	if h == nil {
		return nil
	}

	out := h.Clone()
	for name, values := range out {
		if IsSensitiveHeader(name) {
			redacted := make([]string, len(values))
			for i := range values {
				redacted[i] = RedactedPlaceholder
			}
			out[name] = redacted
			continue
		}
		for i, value := range values {
			out[name][i] = SanitizeLogString(value)
		}
	}
	return out
}

var (
	// pemPrivateKeyRe matches PEM-encoded private key blocks (PKCS#1, PKCS#8,
	// EC, OPENSSH). The (?s) flag lets the lazy dot span the embedded base64
	// body and newline characters.
	pemPrivateKeyRe = regexp.MustCompile(
		`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`)

	// strKeySecretRe matches Stellar strkey secret seeds (an "S" followed by 55
	// base32 characters). Public keys start with "G" and account/contract IDs
	// with "C", so this pattern is unambiguous and safe to apply everywhere.
	strKeySecretRe = regexp.MustCompile(`\bS[A-Z2-7]{55}\b`)

	// authHeaderRe masks the value of an authorization-style header whenever
	// the header is printed verbatim (e.g. inside a verbose error string or a
	// request dump). It accepts both the plain "Authorization: <value>" form
	// and the JSON-encoded `"authorization":"<value>"` form, and masks the
	// entire value up to the end of the line, closing quote, or JSON object
	// boundary.
	authHeaderRe = regexp.MustCompile(
		`(?i)((?:proxy-)?(?:authorization|www-authenticate)"?[[:space:]]*[:=][[:space:]]*"?)[^"\r\n}]*`)

	// bearerRe masks RFC 6750 bearer tokens that appear without a labelled
	// header name (e.g. an error message that quotes the raw token). The
	// scheme prefix is preserved so the log stays interpretable.
	bearerRe = regexp.MustCompile(
		`(?i)(bearer[[:space:]]+)[A-Za-z0-9\-._~+/]+=*`)

	// labeledSecretRe masks values assigned to secret-looking keys in
	// "key=value", "key: value", or JSON `"key": "value"` forms. It covers
	// env-var style assignments (ERST_PKCS11_PIN=..., ERST_SOFTWARE_PRIVATE_KEY_HEX=...),
	// structured log fields and JSON payloads. The value capture stops at
	// whitespace, quotes and common delimiters so surrounding context is
	// preserved; long hexadecimal private keys are only masked here, when
	// labelled, to keep legitimate transaction and contract hashes visible in
	// the logs.
	labeledSecretRe = regexp.MustCompile(
		`(?i)("?(?:api[-_]?key|client[-_]?secret|pass(?:word)?|pin|private[-_]?key|secret|seed|signing[-_]?key|signature|token)(?:[-_](?:hex|pem|b64|base64|env|var|file|path|phrase|id))?"?[[:space:]]*[:=][[:space:]]*["']?)([^"'[:space:],;&}\r\n]*)`)
)

// SanitizeLogString masks sensitive authorization data and private key
// material in an arbitrary log string before it is written to stdout/stderr.
//
// The following are detected and replaced with RedactedPlaceholder:
//   - PEM-encoded private key blocks
//   - Stellar strkey secret seeds ("S..." + 55 base32 characters)
//   - Authorization / Proxy-Authorization / WWW-Authenticate header values,
//     in both plain and JSON-encoded form
//   - Bearer tokens quoted in free-form text (the "Bearer " scheme is kept)
//   - Values assigned to secret-looking labels (private_key, secret, seed,
//     pin, token, signature, ...), including long hex private keys
//
// Unlabelled hexadecimal strings are intentionally left untouched so that
// transaction hashes, ledger hashes and contract IDs remain visible for
// debugging. The function is deterministic and idempotent: sanitizing an
// already-sanitized string returns it unchanged.
func SanitizeLogString(s string) string {
	if s == "" {
		return s
	}

	if pemPrivateKeyRe.MatchString(s) {
		s = pemPrivateKeyRe.ReplaceAllString(s, RedactedPlaceholder)
	}
	if strKeySecretRe.MatchString(s) {
		s = strKeySecretRe.ReplaceAllString(s, RedactedPlaceholder)
	}
	if authHeaderRe.MatchString(s) {
		s = authHeaderRe.ReplaceAllString(s, `${1}`+RedactedPlaceholder)
	}
	if bearerRe.MatchString(s) {
		s = bearerRe.ReplaceAllString(s, `${1}`+RedactedPlaceholder)
	}
	if labeledSecretRe.MatchString(s) {
		s = labeledSecretRe.ReplaceAllString(s, `${1}`+RedactedPlaceholder)
	}
	return s
}
