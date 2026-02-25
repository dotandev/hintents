// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/dotandev/hintents/internal/logger"
)

// StreamMessage represents a message received from the Rust simulator
type StreamMessage struct {
	Type string `json:"type"`
	
	// Event fields
	Event *DiagnosticEvent `json:"event,omitempty"`
	
	// Log fields
	Message *string `json:"message,omitempty"`
	
	// Budget update fields
	CPUInstructions *uint64 `json:"cpu_instructions,omitempty"`
	MemoryBytes     *uint64 `json:"memory_bytes,omitempty"`
}

// StreamHandler processes messages as they arrive
type StreamHandler interface {
	OnEvent(event DiagnosticEvent)
	OnLog(message string)
	OnBudgetUpdate(cpu, mem uint64)
	OnComplete()
	OnError(message string)
}

// SocketListener listens for streaming messages from the Rust simulator
type SocketListener struct {
	socketPath string
	listener   net.Listener
	handler    StreamHandler
	mu         sync.Mutex
	closed     bool
	wg         sync.WaitGroup
}

// NewSocketListener creates a new socket listener
func NewSocketListener(handler StreamHandler) (*SocketListener, error) {
	// Create temporary socket path
	tmpDir := os.TempDir()
	socketPath := filepath.Join(tmpDir, fmt.Sprintf("erst-sim-%d.sock", os.Getpid()))
	
	// Remove existing socket if present
	if err := os.RemoveAll(socketPath); err != nil {
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}
	
	// Create Unix domain socket listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket listener: %w", err)
	}
	
	logger.Logger.Debug("Socket listener created", "path", socketPath)
	
	return &SocketListener{
		socketPath: socketPath,
		listener:   listener,
		handler:    handler,
	}, nil
}

// GetSocketPath returns the path to the Unix domain socket
func (sl *SocketListener) GetSocketPath() string {
	return sl.socketPath
}

// Start begins accepting connections and processing messages
func (sl *SocketListener) Start() {
	sl.wg.Add(1)
	go sl.acceptLoop()
}

// acceptLoop accepts incoming connections
func (sl *SocketListener) acceptLoop() {
	defer sl.wg.Done()
	
	for {
		conn, err := sl.listener.Accept()
		if err != nil {
			sl.mu.Lock()
			closed := sl.closed
			sl.mu.Unlock()
			
			if closed {
				return
			}
			
			logger.Logger.Error("Failed to accept connection", "error", err)
			continue
		}
		
		// Handle connection in a goroutine
		sl.wg.Add(1)
		go sl.handleConnection(conn)
	}
}

// handleConnection processes messages from a single connection
func (sl *SocketListener) handleConnection(conn net.Conn) {
	defer sl.wg.Done()
	defer conn.Close()
	
	logger.Logger.Debug("Simulator connected to socket")
	
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		
		var msg StreamMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			logger.Logger.Warn("Failed to parse stream message", "error", err, "line", line)
			continue
		}
		
		sl.dispatchMessage(&msg)
	}
	
	if err := scanner.Err(); err != nil {
		logger.Logger.Error("Error reading from socket", "error", err)
	}
	
	logger.Logger.Debug("Simulator disconnected from socket")
}

// dispatchMessage routes a message to the appropriate handler method
func (sl *SocketListener) dispatchMessage(msg *StreamMessage) {
	switch msg.Type {
	case "event":
		if msg.Event != nil {
			sl.handler.OnEvent(*msg.Event)
		}
	case "log":
		if msg.Message != nil {
			sl.handler.OnLog(*msg.Message)
		}
	case "budget_update":
		if msg.CPUInstructions != nil && msg.MemoryBytes != nil {
			sl.handler.OnBudgetUpdate(*msg.CPUInstructions, *msg.MemoryBytes)
		}
	case "complete":
		sl.handler.OnComplete()
	case "error":
		if msg.Message != nil {
			sl.handler.OnError(*msg.Message)
		}
	default:
		logger.Logger.Warn("Unknown stream message type", "type", msg.Type)
	}
}

// Close stops the listener and cleans up resources
func (sl *SocketListener) Close() error {
	sl.mu.Lock()
	if sl.closed {
		sl.mu.Unlock()
		return nil
	}
	sl.closed = true
	sl.mu.Unlock()
	
	// Close listener to stop accepting new connections
	if err := sl.listener.Close(); err != nil {
		logger.Logger.Warn("Error closing listener", "error", err)
	}
	
	// Wait for all goroutines to finish
	sl.wg.Wait()
	
	// Remove socket file
	if err := os.Remove(sl.socketPath); err != nil && !os.IsNotExist(err) {
		logger.Logger.Warn("Failed to remove socket file", "error", err, "path", sl.socketPath)
	}
	
	logger.Logger.Debug("Socket listener closed", "path", sl.socketPath)
	return nil
}

// DefaultStreamHandler is a simple handler that collects all messages
type DefaultStreamHandler struct {
	Events        []DiagnosticEvent
	Logs          []string
	LastCPU       uint64
	LastMemory    uint64
	Completed     bool
	Error         string
	mu            sync.Mutex
}

// NewDefaultStreamHandler creates a new default handler
func NewDefaultStreamHandler() *DefaultStreamHandler {
	return &DefaultStreamHandler{
		Events: make([]DiagnosticEvent, 0),
		Logs:   make([]string, 0),
	}
}

func (h *DefaultStreamHandler) OnEvent(event DiagnosticEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Events = append(h.Events, event)
	logger.Logger.Debug("Received event", "type", event.EventType)
}

func (h *DefaultStreamHandler) OnLog(message string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Logs = append(h.Logs, message)
	logger.Logger.Debug("Received log", "message", message)
}

func (h *DefaultStreamHandler) OnBudgetUpdate(cpu, mem uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.LastCPU = cpu
	h.LastMemory = mem
	logger.Logger.Debug("Budget update", "cpu", cpu, "memory", mem)
}

func (h *DefaultStreamHandler) OnComplete() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Completed = true
	logger.Logger.Info("Simulation completed")
}

func (h *DefaultStreamHandler) OnError(message string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Error = message
	logger.Logger.Error("Simulation error", "error", message)
}

// GetResults returns the collected data
func (h *DefaultStreamHandler) GetResults() ([]DiagnosticEvent, []string, uint64, uint64, bool, string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.Events, h.Logs, h.LastCPU, h.LastMemory, h.Completed, h.Error
}
