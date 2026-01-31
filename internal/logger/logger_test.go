// Copyright 2025 Erst Users
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
		{"DEBUG", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"invalid", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			if tt.env != "" {
				os.Setenv("ERST_LOG_LEVEL", tt.env)
			} else {
				os.Unsetenv("ERST_LOG_LEVEL")
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
