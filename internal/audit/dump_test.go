// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validTrace returns a JSON-shaped trace matching the TS ExecutionTrace type.
// Values use types that survive a JSON round-trip unchanged (strings, bools,
// arrays of strings), so re-encoded output can be compared byte-for-byte.
func validTrace() map[string]interface{} {
	return map[string]interface{}{
		"input":     map[string]interface{}{"amount": float64(50), "currency": "USD"},
		"state":     map[string]interface{}{"ok": true},
		"events":    []interface{}{"E1", "E2"},
		"timestamp": "2026-01-01T00:00:00Z",
	}
}

// validSignedDump returns a JSON-shaped SignedAuditLog matching the TS type.
func validSignedDump() map[string]interface{} {
	return map[string]interface{}{
		"trace":     validTrace(),
		"hash":      "abc123",
		"signature": "sig00",
		"algorithm": "Ed25519+SHA256",
		"publicKey": "pk",
		"signer":    map[string]interface{}{"provider": "software"},
	}
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestParseDump_RoundTrip(t *testing.T) {
	original := &Dump{
		Input:     map[string]interface{}{"amount": float64(100), "currency": "USD"},
		State:     map[string]interface{}{"balance": float64(400)},
		Events:    []interface{}{"INIT", "COMPLETE"},
		Timestamp: "2026-02-24T12:00:00.000Z",
	}

	encoded, err := json.Marshal(original)
	require.NoError(t, err)

	parsed, err := ParseDump(encoded)
	require.NoError(t, err)

	assert.Equal(t, original, parsed)

	reencoded, err := json.Marshal(parsed)
	require.NoError(t, err)
	assert.JSONEq(t, string(encoded), string(reencoded))
}

func TestParseDump_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		mutate func(m map[string]interface{})
	}{
		{name: "invalid JSON", data: []byte(`{"input": `)},
		{name: "empty payload", data: []byte(``)},
		{name: "top-level array", data: []byte(`[]`)},
		{name: "top-level null", data: []byte(`null`)},
		{name: "missing input", mutate: func(m map[string]interface{}) { delete(m, "input") }},
		{name: "missing state", mutate: func(m map[string]interface{}) { delete(m, "state") }},
		{name: "missing events", mutate: func(m map[string]interface{}) { delete(m, "events") }},
		{name: "missing timestamp", mutate: func(m map[string]interface{}) { delete(m, "timestamp") }},
		{name: "null input", mutate: func(m map[string]interface{}) { m["input"] = nil }},
		{name: "null events", mutate: func(m map[string]interface{}) { m["events"] = nil }},
		{name: "input is a string", mutate: func(m map[string]interface{}) { m["input"] = "oops" }},
		{name: "state is an array", mutate: func(m map[string]interface{}) { m["state"] = []interface{}{} }},
		{name: "events is an object", mutate: func(m map[string]interface{}) { m["events"] = map[string]interface{}{} }},
		{name: "timestamp is a number", mutate: func(m map[string]interface{}) { m["timestamp"] = float64(1735689600) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.data
			if tt.mutate != nil {
				m := validTrace()
				tt.mutate(m)
				data = mustMarshal(t, m)
			}

			dump, err := ParseDump(data)
			assert.Error(t, err)
			assert.Nil(t, dump, "must not return a zero-valued struct on error")
		})
	}
}

func TestParseDump_Valid(t *testing.T) {
	dump, err := ParseDump(mustMarshal(t, validTrace()))
	require.NoError(t, err)
	require.NotNil(t, dump)
	assert.Equal(t, "2026-01-01T00:00:00Z", dump.Timestamp)
	assert.Equal(t, []interface{}{"E1", "E2"}, dump.Events)
	assert.Equal(t, true, dump.State["ok"])
}

func TestParseSignedDump_Golden(t *testing.T) {
	data, err := os.ReadFile("testdata/signed_dump.json")
	require.NoError(t, err)

	dump, err := ParseSignedDump(data)
	require.NoError(t, err)
	require.NotNil(t, dump)

	assert.Equal(t, "3a13a2cd6c97fcc89034b8eb52e2e59b71618f52c6cade039c89284b49851409", dump.Hash)
	assert.Equal(t, "Ed25519+SHA256", dump.Algorithm)
	assert.Equal(t, "software", dump.Signer.Provider)
	assert.Contains(t, dump.PublicKey, "BEGIN PUBLIC KEY")

	assert.Equal(t, "2026-02-24T12:00:00.000Z", dump.Trace.Timestamp)
	assert.Equal(t, []interface{}{"INIT_TRANSFER", "DEBIT_ACCOUNT", "FEE_CALC"}, dump.Trace.Events)
	assert.Equal(t, "u_123", dump.Trace.Input["user_id"])
	assert.Equal(t, float64(400), dump.Trace.State["balance_after"])
}

func TestParseSignedDump_RoundTrip(t *testing.T) {
	encoded := mustMarshal(t, validSignedDump())

	parsed, err := ParseSignedDump(encoded)
	require.NoError(t, err)

	reencoded, err := json.Marshal(parsed)
	require.NoError(t, err)
	assert.JSONEq(t, string(encoded), string(reencoded))
}

func TestParseSignedDump_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		mutate func(m map[string]interface{})
	}{
		{name: "invalid JSON", data: []byte(`{"trace": `)},
		{name: "top-level array", data: []byte(`[]`)},
		{name: "top-level null", data: []byte(`null`)},
		{name: "missing trace", mutate: func(m map[string]interface{}) { delete(m, "trace") }},
		{name: "missing hash", mutate: func(m map[string]interface{}) { delete(m, "hash") }},
		{name: "missing signature", mutate: func(m map[string]interface{}) { delete(m, "signature") }},
		{name: "missing algorithm", mutate: func(m map[string]interface{}) { delete(m, "algorithm") }},
		{name: "missing publicKey", mutate: func(m map[string]interface{}) { delete(m, "publicKey") }},
		{name: "missing signer", mutate: func(m map[string]interface{}) { delete(m, "signer") }},
		{name: "missing signer.provider", mutate: func(m map[string]interface{}) {
			m["signer"] = map[string]interface{}{"label": "x"}
		}},
		{name: "null trace", mutate: func(m map[string]interface{}) { m["trace"] = nil }},
		{name: "null signer", mutate: func(m map[string]interface{}) { m["signer"] = nil }},
		{name: "trace missing events", mutate: func(m map[string]interface{}) {
			delete(m["trace"].(map[string]interface{}), "events")
		}},
		{name: "trace timestamp is a number", mutate: func(m map[string]interface{}) {
			m["trace"].(map[string]interface{})["timestamp"] = float64(1)
		}},
		{name: "hash is a number", mutate: func(m map[string]interface{}) { m["hash"] = float64(42) }},
		{name: "signer is a string", mutate: func(m map[string]interface{}) { m["signer"] = "software" }},
		{name: "signer.provider is a number", mutate: func(m map[string]interface{}) {
			m["signer"] = map[string]interface{}{"provider": float64(1)}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.data
			if tt.mutate != nil {
				m := validSignedDump()
				tt.mutate(m)
				data = mustMarshal(t, m)
			}

			dump, err := ParseSignedDump(data)
			assert.Error(t, err)
			assert.Nil(t, dump, "must not return a zero-valued struct on error")
		})
	}
}

func TestParseSignedDump_Valid(t *testing.T) {
	dump, err := ParseSignedDump(mustMarshal(t, validSignedDump()))
	require.NoError(t, err)
	require.NotNil(t, dump)
	assert.Equal(t, "abc123", dump.Hash)
	assert.Equal(t, "software", dump.Signer.Provider)
	assert.Equal(t, "2026-01-01T00:00:00Z", dump.Trace.Timestamp)
}
