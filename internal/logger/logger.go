// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0
<<<<<<< HEAD
=======

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
>>>>>>> f6fee4164e00510feac1e9c06e2ca622d6ef9a31

package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	Logger *slog.Logger
	level  = new(slog.LevelVar)
	mu     sync.Mutex
)

func init() {
	lvl := parseLevelFromEnv()
	initLogger(lvl, os.Stderr, false)
}

func parseLevelFromEnv() slog.Level {
	env := strings.ToUpper(os.Getenv("ERST_LOG_LEVEL"))
	switch env {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func initLogger(lvl slog.Level, w io.Writer, useJSON bool) {
	if w == nil {
		w = os.Stderr
	}

	level.Set(lvl)

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
	return h.handler.Handle(ctx, record)
}

func (h *TextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TextHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h *TextHandler) WithGroup(name string) slog.Handler {
	return &TextHandler{handler: h.handler.WithGroup(name)}
}
