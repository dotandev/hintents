// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package report_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dotandev/hintents/internal/audit"
	"github.com/dotandev/hintents/internal/report"
)

func auditDumpFixture() *audit.Dump {
	return &audit.Dump{
		Input: map[string]interface{}{
			"amount":   100,
			"currency": "USD",
			"user_id":  "u_123",
		},
		State: map[string]interface{}{
			"balance_before": 500,
			"balance_after":  400,
		},
		Events:    []interface{}{"INIT_TRANSFER", "DEBIT_ACCOUNT", "FEE_CALC"},
		Timestamp: "2026-02-24T12:00:00.000Z",
	}
}

func signedDumpFixture() *audit.SignedDump {
	return &audit.SignedDump{
		Trace:     *auditDumpFixture(),
		Hash:      "abc123deadbeef",
		Signature: "sig0011223344",
		Algorithm: "Ed25519+SHA256",
		PublicKey: "-----BEGIN PUBLIC KEY-----\nMCowBQYDK2VwAyEA...\n-----END PUBLIC KEY-----",
		Signer: struct {
			Provider string `json:"provider"`
		}{Provider: "mock"},
	}
}

func TestFromDump_BasicFields(t *testing.T) {
	dump := auditDumpFixture()
	r := report.FromDump(dump)

	assert.Equal(t, "Audit Report", r.Title)
	assert.NotNil(t, r.Summary)
	assert.Equal(t, 3, r.Summary.TotalEvents)
	assert.Equal(t, "complete", r.Summary.Status)
}

func TestFromDump_ExecutionSteps(t *testing.T) {
	dump := auditDumpFixture()
	r := report.FromDump(dump)

	require.NotNil(t, r.Execution)
	// Expect: 1 input step + 3 event steps + 1 state-snapshot step = 5
	assert.Len(t, r.Execution.Steps, 5)
	assert.Equal(t, "input", r.Execution.Steps[0].Operation)
	assert.Equal(t, "event", r.Execution.Steps[1].Operation)
	assert.Equal(t, "state-snapshot", r.Execution.Steps[4].Operation)
}

func TestFromDump_EventDistribution(t *testing.T) {
	dump := auditDumpFixture()
	r := report.FromDump(dump)

	require.NotNil(t, r.Analytics)
	dist := r.Analytics.EventDistribution
	assert.Equal(t, 1, dist["INIT_TRANSFER"])
	assert.Equal(t, 1, dist["DEBIT_ACCOUNT"])
	assert.Equal(t, 1, dist["FEE_CALC"])
}

func TestFromDump_Timestamp(t *testing.T) {
	dump := auditDumpFixture()
	r := report.FromDump(dump)

	assert.Equal(t, 2026, r.GeneratedAt.Year())
	assert.Equal(t, 2, int(r.GeneratedAt.Month()))
	assert.Equal(t, 24, r.GeneratedAt.Day())
}

func TestFromDump_EmptyEvents(t *testing.T) {
	dump := &audit.Dump{
		Input:     map[string]interface{}{"x": 1},
		State:     map[string]interface{}{},
		Events:    []interface{}{},
		Timestamp: "2026-02-24T00:00:00Z",
	}
	r := report.FromDump(dump)
	assert.Equal(t, 0, r.Summary.TotalEvents)
}

func TestFromSignedDump_Title(t *testing.T) {
	dump := signedDumpFixture()
	r := report.FromSignedDump(dump)

	assert.Equal(t, "Signed Audit Report", r.Title)
}

func TestFromSignedDump_Tags(t *testing.T) {
	dump := signedDumpFixture()
	r := report.FromSignedDump(dump)

	require.NotNil(t, r.Metadata.Tags)
	assert.Equal(t, "Ed25519+SHA256", r.Metadata.Tags["algorithm"])
	assert.Equal(t, "abc123deadbeef", r.Metadata.Tags["hash"])
	assert.Equal(t, "mock", r.Metadata.Tags["signer"])
}

func TestParseDump_ValidJSON(t *testing.T) {
	raw := `{
		"input":  {"amount": 50},
		"state":  {"ok": true},
		"events": ["E1", "E2"],
		"timestamp": "2026-01-01T00:00:00Z"
	}`
	dump, err := report.ParseDump([]byte(raw))
	require.NoError(t, err)
	assert.Equal(t, 2, len(dump.Events))
}

func TestParseDump_InvalidJSON(t *testing.T) {
	_, err := report.ParseDump([]byte("not json"))
	assert.Error(t, err)
}

func TestParseSignedDump_ValidJSON(t *testing.T) {
	inner := auditDumpFixture()
	data, err := json.Marshal(map[string]interface{}{
		"trace":     inner,
		"hash":      "h",
		"signature": "s",
		"algorithm": "Ed25519+SHA256",
		"publicKey": "pk",
		"signer":    map[string]string{"provider": "software"},
	})
	require.NoError(t, err)

	dump, err := report.ParseSignedDump(data)
	require.NoError(t, err)
	assert.Equal(t, "Ed25519+SHA256", dump.Algorithm)
	assert.Equal(t, "software", dump.Signer.Provider)
}

func TestRenderDumpHTML_RawTrace(t *testing.T) {
	raw := `{
		"input":  {"amount": 100, "currency": "USD"},
		"state":  {"balance": 400},
		"events": ["INIT", "COMPLETE"],
		"timestamp": "2026-02-24T00:00:00Z"
	}`
	html, err := report.RenderDumpHTML([]byte(raw))
	require.NoError(t, err)
	body := string(html)
	assert.True(t, strings.Contains(body, "<!DOCTYPE html>"))
	assert.True(t, strings.Contains(body, "Audit Report"))
}

func TestRenderDumpHTML_SignedTrace(t *testing.T) {
	inner := auditDumpFixture()
	data, err := json.Marshal(map[string]interface{}{
		"trace":     inner,
		"hash":      "deadbeef",
		"signature": "sig",
		"algorithm": "Ed25519+SHA256",
		"publicKey": "pk",
		"signer":    map[string]string{"provider": "software"},
	})
	require.NoError(t, err)

	html, err := report.RenderDumpHTML(data)
	require.NoError(t, err)
	body := string(html)
	assert.True(t, strings.Contains(body, "<!DOCTYPE html>"))
	assert.True(t, strings.Contains(body, "Signed Audit Report"))
}

func TestRenderDumpHTML_InvalidJSON(t *testing.T) {
	_, err := report.RenderDumpHTML([]byte("bad"))
	assert.Error(t, err)
}
