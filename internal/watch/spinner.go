// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"sync"
	"time"

	"github.com/dotandev/hintents/internal/terminal"
)

type Spinner struct {
	renderer  terminal.Renderer
	frames    []string
	current   int
	done      chan struct{}
	mu        sync.Mutex
	isRunning bool
}

func NewSpinner() *Spinner {
	return &Spinner{
		renderer: terminal.NewANSIRenderer(),
		frames:   []string{"|", "/", "-", "\\"},
		done:     make(chan struct{}),
	}
}

func (s *Spinner) WithRenderer(r terminal.Renderer) *Spinner {
	s.renderer = r
	return s
}

func (s *Spinner) Start(message string) {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = true
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.done:
				s.renderer.ClearLine()
				return
			case <-ticker.C:
				s.mu.Lock()
				s.renderer.Printf("\r%s %s", s.frames[s.current], message)
				s.current = (s.current + 1) % len(s.frames)
				s.mu.Unlock()
			}
		}
	}()
}

func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = false
	s.mu.Unlock()

	select {
	case s.done <- struct{}{}:
	default:
	}

	time.Sleep(50 * time.Millisecond)
}

func (s *Spinner) StopWithMessage(message string) {
	s.Stop()
	s.renderer.Printf("\r[OK] %s\n", message)
}

func (s *Spinner) StopWithError(message string) {
	s.Stop()
	s.renderer.Printf("\r[ERROR] %s\n", message)
}
