// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
