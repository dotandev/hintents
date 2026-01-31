// Copyright 2026 dotandev
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"fmt"
	"sync"
	"time"
)

type Spinner struct {
	frames    []string
	current   int
	done      chan struct{}
	mu        sync.Mutex
	isRunning bool
}

func NewSpinner() *Spinner {
	return &Spinner{
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		done:   make(chan struct{}),
	}
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
				fmt.Print("\r\033[K")
				return
			case <-ticker.C:
				s.mu.Lock()
				fmt.Printf("\r%s %s", s.frames[s.current], message)
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
	fmt.Printf("\r✓ %s\n", message)
}

func (s *Spinner) StopWithError(message string) {
	s.Stop()
	fmt.Printf("\r✗ %s\n", message)
}
