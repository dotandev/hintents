package simulator

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// WASMFrame tests
// ---------------------------------------------------------------------------

func TestWASMFrame_String_WithSource(t *testing.T) {
	t.Parallel()
	frame := WASMFrame{
		Index:        0,
		FunctionName: "token::write_balance",
		ModuleOffset: 0x1abc,
		SourceFile:   "src/token.rs",
		SourceLine:   142,
		SourceColumn: 5,
	}
	got := frame.String()
	if !strings.Contains(got, "token::write_balance") {
		t.Errorf("expected function name in frame string, got: %s", got)
	}
	if !strings.Contains(got, "src/token.rs:142:5") {
		t.Errorf("expected source location in frame string, got: %s", got)
	}
}

func TestWASMFrame_String_WithoutSource(t *testing.T) {
	t.Parallel()
	frame := WASMFrame{
		Index:        1,
		FunctionName: "invoke_contract_fn[0]",
		ModuleOffset: 0x0800,
	}
	got := frame.String()
	if !strings.Contains(got, "wasm+0x800") {
		t.Errorf("expected wasm offset in frame string, got: %s", got)
	}
}

// ---------------------------------------------------------------------------
// WatchpointEvent tests
// ---------------------------------------------------------------------------

func TestWatchpointEvent_FormatCallStack_Empty(t *testing.T) {
	t.Parallel()
	evt := WatchpointEvent{}
	got := evt.FormatCallStack()
	if !strings.Contains(got, "no call stack available") {
		t.Errorf("expected empty call stack message, got: %s", got)
	}
}

func TestWatchpointEvent_FormatCallStack_WithFrames(t *testing.T) {
	t.Parallel()
	evt := WatchpointEvent{
		CallStack: []WASMFrame{
			{Index: 0, FunctionName: "token::write_balance", SourceFile: "src/token.rs", SourceLine: 142, ModuleOffset: 0x1abc},
			{Index: 1, FunctionName: "token::transfer", SourceFile: "src/token.rs", SourceLine: 98, ModuleOffset: 0x1a00},
		},
	}
	got := evt.FormatCallStack()
	if !strings.Contains(got, "token::write_balance") {
		t.Errorf("expected first frame in call stack output")
	}
	if !strings.Contains(got, "token::transfer") {
		t.Errorf("expected second frame in call stack output")
	}
}

func TestWatchpointEvent_FormatCallStack_IncludesGitHubURL(t *testing.T) {
	t.Parallel()
	url := "https://github.com/stellar/soroban-examples/blob/main/token/src/token.rs#L142"
	evt := WatchpointEvent{
		CallStack: []WASMFrame{
			{Index: 0, FunctionName: "token::write_balance", GitHubURL: url, SourceFile: "src/token.rs", SourceLine: 142},
		},
	}
	got := evt.FormatCallStack()
	if !strings.Contains(got, url) {
		t.Errorf("expected GitHub URL in call stack output, got: %s", got)
	}
}

func TestWatchpointEvent_Summary(t *testing.T) {
	t.Parallel()
	evt := WatchpointEvent{
		WatchedKey:      "ContractData/balance",
		Kind:            WatchpointWrite,
		WASMInstruction: "i64.store",
		CallStack: []WASMFrame{
			{Index: 0, FunctionName: "token::write_balance"},
		},
	}
	got := evt.Summary()
	if !strings.Contains(got, "WRITE") {
		t.Errorf("expected kind in summary, got: %s", got)
	}
	if !strings.Contains(got, "ContractData/balance") {
		t.Errorf("expected key in summary, got: %s", got)
	}
	if !strings.Contains(got, "token::write_balance") {
		t.Errorf("expected function name in summary, got: %s", got)
	}
}

// ---------------------------------------------------------------------------
// Watchpoint.matches tests
// ---------------------------------------------------------------------------

func TestWatchpoint_Matches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		trigger WatchpointTrigger
		kind    WatchpointEventKind
		want    bool
	}{
		{"write trigger, write event", TriggerOnWrite, WatchpointWrite, true},
		{"write trigger, delete event", TriggerOnWrite, WatchpointDelete, false},
		{"delete trigger, delete event", TriggerOnDelete, WatchpointDelete, true},
		{"delete trigger, write event", TriggerOnDelete, WatchpointWrite, false},
		{"any trigger, write event", TriggerOnAny, WatchpointWrite, true},
		{"any trigger, delete event", TriggerOnAny, WatchpointDelete, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wp := &Watchpoint{Key: "k", Trigger: tc.trigger}
			if got := wp.matches(tc.kind); got != tc.want {
				t.Errorf("matches(%v) = %v, want %v", tc.kind, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WatchpointManager tests
// ---------------------------------------------------------------------------

func TestWatchpointManager_AddAndList(t *testing.T) {
	t.Parallel()
	m := NewWatchpointManager()

	if err := m.Add(Watchpoint{Key: "key1", Trigger: TriggerOnWrite}); err != nil {
		t.Fatalf("Add returned unexpected error: %v", err)
	}
	if err := m.Add(Watchpoint{Key: "key2", Trigger: TriggerOnDelete}); err != nil {
		t.Fatalf("Add returned unexpected error: %v", err)
	}

	list := m.List()
	if len(list) != 2 {
		t.Errorf("expected 2 watchpoints, got %d", len(list))
	}
	if m.Count() != 2 {
		t.Errorf("expected Count() == 2, got %d", m.Count())
	}
}

func TestWatchpointManager_Add_EmptyKeyReturnsError(t *testing.T) {
	t.Parallel()
	m := NewWatchpointManager()
	err := m.Add(Watchpoint{Key: "", Trigger: TriggerOnWrite})
	if err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
}

func TestWatchpointManager_Add_ZeroTriggerReturnsError(t *testing.T) {
	t.Parallel()
	m := NewWatchpointManager()
	err := m.Add(Watchpoint{Key: "key1", Trigger: 0})
	if err == nil {
		t.Fatal("expected error for zero trigger, got nil")
	}
}

func TestWatchpointManager_Add_ReplacesExistingKey(t *testing.T) {
	t.Parallel()
	m := NewWatchpointManager()
	_ = m.Add(Watchpoint{Key: "k", Trigger: TriggerOnWrite, Label: "first"})
	_ = m.Add(Watchpoint{Key: "k", Trigger: TriggerOnDelete, Label: "second"})

	if m.Count() != 1 {
		t.Errorf("expected 1 watchpoint after replacement, got %d", m.Count())
	}
	list := m.List()
	if list[0].Label != "second" {
		t.Errorf("expected replaced watchpoint to have label 'second', got %q", list[0].Label)
	}
}

func TestWatchpointManager_Remove(t *testing.T) {
	t.Parallel()
	m := NewWatchpointManager()
	_ = m.Add(Watchpoint{Key: "key1", Trigger: TriggerOnWrite})
	m.Remove("key1")
	if m.Count() != 0 {
		t.Errorf("expected 0 watchpoints after Remove, got %d", m.Count())
	}
}

func TestWatchpointManager_Remove_NonExistentIsNoOp(t *testing.T) {
	t.Parallel()
	m := NewWatchpointManager()
	// Should not panic.
	m.Remove("does-not-exist")
}

func TestWatchpointManager_Clear(t *testing.T) {
	t.Parallel()
	m := NewWatchpointManager()
	_ = m.Add(Watchpoint{Key: "a", Trigger: TriggerOnWrite})
	_ = m.Add(Watchpoint{Key: "b", Trigger: TriggerOnDelete})
	m.Clear()
	if m.Count() != 0 {
		t.Errorf("expected 0 watchpoints after Clear, got %d", m.Count())
	}
}

func TestWatchpointManager_Notify_FiresOnMatchingKey(t *testing.T) {
	t.Parallel()
	m := NewWatchpointManager()
	_ = m.Add(Watchpoint{Key: "watched-key", Trigger: TriggerOnWrite})

	var fired []WatchpointEvent
	m.OnFire(func(evt WatchpointEvent) { fired = append(fired, evt) })

	m.Notify(WatchpointEvent{WatchedKey: "watched-key", Kind: WatchpointWrite})
	if len(fired) != 1 {
		t.Errorf("expected 1 event fired, got %d", len(fired))
	}
}

func TestWatchpointManager_Notify_SkipsUnwatchedKey(t *testing.T) {
	t.Parallel()
	m := NewWatchpointManager()
	_ = m.Add(Watchpoint{Key: "watched-key", Trigger: TriggerOnWrite})

	var fired []WatchpointEvent
	m.OnFire(func(evt WatchpointEvent) { fired = append(fired, evt) })

	m.Notify(WatchpointEvent{WatchedKey: "other-key", Kind: WatchpointWrite})
	if len(fired) != 0 {
		t.Errorf("expected 0 events for unwatched key, got %d", len(fired))
	}
}

func TestWatchpointManager_Notify_SkipsNonMatchingKind(t *testing.T) {
	t.Parallel()
	m := NewWatchpointManager()
	_ = m.Add(Watchpoint{Key: "k", Trigger: TriggerOnWrite}) // write-only watchpoint

	var fired []WatchpointEvent
	m.OnFire(func(evt WatchpointEvent) { fired = append(fired, evt) })

	m.Notify(WatchpointEvent{WatchedKey: "k", Kind: WatchpointDelete})
	if len(fired) != 0 {
		t.Errorf("expected 0 events when kind does not match trigger, got %d", len(fired))
	}
}

func TestWatchpointManager_Notify_StampsTimestamp(t *testing.T) {
	t.Parallel()
	m := NewWatchpointManager()
	_ = m.Add(Watchpoint{Key: "k", Trigger: TriggerOnAny})

	var fired []WatchpointEvent
	m.OnFire(func(evt WatchpointEvent) { fired = append(fired, evt) })

	before := time.Now().UTC()
	m.Notify(WatchpointEvent{WatchedKey: "k", Kind: WatchpointWrite})
	after := time.Now().UTC()

	if len(fired) != 1 {
		t.Fatalf("expected 1 event, got %d", len(fired))
	}
	ts := fired[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Errorf("timestamp %v outside expected range [%v, %v]", ts, before, after)
	}
}

func TestWatchpointManager_Notify_CallsMultipleHandlers(t *testing.T) {
	t.Parallel()
	m := NewWatchpointManager()
	_ = m.Add(Watchpoint{Key: "k", Trigger: TriggerOnAny})

	count := 0
	m.OnFire(func(_ WatchpointEvent) { count++ })
	m.OnFire(func(_ WatchpointEvent) { count++ })

	m.Notify(WatchpointEvent{WatchedKey: "k", Kind: WatchpointWrite})
	if count != 2 {
		t.Errorf("expected both handlers to be called, count = %d", count)
	}
}

func TestWatchpointManager_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	m := NewWatchpointManager()
	_ = m.Add(Watchpoint{Key: "shared-key", Trigger: TriggerOnAny})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Notify(WatchpointEvent{WatchedKey: "shared-key", Kind: WatchpointWrite})
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Runner tests
// ---------------------------------------------------------------------------

func TestNewRunner_InvalidConfig_EmptyEnvelope(t *testing.T) {
	t.Parallel()
	_, err := NewRunner(RunnerConfig{
		Network: "testnet",
	})
	if err == nil {
		t.Fatal("expected error for empty TransactionEnvelope, got nil")
	}
}

func TestNewRunner_InvalidConfig_EmptyNetwork(t *testing.T) {
	t.Parallel()
	_, err := NewRunner(RunnerConfig{
		TransactionEnvelope: []byte("fake-xdr"),
	})
	if err == nil {
		t.Fatal("expected error for empty Network, got nil")
	}
}

func TestRunner_Run_Success(t *testing.T) {
	t.Parallel()
	r, err := NewRunner(RunnerConfig{
		TransactionEnvelope: []byte("fake-xdr"),
		Network:             "testnet",
		ContractID:          "CAFEBABE",
		LedgerFootprint:     make(map[LedgerKey]LedgerValue),
	})
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	result, err := r.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected result.Success == true")
	}
}

func TestRunner_Run_WatchpointWriteFires(t *testing.T) {
	t.Parallel()
	wpm := NewWatchpointManager()
	_ = wpm.Add(Watchpoint{
		Key:     "ContractData/AAAAAA==/balance",
		Trigger: TriggerOnWrite,
		Label:   "balance watchpoint",
	})

	r, err := NewRunner(RunnerConfig{
		TransactionEnvelope: []byte("fake-xdr"),
		Network:             "testnet",
		ContractID:          "CAFEBABE",
		LedgerFootprint:     make(map[LedgerKey]LedgerValue),
		Watchpoints:         wpm,
	})
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}

	result, err := r.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// The stub executeTransaction emits a write to "ContractData/AAAAAA==/balance".
	found := false
	for _, evt := range result.WatchpointEvents {
		if evt.WatchedKey == "ContractData/AAAAAA==/balance" && evt.Kind == WatchpointWrite {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected write watchpoint event for balance key, got events: %+v", result.WatchpointEvents)
	}
}

func TestRunner_Run_WatchpointDeleteFires(t *testing.T) {
	t.Parallel()
	wpm := NewWatchpointManager()
	_ = wpm.Add(Watchpoint{
		Key:     "ContractData/AAAAAA==/allowance",
		Trigger: TriggerOnDelete,
	})

	r, err := NewRunner(RunnerConfig{
		TransactionEnvelope: []byte("fake-xdr"),
		Network:             "testnet",
		ContractID:          "CAFEBABE",
		LedgerFootprint:     make(map[LedgerKey]LedgerValue),
		Watchpoints:         wpm,
	})
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}

	result, err := r.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	found := false
	for _, evt := range result.WatchpointEvents {
		if evt.WatchedKey == "ContractData/AAAAAA==/allowance" && evt.Kind == WatchpointDelete {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected delete watchpoint event for allowance key, got events: %+v", result.WatchpointEvents)
	}
}

func TestRunner_Run_WatchpointEventHasCallStack(t *testing.T) {
	t.Parallel()
	wpm := NewWatchpointManager()
	_ = wpm.Add(Watchpoint{Key: "ContractData/AAAAAA==/balance", Trigger: TriggerOnAny})

	r, _ := NewRunner(RunnerConfig{
		TransactionEnvelope: []byte("fake-xdr"),
		Network:             "testnet",
		ContractID:          "CAFEBABE",
		LedgerFootprint:     make(map[LedgerKey]LedgerValue),
		Watchpoints:         wpm,
	})
	result, _ := r.Run()

	for _, evt := range result.WatchpointEvents {
		if evt.WatchedKey == "ContractData/AAAAAA==/balance" {
			if len(evt.CallStack) == 0 {
				t.Error("expected non-empty call stack in watchpoint event")
			}
			return
		}
	}
	t.Error("balance watchpoint event not found")
}

func TestRunner_Run_WatchpointEventHasWASMInstruction(t *testing.T) {
	t.Parallel()
	wpm := NewWatchpointManager()
	_ = wpm.Add(Watchpoint{Key: "ContractData/AAAAAA==/balance", Trigger: TriggerOnWrite})

	r, _ := NewRunner(RunnerConfig{
		TransactionEnvelope: []byte("fake-xdr"),
		Network:             "testnet",
		ContractID:          "CAFEBABE",
		LedgerFootprint:     make(map[LedgerKey]LedgerValue),
		Watchpoints:         wpm,
	})
	result, _ := r.Run()

	for _, evt := range result.WatchpointEvents {
		if evt.WatchedKey == "ContractData/AAAAAA==/balance" {
			if evt.WASMInstruction == "" {
				t.Error("expected WASMInstruction to be populated in watchpoint event")
			}
			return
		}
	}
	t.Error("balance watchpoint event not found")
}

func TestRunner_Run_NoWatchpointManager_NoEvents(t *testing.T) {
	t.Parallel()
	r, _ := NewRunner(RunnerConfig{
		TransactionEnvelope: []byte("fake-xdr"),
		Network:             "testnet",
		ContractID:          "CAFEBABE",
		LedgerFootprint:     make(map[LedgerKey]LedgerValue),
		Watchpoints:         nil, // disabled
	})
	result, _ := r.Run()
	if len(result.WatchpointEvents) != 0 {
		t.Errorf("expected no watchpoint events when manager is nil, got %d", len(result.WatchpointEvents))
	}
}

func TestRunner_Run_FootprintUpdatedOnWrite(t *testing.T) {
	t.Parallel()
	footprint := make(map[LedgerKey]LedgerValue)
	r, _ := NewRunner(RunnerConfig{
		TransactionEnvelope: []byte("fake-xdr"),
		Network:             "testnet",
		ContractID:          "CAFEBABE",
		LedgerFootprint:     footprint,
	})
	_, _ = r.Run()

	// The stub writes to "ContractData/AAAAAA==/balance".
	if _, ok := footprint["ContractData/AAAAAA==/balance"]; !ok {
		t.Error("expected footprint to contain written key after simulation")
	}
}

func TestRunner_Run_FootprintDeletedOnDelete(t *testing.T) {
	t.Parallel()
	footprint := map[LedgerKey]LedgerValue{
		"ContractData/AAAAAA==/allowance": []byte(`{"spender":"GBXYZ","amount":500}`),
	}
	r, _ := NewRunner(RunnerConfig{
		TransactionEnvelope: []byte("fake-xdr"),
		Network:             "testnet",
		ContractID:          "CAFEBABE",
		LedgerFootprint:     footprint,
	})
	_, _ = r.Run()

	// The stub deletes "ContractData/AAAAAA==/allowance".
	if _, ok := footprint["ContractData/AAAAAA==/allowance"]; ok {
		t.Error("expected deleted key to be absent from footprint after simulation")
	}
}

// ---------------------------------------------------------------------------
// FormatWatchpointReport tests
// ---------------------------------------------------------------------------

func TestFormatWatchpointReport_Empty(t *testing.T) {
	t.Parallel()
	result := SimulationResult{}
	got := FormatWatchpointReport(result)
	if got != "" {
		t.Errorf("expected empty report for no events, got: %s", got)
	}
}

func TestFormatWatchpointReport_WithEvents(t *testing.T) {
	t.Parallel()
	result := SimulationResult{
		WatchpointEvents: []WatchpointEvent{
			{
				WatchedKey:      "ContractData/balance",
				Kind:            WatchpointWrite,
				ContractID:      "CAFEBABE",
				FunctionName:    "call",
				WASMInstruction: "i64.store",
				Timestamp:       time.Now().UTC(),
				CallStack: []WASMFrame{
					{Index: 0, FunctionName: "token::write_balance", SourceFile: "src/token.rs", SourceLine: 142},
				},
			},
		},
	}
	report := FormatWatchpointReport(result)
	for _, substr := range []string{
		"Watchpoint Report",
		"ContractData/balance",
		"WRITE",
		"CAFEBABE",
		"i64.store",
		"token::write_balance",
	} {
		if !strings.Contains(report, substr) {
			t.Errorf("expected %q in report, got:\n%s", substr, report)
		}
	}
}

// ---------------------------------------------------------------------------
// buildWatchpointEvent tests
// ---------------------------------------------------------------------------

func TestBuildWatchpointEvent_Fields(t *testing.T) {
	t.Parallel()
	frames := []WASMFrame{{Index: 0, FunctionName: "fn"}}
	evt := buildWatchpointEvent(
		"my-key",
		WatchpointWrite,
		[]byte("new"),
		[]byte("old"),
		frames,
		"CONTRACT01",
		"call",
		"i32.store",
	)
	if evt.WatchedKey != "my-key" {
		t.Errorf("WatchedKey = %q, want %q", evt.WatchedKey, "my-key")
	}
	if evt.Kind != WatchpointWrite {
		t.Errorf("Kind = %q, want %q", evt.Kind, WatchpointWrite)
	}
	if string(evt.Value) != "new" {
		t.Errorf("Value = %q, want %q", evt.Value, "new")
	}
	if string(evt.PreviousValue) != "old" {
		t.Errorf("PreviousValue = %q, want %q", evt.PreviousValue, "old")
	}
	if evt.ContractID != "CONTRACT01" {
		t.Errorf("ContractID = %q, want %q", evt.ContractID, "CONTRACT01")
	}
	if evt.WASMInstruction != "i32.store" {
		t.Errorf("WASMInstruction = %q, want %q", evt.WASMInstruction, "i32.store")
	}
	if evt.Timestamp.IsZero() {
		t.Error("expected Timestamp to be populated")
	}
}