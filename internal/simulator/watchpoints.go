// Package simulator provides the core simulation harness for Soroban smart contracts.
package simulator

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// WatchpointEventKind represents the type of ledger state mutation that triggered a watchpoint.
type WatchpointEventKind string

const (
	// WatchpointWrite indicates a ledger key was written.
	WatchpointWrite WatchpointEventKind = "WRITE"
	// WatchpointDelete indicates a ledger key was deleted.
	WatchpointDelete WatchpointEventKind = "DELETE"
)

// WatchpointTrigger describes which mutation kinds should activate a watchpoint.
type WatchpointTrigger uint8

const (
	// TriggerOnWrite activates the watchpoint when the key is written.
	TriggerOnWrite WatchpointTrigger = 1 << iota
	// TriggerOnDelete activates the watchpoint when the key is deleted.
	TriggerOnDelete
	// TriggerOnAny activates the watchpoint on any mutation (write or delete).
	TriggerOnAny WatchpointTrigger = TriggerOnWrite | TriggerOnDelete
)

// WASMFrame represents a single frame in the WASM call stack captured at the
// moment a watchpoint fires.
type WASMFrame struct {
	// Index is the zero-based position of this frame in the call stack,
	// where 0 is the innermost (most recent) frame.
	Index int

	// FunctionName is the demagnled name of the WASM function, if available.
	// Falls back to the raw function index (e.g. "func[42]") when no name section is present.
	FunctionName string

	// InstructionOffset is the byte offset of the WASM instruction within the
	// function body at the time of the watchpoint event.
	InstructionOffset uint32

	// ModuleOffset is the absolute byte offset of the instruction within the
	// WASM binary module.
	ModuleOffset uint32

	// SourceFile is the Rust source file path resolved via DWARF debug info.
	// Empty when debug symbols are unavailable.
	SourceFile string

	// SourceLine is the 1-based Rust source line number. Zero when unavailable.
	SourceLine uint32

	// SourceColumn is the 1-based Rust source column. Zero when unavailable.
	SourceColumn uint32

	// GitHubURL is the resolved GitHub permalink to the source location.
	// Empty when the repository root or remote cannot be determined.
	GitHubURL string
}

// String returns a human-readable one-line representation of the frame.
func (f WASMFrame) String() string {
	loc := fmt.Sprintf("wasm+0x%x", f.ModuleOffset)
	if f.SourceFile != "" {
		loc = fmt.Sprintf("%s:%d", f.SourceFile, f.SourceLine)
		if f.SourceColumn > 0 {
			loc = fmt.Sprintf("%s:%d", loc, f.SourceColumn)
		}
	}
	return fmt.Sprintf("#%d  %s  (%s)", f.Index, f.FunctionName, loc)
}

// WatchpointEvent is emitted by the simulator each time a watched ledger key
// is mutated during contract execution.
type WatchpointEvent struct {
	// WatchedKey is the ledger key that was matched by the watchpoint.
	WatchedKey string

	// Kind indicates whether the mutation was a write or a delete.
	Kind WatchpointEventKind

	// Value is the new serialised value written to the key. Empty for deletes.
	Value []byte

	// PreviousValue is the serialised value the key held before this mutation.
	// Empty when the key did not previously exist or for deletes where the old
	// value could not be read.
	PreviousValue []byte

	// CallStack is the ordered list of WASM frames captured at the moment the
	// mutation occurred. Index 0 is the innermost frame.
	CallStack []WASMFrame

	// ContractID is the Stellar contract ID (hex-encoded) of the contract that
	// initiated the mutation.
	ContractID string

	// FunctionName is the top-level Soroban host function that was invoked.
	FunctionName string

	// WASMInstruction is a human-readable representation of the specific WASM
	// instruction that performed the store or delete (e.g. "i64.store", "call $remove_entry").
	WASMInstruction string

	// Timestamp is the wall-clock time at which the event was captured. Useful
	// for correlating events across multiple simulation runs.
	Timestamp time.Time
}

// FormatCallStack returns a multi-line, numbered representation of the call
// stack suitable for terminal output.
func (e *WatchpointEvent) FormatCallStack() string {
	if len(e.CallStack) == 0 {
		return "  (no call stack available)"
	}
	var sb strings.Builder
	for _, frame := range e.CallStack {
		sb.WriteString("  ")
		sb.WriteString(frame.String())
		if frame.GitHubURL != "" {
			sb.WriteString("\n      -> ")
			sb.WriteString(frame.GitHubURL)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// Summary returns a compact, single-line description of the event.
func (e *WatchpointEvent) Summary() string {
	top := "(unknown)"
	if len(e.CallStack) > 0 {
		top = e.CallStack[0].FunctionName
	}
	return fmt.Sprintf("[%s] key=%q via %s @ %s", e.Kind, e.WatchedKey, top, e.WASMInstruction)
}

// Watchpoint holds the configuration for a single key-level watchpoint.
type Watchpoint struct {
	// Key is the ledger key to watch, encoded as a canonical string.
	Key string

	// Trigger controls which mutation kinds (write, delete, or both) fire this watchpoint.
	Trigger WatchpointTrigger

	// Label is an optional human-readable label used in diagnostic output.
	Label string
}

// matches reports whether the watchpoint should fire for the given event kind.
func (w *Watchpoint) matches(kind WatchpointEventKind) bool {
	switch kind {
	case WatchpointWrite:
		return w.Trigger&TriggerOnWrite != 0
	case WatchpointDelete:
		return w.Trigger&TriggerOnDelete != 0
	default:
		return false
	}
}

// WatchpointManager manages the set of active watchpoints and dispatches
// WatchpointEvents to registered handlers during simulation.
//
// All methods on WatchpointManager are safe for concurrent use.
type WatchpointManager struct {
	mu         sync.RWMutex
	watchpoints map[string]*Watchpoint // keyed by ledger key
	handlers   []WatchpointHandler
}

// WatchpointHandler is a callback invoked when a watchpoint fires.
// Implementations must not call back into WatchpointManager.
type WatchpointHandler func(event WatchpointEvent)

// NewWatchpointManager creates and returns an initialised WatchpointManager.
func NewWatchpointManager() *WatchpointManager {
	return &WatchpointManager{
		watchpoints: make(map[string]*Watchpoint),
	}
}

// Add registers a new watchpoint. If a watchpoint for the same key already
// exists it is replaced. Returns an error if key is empty.
func (m *WatchpointManager) Add(wp Watchpoint) error {
	if strings.TrimSpace(wp.Key) == "" {
		return fmt.Errorf("watchpoint key must not be empty")
	}
	if wp.Trigger == 0 {
		return fmt.Errorf("watchpoint for key %q has no trigger conditions set", wp.Key)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.watchpoints[wp.Key] = &wp
	return nil
}

// Remove deletes the watchpoint for the given key. It is a no-op if no
// watchpoint for that key exists.
func (m *WatchpointManager) Remove(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.watchpoints, key)
}

// Clear removes all registered watchpoints.
func (m *WatchpointManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watchpoints = make(map[string]*Watchpoint)
}

// List returns a copy of all currently registered watchpoints.
func (m *WatchpointManager) List() []Watchpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Watchpoint, 0, len(m.watchpoints))
	for _, wp := range m.watchpoints {
		out = append(out, *wp)
	}
	return out
}

// Count returns the number of active watchpoints.
func (m *WatchpointManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.watchpoints)
}

// OnFire registers a handler that is called each time any watchpoint fires.
// Multiple handlers may be registered; they are called in registration order.
func (m *WatchpointManager) OnFire(h WatchpointHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, h)
}

// Notify is called by the simulation runner when a ledger key mutation occurs.
// It checks whether the key is being watched and, if so, dispatches the event
// to all registered handlers.
//
// Notify is safe to call from the simulation hot-path; handler dispatch
// happens synchronously so callers must ensure handlers are fast.
func (m *WatchpointManager) Notify(event WatchpointEvent) {
	m.mu.RLock()
	wp, ok := m.watchpoints[event.WatchedKey]
	if !ok || !wp.matches(event.Kind) {
		m.mu.RUnlock()
		return
	}
	// Capture handlers slice under read lock to avoid race with OnFire.
	handlers := m.handlers
	m.mu.RUnlock()

	// Stamp the event with the label from the matched watchpoint and wall time.
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	for _, h := range handlers {
		h(event)
	}
}

// buildWatchpointEvent is a helper used by the runner to construct a fully
// populated WatchpointEvent from raw simulator internals.
//
// Parameters:
//   - key: the canonical ledger key that was mutated.
//   - kind: WatchpointWrite or WatchpointDelete.
//   - newValue: serialised value after the write; nil for deletes.
//   - oldValue: serialised value before the mutation; nil when unknown.
//   - frames: the WASM call stack captured at mutation time.
//   - contractID: hex-encoded contract ID.
//   - hostFn: the top-level host function name.
//   - instruction: human-readable WASM instruction string.
func buildWatchpointEvent(
	key string,
	kind WatchpointEventKind,
	newValue []byte,
	oldValue []byte,
	frames []WASMFrame,
	contractID string,
	hostFn string,
	instruction string,
) WatchpointEvent {
	return WatchpointEvent{
		WatchedKey:      key,
		Kind:            kind,
		Value:           newValue,
		PreviousValue:   oldValue,
		CallStack:       frames,
		ContractID:      contractID,
		FunctionName:    hostFn,
		WASMInstruction: instruction,
		Timestamp:       time.Now().UTC(),
	}
}