// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package eventbus

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestSubscribeAndEmit(t *testing.T) {
	bus := New()

	var received []any
	bus.Subscribe("test.topic", func(payload any) {
		received = append(received, payload)
	})

	bus.Emit("test.topic", "hello")
	bus.Emit("test.topic", 42)

	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
	if received[0] != "hello" {
		t.Errorf("expected 'hello', got %v", received[0])
	}
	if received[1] != 42 {
		t.Errorf("expected 42, got %v", received[1])
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := New()

	var count int
	id := bus.Subscribe("topic", func(any) { count++ })

	bus.Emit("topic", nil)
	if count != 1 {
		t.Fatalf("expected 1 call before unsubscribe, got %d", count)
	}

	bus.Unsubscribe("topic", id)
	bus.Emit("topic", nil)

	if count != 1 {
		t.Errorf("handler called after Unsubscribe: got %d calls total", count)
	}
}

func TestUnsubscribeUnknownID(t *testing.T) {
	bus := New()
	// Should not panic
	bus.Unsubscribe("no-such-topic", HandlerID(999))
}

func TestEmitNoSubscribers(t *testing.T) {
	bus := New()
	// Should not panic
	bus.Emit("ghost.topic", "payload")
}

func TestMultipleSubscribers(t *testing.T) {
	bus := New()

	var mu sync.Mutex
	var results []string

	bus.Subscribe("multi", func(p any) {
		mu.Lock()
		results = append(results, "A:"+p.(string))
		mu.Unlock()
	})
	bus.Subscribe("multi", func(p any) {
		mu.Lock()
		results = append(results, "B:"+p.(string))
		mu.Unlock()
	})

	bus.Emit("multi", "x")

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d: %v", len(results), results)
	}
}

func TestTopicIsolation(t *testing.T) {
	bus := New()

	var aCount, bCount int
	bus.Subscribe("topic.a", func(any) { aCount++ })
	bus.Subscribe("topic.b", func(any) { bCount++ })

	bus.Emit("topic.a", nil)
	bus.Emit("topic.a", nil)
	bus.Emit("topic.b", nil)

	if aCount != 2 {
		t.Errorf("topic.a: expected 2, got %d", aCount)
	}
	if bCount != 1 {
		t.Errorf("topic.b: expected 1, got %d", bCount)
	}
}

func TestTopicsAndSubscriberCount(t *testing.T) {
	bus := New()

	if len(bus.Topics()) != 0 {
		t.Error("expected no topics initially")
	}

	bus.Subscribe("alpha", func(any) {})
	bus.Subscribe("alpha", func(any) {})
	bus.Subscribe("beta", func(any) {})

	if bus.SubscriberCount("alpha") != 2 {
		t.Errorf("expected 2 subscribers on alpha, got %d", bus.SubscriberCount("alpha"))
	}
	if bus.SubscriberCount("beta") != 1 {
		t.Errorf("expected 1 subscriber on beta, got %d", bus.SubscriberCount("beta"))
	}
	if bus.SubscriberCount("ghost") != 0 {
		t.Error("expected 0 subscribers on unknown topic")
	}
}

func TestTopicsCleanedUpAfterUnsubscribe(t *testing.T) {
	bus := New()

	id := bus.Subscribe("temp", func(any) {})
	bus.Unsubscribe("temp", id)

	topics := bus.Topics()
	for _, topic := range topics {
		if topic == "temp" {
			t.Error("expected 'temp' topic to be removed after last unsubscribe")
		}
	}
}

// TestConcurrentEmitAndUnsubscribe is the core regression test for the issue:
// concurrent map write panic when Unsubscribe is called while Emit is running.
// Run with: go test -race ./internal/eventbus/
func TestConcurrentEmitAndUnsubscribe(t *testing.T) {
	bus := New()

	const goroutines = 50
	const iterations = 200

	var wg sync.WaitGroup

	// Continuously emit events
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < goroutines*iterations; i++ {
			bus.Emit("concurrent.topic", i)
		}
	}()

	// Concurrently subscribe and unsubscribe
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				id := bus.Subscribe("concurrent.topic", func(any) {})
				bus.Unsubscribe("concurrent.topic", id)
			}
		}()
	}

	wg.Wait()
}

// TestConcurrentMultipleEmitters tests multiple goroutines emitting simultaneously.
func TestConcurrentMultipleEmitters(t *testing.T) {
	bus := New()

	var counter atomic.Int64
	bus.Subscribe("count", func(any) {
		counter.Add(1)
	})

	const emitters = 20
	const emitsEach = 100

	var wg sync.WaitGroup
	for i := 0; i < emitters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < emitsEach; j++ {
				bus.Emit("count", nil)
			}
		}()
	}
	wg.Wait()

	if got := counter.Load(); got != emitters*emitsEach {
		t.Errorf("expected %d total handler calls, got %d", emitters*emitsEach, got)
	}
}

// TestUnsubscribeDuringEmit verifies that unsubscribing from inside a handler
// does not deadlock (because Emit releases the lock before invoking handlers).
func TestUnsubscribeDuringEmit(t *testing.T) {
	bus := New()

	var id HandlerID
	var callCount int

	id = bus.Subscribe("self-remove", func(any) {
		callCount++
		bus.Unsubscribe("self-remove", id)
	})

	bus.Emit("self-remove", nil)
	bus.Emit("self-remove", nil) // handler should not be called again

	if callCount != 1 {
		t.Errorf("expected handler called exactly once, got %d", callCount)
	}
}