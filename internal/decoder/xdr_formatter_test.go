// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package decoder

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewXDRFormatter(t *testing.T) {
	tests := []struct {
		name   string
		format FormatType
	}{
		{"JSON format", FormatJSON},
		{"Table format", FormatTable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewXDRFormatter(tt.format)
			if formatter == nil {
				t.Fatal("expected non-nil formatter")
				return
			}
			if formatter.format != tt.format {
				t.Errorf("expected format %s, got %s", tt.format, formatter.format)
			}
		})
	}
}

func TestFormatJSON(t *testing.T) {
	formatter := NewXDRFormatter(FormatJSON)

	data := map[string]interface{}{
		"type":     "account",
		"balance":  1000000,
		"sequence": 42,
	}

	output, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "type") {
		t.Error("expected JSON to contain 'type' key")
	}

	if !strings.Contains(output, "1000000") {
		t.Error("expected JSON to contain balance value")
	}

	var unmarshaled map[string]interface{}
	if err := json.Unmarshal([]byte(output), &unmarshaled); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestFormatTable(t *testing.T) {
	formatter := NewXDRFormatter(FormatTable)

	data := map[string]interface{}{
		"type":    "account",
		"balance": 1000000,
	}

	output, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output == "" {
		t.Error("expected non-empty table output")
	}
}

func TestFormatUnsupported(t *testing.T) {
	formatter := &XDRFormatter{format: "invalid"}

	_, err := formatter.Format(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for unsupported format")
	}

	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("expected unsupported format error, got: %v", err)
	}
}

func TestSummarizeXDRObject(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		contains string
	}{
		{"nil ledger entry", nil, ""},
		{"unknown type", 123, "int"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SummarizeXDRObject(tt.input)
			if result == "" {
				t.Error("expected non-empty summary")
			}
		})
	}
}

func TestXDRFormatterWithSlice(t *testing.T) {
	formatter := NewXDRFormatter(FormatTable)

	items := []interface{}{
		map[string]string{"id": "1", "type": "account"},
		map[string]string{"id": "2", "type": "offer"},
	}

	output, err := formatter.Format(items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Items") {
		t.Error("expected table to show items count")
	}
}

func TestFormatEmptyData(t *testing.T) {
	formatter := NewXDRFormatter(FormatJSON)

	data := map[string]interface{}{}

	output, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output != "{}" {
		t.Errorf("expected '{}', got %q", output)
	}
}

func TestJSONFormattingIsValid(t *testing.T) {
	formatter := NewXDRFormatter(FormatJSON)

	testCases := []interface{}{
		map[string]interface{}{"a": 1, "b": "test"},
		[]interface{}{1, 2, 3},
		struct{ Name string }{Name: "test"},
	}

	for _, tc := range testCases {
		output, err := formatter.Format(tc)
		if err != nil {
			t.Fatalf("formatting failed: %v", err)
		}

		var result interface{}
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Errorf("output is not valid JSON: %v", err)
		}
	}
}

func TestTableFormatWritesHeaders(t *testing.T) {
	formatter := NewXDRFormatter(FormatTable)

	data := map[string]interface{}{
		"field1": "value1",
		"field2": 42,
	}

	output, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output == "" {
		t.Error("expected table output")
	}
}

func BenchmarkFormatJSON(b *testing.B) {
	formatter := NewXDRFormatter(FormatJSON)
	data := map[string]interface{}{
		"type":      "account",
		"balance":   1000000,
		"sequence":  42,
		"flags":     256,
		"threshold": 1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = formatter.Format(data)
	}
}

func BenchmarkFormatTable(b *testing.B) {
	formatter := NewXDRFormatter(FormatTable)
	data := map[string]interface{}{
		"type":     "account",
		"balance":  1000000,
		"sequence": 42,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = formatter.Format(data)
	}
}
