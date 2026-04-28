// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
)

var (
	Logger *slog.Logger
	level  = new(slog.LevelVar)
	mu     sync.Mutex
)

// RedactPIN replaces any HSM PIN values in the input string with "*****".
// This is used to prevent sensitive credentials from appearing in logs.
func RedactPIN(s string) string {
	// Pattern 1: ERST_PKCS11_PIN=1234 or ERST_PKCS11_PIN: 1234
	pattern1 := regexp.MustCompile(`ERST_PKCS11_PIN[=:]\s*[^\s"']+`)
	s = pattern1.ReplaceAllString(s, "ERST_PKCS11_PIN=*****")

	// Pattern 2: JSON-style "pin": "1234" or 'pin': '1234'
	pattern2 := regexp.MustCompile(`(["']pin["']\s*[:=]\s*["'])([^"']+)`)
	s = pattern2.ReplaceAllString(s, `$1*****`)

	return s
}

// redactingWriter wraps an io.Writer and redacts PIN values from all output.
type redactingWriter struct {
	w io.Writer
}

func (rw *redactingWriter) Write(p []byte) (n int, err error) {
	redacted := RedactPIN(string(p))
	return rw.w.Write([]byte(redacted))
}

// Custom log levels
const (
	LevelTrace = slog.Level(-8) // More verbose than Debug (-4)
)

func init() {
	lvl := parseLevelFromEnv()
	initLogger(lvl, os.Stderr, false)
}

func parseLevelFromEnv() slog.Level {
	env := strings.ToUpper(os.Getenv("ERST_LOG_LEVEL"))
	return ParseLevel(env)
}

// ParseLevel converts a string to a slog.Level
func ParseLevel(levelStr string) slog.Level {
	switch strings.ToUpper(levelStr) {
	case "TRACE":
		return LevelTrace
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ParseLogLevel converts a human-readable level string (e.g. "debug", "info",
// "warn", "error") into the corresponding slog.Level. Unknown values default
// to slog.LevelInfo.
func ParseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace", "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// RustLogFilter returns the RUST_LOG-compatible filter string that corresponds
// to the given ERST log level name. This is used when spawning the Rust
// simulator subprocess so that a single ERST_LOG_LEVEL value controls both the
// Go logger and the Rust tracing subscriber.
func RustLogFilter(erstLevel string) string {
	switch strings.ToLower(strings.TrimSpace(erstLevel)) {
	case "trace":
		return "trace"
	case "debug":
		return "debug"
	case "info":
		return "info"
	case "warn", "warning":
		return "warn"
	case "error":
		return "error"
	default:
		return "info"
	}
}

func initLogger(lvl slog.Level, w io.Writer, useJSON bool) {
	if w == nil {
		w = os.Stderr
	}

	level.Set(lvl)

	// Wrap the writer with redactingWriter to scrub PIN values
	w = &redactingWriter{w: w}

	var handler slog.Handler
	if useJSON {
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		})
	} else {
		handler = NewTextHandler(w, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		})
	}

	Logger = slog.New(handler)
}

func SetLevel(lvl slog.Level) {
	mu.Lock()
	defer mu.Unlock()
	level.Set(lvl)
}

func SetOutput(w io.Writer, useJSON bool) {
	mu.Lock()
	defer mu.Unlock()
	initLogger(level.Level(), w, useJSON)
}

type TextHandler struct {
	handler slog.Handler
}

func NewTextHandler(w io.Writer, opts *slog.HandlerOptions) *TextHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &TextHandler{
		handler: slog.NewTextHandler(w, opts),
	}
}

func (h *TextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *TextHandler) Handle(ctx context.Context, record slog.Record) error {
	// Use the underlying handler directly - redaction happens at the string level
	// through the RedactPIN function which can be called by loggers when needed
	return h.handler.Handle(ctx, record)
}

func (h *TextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TextHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h *TextHandler) WithGroup(name string) slog.Handler {
	return &TextHandler{handler: h.handler.WithGroup(name)}
}

// Trace logs at trace level (more verbose than debug)
func Trace(msg string, args ...any) {
	Logger.Log(context.Background(), LevelTrace, msg, args...)
}

// GetRustLogLevel returns the Rust env_logger compatible log level string
func GetRustLogLevel() string {
	currentLevel := level.Level()
	switch {
	case currentLevel <= LevelTrace:
		return "trace"
	case currentLevel <= slog.LevelDebug:
		return "debug"
	case currentLevel <= slog.LevelInfo:
		return "info"
	case currentLevel <= slog.LevelWarn:
		return "warn"
	default:
		return "error"
	}
}

// GetRustLogFormat returns the format for Rust logger (json or text)
func GetRustLogFormat() string {
	// Check if we're using JSON format by inspecting the handler
	// For now, we'll use an environment variable or default to text
	if format := os.Getenv("ERST_LOG_FORMAT"); format == "json" {
		return "json"
	}
	return "text"
}
