// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"io"
	"log/slog"
	"os"
)

// Logger is the global logger instance
var Logger *slog.Logger

// Level is the current log level
var Level = new(slog.LevelVar)

func init() {
	// Initialize with a default logger to prevent panics
	Init(slog.LevelInfo, os.Stderr)
}

// Init initializes the logger with the specified level
func Init(level slog.Level, output io.Writer) {
	if output == nil {
		output = os.Stderr
	}

	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level:     Level,
		AddSource: true,
	})

	Logger = slog.New(handler)
	Level.Set(level)
}

// SetLevel changes the log level programmatically
func SetLevel(level slog.Level) {
	Level.Set(level)
}
