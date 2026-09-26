package session

import (
	"sync"
	"testing"
	"time"
)

func TestStateStoreMiddlewareCanCallStoreWithoutDeadlocking(t *testing.T) {
	store := NewStateStore()
	done := make(chan struct{})

	store.Use(func(next Dispatcher) Dispatcher {
		_, _ = store.Get("before")
		return func(action Action) {
			next(action)
			_, _ = store.Get(action.Type)
			close(done)
		}
	})

	store.Dispatch(Action{Type: "value", Payload: "ok"})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("dispatch deadlocked while middleware called the store")
	}
}

func TestStateStoreConcurrentUseAndDispatch(t *testing.T) {
	store := NewStateStore()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			store.Use(func(next Dispatcher) Dispatcher { return next })
		}()
		go func(i int) {
			defer wg.Done()
			store.Dispatch(Action{Type: "value", Payload: i})
		}(i)
	}
	wg.Wait()
}
