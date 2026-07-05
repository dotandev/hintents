// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestParseLevelFromEnv(t *testing.T) {
	tests := []struct {
		env      string
		expected slog.Level
	}{
		{"TRACE", LevelTrace},
		{"DEBUG", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"trace", LevelTrace},
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"invalid", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			if tt.env != "" {
				_ = os.Setenv("ERST_LOG_LEVEL", tt.env)
			} else {
				_ = os.Unsetenv("ERST_LOG_LEVEL")
			}
			lvl := parseLevelFromEnv()
			if lvl != tt.expected {
				t.Errorf("parseLevelFromEnv(%q) = %v, want %v", tt.env, lvl, tt.expected)
			}
		})
	}
	os.Unsetenv("ERST_LOG_LEVEL")
}

func TestLoggerInitialization(t *testing.T) {
	if Logger == nil {
		t.Fatal("Logger should be initialized after package init")
	}
}

func TestSetLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf, false)

	SetLevel(slog.LevelDebug)
	if level.Level() != slog.LevelDebug {
		t.Errorf("SetLevel(Debug) failed: got %v", level.Level())
	}

	SetLevel(slog.LevelError)
	if level.Level() != slog.LevelError {
		t.Errorf("SetLevel(Error) failed: got %v", level.Level())
	}
}

func TestLogLevelFiltering(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf, false)
	SetLevel(slog.LevelWarn)

	Logger.Debug("debug message")
	Logger.Info("info message")
	Logger.Warn("warn message")
	Logger.Error("error message")

	output := buf.String()
	if strings.Contains(output, "debug") {
		t.Error("debug message should be filtered at WARN level")
	}
	if strings.Contains(output, "info") {
		t.Error("info message should be filtered at WARN level")
	}
	if !strings.Contains(output, "warn") {
		t.Error("warn message should appear at WARN level")
	}
	if !strings.Contains(output, "error") {
		t.Error("error message should appear at WARN level")
	}
}

func TestTextOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf, false)
	SetLevel(slog.LevelDebug)

	Logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Error("message not found in output")
	}
	if !strings.Contains(output, "key") {
		t.Error("attribute key not found in output")
	}
}

func TestJSONOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf, true)
	SetLevel(slog.LevelDebug)

	Logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Error("message not found in JSON output")
	}
	if !strings.Contains(output, "\"msg\"") {
		t.Error("msg field not found in JSON output")
	}
	if !strings.Contains(output, "key") {
		t.Error("attribute key not found in JSON output")
	}
}

func TestLoggerConcurrency(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf, false)
	SetLevel(slog.LevelDebug)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			Logger.Info("concurrent log", "id", id)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	output := buf.String()
	if output == "" {
		t.Error("no output from concurrent logging")
	}
}

func TestSetOutputWithNilWriter(t *testing.T) {
	defer func() {
		buf := &bytes.Buffer{}
		SetOutput(buf, false)
	}()

	SetLevel(slog.LevelInfo)
	Logger.Info("test")
}

func TestTextHandlerWithAttrs(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf, false)
	SetLevel(slog.LevelDebug)

	Logger.Info("message with context", "ctx_key", "ctx_value")

	output := buf.String()
	if !strings.Contains(output, "message with context") {
		t.Error("message not found")
	}
	if !strings.Contains(output, "ctx_key") {
		t.Error("context key not found")
	}
}

func TestLoggerAttributes(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf, false)
	SetLevel(slog.LevelDebug)

	Logger.Info("test",
		"string", "value",
		"int", 42,
		"bool", true,
	)

	output := buf.String()
	if !strings.Contains(output, "test") {
		t.Error("message not found")
	}
	if !strings.Contains(output, "string") {
		t.Error("string attribute not found")
	}
	if !strings.Contains(output, "value") {
		t.Error("string value not found")
	}
}

func TestErrorLogging(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf, false)
	SetLevel(slog.LevelDebug)

	Logger.Error("error occurred", "error", "test error")

	output := buf.String()
	if !strings.Contains(output, "error occurred") {
		t.Error("error message not found")
	}
}

func BenchmarkLogging(b *testing.B) {
	buf := &bytes.Buffer{}
	SetOutput(buf, false)
	SetLevel(slog.LevelInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Logger.Info("benchmark", "iteration", i)
	}
}

func BenchmarkJSONLogging(b *testing.B) {
	buf := &bytes.Buffer{}
	SetOutput(buf, true)
	SetLevel(slog.LevelInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Logger.Info("benchmark", "iteration", i)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"trace", LevelTrace},
		{"TRACE", LevelTrace},
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"invalid", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTraceLogging(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf, false)
	SetLevel(LevelTrace)

	Trace("trace message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "trace message") {
		t.Error("trace message not found in output")
	}
}

func TestTraceLevelFiltering(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf, false)
	SetLevel(slog.LevelDebug)

	Trace("trace message")
	Logger.Debug("debug message")

	output := buf.String()
	if strings.Contains(output, "trace message") {
		t.Error("trace message should be filtered at DEBUG level")
	}
	if !strings.Contains(output, "debug message") {
		t.Error("debug message should appear at DEBUG level")
	}
}

func TestGetRustLogLevel(t *testing.T) {
	tests := []struct {
		goLevel      slog.Level
		expectedRust string
	}{
		{LevelTrace, "trace"},
		{slog.LevelDebug, "debug"},
		{slog.LevelInfo, "info"},
		{slog.LevelWarn, "warn"},
		{slog.LevelError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedRust, func(t *testing.T) {
			SetLevel(tt.goLevel)
			result := GetRustLogLevel()
			if result != tt.expectedRust {
				t.Errorf("GetRustLogLevel() with level %v = %q, want %q", tt.goLevel, result, tt.expectedRust)
			}
		})
	}
}

func TestGetRustLogFormat(t *testing.T) {
	// Test default (text)
	_ = os.Unsetenv("ERST_LOG_FORMAT")
	if format := GetRustLogFormat(); format != "text" {
		t.Errorf("GetRustLogFormat() = %q, want %q", format, "text")
	}

	// Test JSON
	_ = os.Setenv("ERST_LOG_FORMAT", "json")
	if format := GetRustLogFormat(); format != "json" {
		t.Errorf("GetRustLogFormat() with ERST_LOG_FORMAT=json = %q, want %q", format, "json")
	}
	_ = os.Unsetenv("ERST_LOG_FORMAT")
}

func TestLogLevelHierarchy(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf, false)

	// Test at each level
	levels := []struct {
		level    slog.Level
		messages []string
		filtered []string
	}{
		{
			level:    LevelTrace,
			messages: []string{"trace", "debug", "info", "warn", "error"},
			filtered: []string{},
		},
		{
			level:    slog.LevelDebug,
			messages: []string{"debug", "info", "warn", "error"},
			filtered: []string{"trace"},
		},
		{
			level:    slog.LevelInfo,
			messages: []string{"info", "warn", "error"},
			filtered: []string{"trace", "debug"},
		},
		{
			level:    slog.LevelWarn,
			messages: []string{"warn", "error"},
			filtered: []string{"trace", "debug", "info"},
		},
		{
			level:    slog.LevelError,
			messages: []string{"error"},
			filtered: []string{"trace", "debug", "info", "warn"},
		},
	}

	for _, tt := range levels {
		t.Run(tt.level.String(), func(t *testing.T) {
			buf.Reset()
			SetLevel(tt.level)

			Trace("trace message")
			Logger.Debug("debug message")
			Logger.Info("info message")
			Logger.Warn("warn message")
			Logger.Error("error message")

			output := buf.String()

			for _, msg := range tt.messages {
				if !strings.Contains(output, msg+" message") {
					t.Errorf("Expected %q message to appear at level %v", msg, tt.level)
				}
			}

			for _, msg := range tt.filtered {
				if strings.Contains(output, msg+" message") {
					t.Errorf("Expected %q message to be filtered at level %v", msg, tt.level)
				}
			}
		})
	}
}

func TestRedactPIN(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "ERST_PKCS11_PIN=1234",
			expected: "ERST_PKCS11_PIN=*****",
		},
		{
			input:    "ERST_PKCS11_PIN=12345678",
			expected: "ERST_PKCS11_PIN=*****",
		},
		{
			input:    "ERST_PKCS11_PIN: 1234",
			expected: "ERST_PKCS11_PIN=*****",
		},
		{
			input:    "Config: ERST_PKCS11_PIN=secret123",
			expected: "Config: ERST_PKCS11_PIN=*****",
		},
		{
			input:    "No PIN here",
			expected: "No PIN here",
		},
		{
			input:    "ERST_PKCS11_MODULE=/usr/lib/softhsm2.so ERST_PKCS11_PIN=1234",
			expected: "ERST_PKCS11_MODULE=/usr/lib/softhsm2.so ERST_PKCS11_PIN=*****",
		},
		{
			input:    "\"pin\": \"1234\"",
			expected: "\"pin\": \"*****\"",
		},
		{
			input:    "'pin': '1234'",
			expected: "'pin': '*****'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := RedactPIN(tt.input)
			if result != tt.expected {
				t.Errorf("RedactPIN(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRedactingWriter(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "ERST_PKCS11_PIN=1234\n",
			expected: "ERST_PKCS11_PIN=*****\n",
		},
		{
			input:    "Connecting with ERST_PKCS11_PIN=secret\n",
			expected: "Connecting with ERST_PKCS11_PIN=*****\n",
		},
		{
			input:    "No PIN in this log\n",
			expected: "No PIN in this log\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			buf := &bytes.Buffer{}
			rw := &redactingWriter{w: buf}

			_, err := rw.Write([]byte(tt.input))
			if err != nil {
				t.Fatalf("Write() error = %v", err)
			}

			result := buf.String()
			if result != tt.expected {
				t.Errorf("redactingWriter.Write(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoggerRedactsPIN(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf, false)
	SetLevel(slog.LevelDebug)

	Logger.Info("HSM config", "ERST_PKCS11_PIN", "1234")
	Logger.Info("Config loaded", "config", "ERST_PKCS11_PIN=secret123")

	output := buf.String()

	// Verify PIN is redacted
	if strings.Contains(output, "1234") {
		t.Error("PIN value should be redacted from logs")
	}
	if strings.Contains(output, "secret123") {
		t.Error("PIN value should be redacted from logs")
	}
	if !strings.Contains(output, "*****") {
		t.Error("Redaction marker should appear in logs")
	}
}
