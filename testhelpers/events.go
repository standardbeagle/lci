package testhelpers

import (
	"context"
	"sync"
	"time"
)

// EventBus provides a simple event notification system for tests.
// Components can emit events when state changes occur, allowing tests
// to synchronize without arbitrary sleeps.
type EventBus struct {
	mu        sync.RWMutex
	listeners map[string][]chan struct{}
}

// NewEventBus creates a new event bus for test synchronization.
func NewEventBus() *EventBus {
	return &EventBus{
		listeners: make(map[string][]chan struct{}),
	}
}

// Subscribe registers a listener for a specific event type.
// Returns a channel that will be closed when the event fires.
func (eb *EventBus) Subscribe(eventType string) <-chan struct{} {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan struct{})
	eb.listeners[eventType] = append(eb.listeners[eventType], ch)
	return ch
}

// Emit fires an event, notifying all subscribers.
func (eb *EventBus) Emit(eventType string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if listeners, ok := eb.listeners[eventType]; ok {
		for _, ch := range listeners {
			close(ch)
		}
		// Clear listeners after notification
		delete(eb.listeners, eventType)
	}
}

// WaitFor waits for an event with a timeout.
// Returns true if event occurred, false if timeout.
func (eb *EventBus) WaitFor(ctx context.Context, eventType string, timeout time.Duration) bool {
	ch := eb.Subscribe(eventType)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-ch:
		return true
	case <-ctx.Done():
		return false
	}
}

// WaitForAll waits for multiple events with a timeout.
// Returns true if all events occurred, false if timeout.
func (eb *EventBus) WaitForAll(ctx context.Context, timeout time.Duration, eventTypes ...string) bool {
	channels := make([]<-chan struct{}, len(eventTypes))
	for i, eventType := range eventTypes {
		channels[i] = eb.Subscribe(eventType)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for _, ch := range channels {
		select {
		case <-ch:
			// Event occurred, continue to next
		case <-ctx.Done():
			return false
		}
	}
	return true
}

// CompletionSignal is a reusable signal for operation completion.
type CompletionSignal struct {
	done chan struct{}
	once sync.Once
}

// NewCompletionSignal creates a new completion signal.
func NewCompletionSignal() *CompletionSignal {
	return &CompletionSignal{
		done: make(chan struct{}),
	}
}

// Complete signals that the operation is complete.
// Safe to call multiple times (only first call takes effect).
func (cs *CompletionSignal) Complete() {
	cs.once.Do(func() {
		close(cs.done)
	})
}

// Wait waits for completion or timeout.
// Returns true if completed, false if timeout.
func (cs *CompletionSignal) Wait(timeout time.Duration) bool {
	select {
	case <-cs.done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// WaitCtx waits for completion or context cancellation.
// Returns true if completed, false if context cancelled.
func (cs *CompletionSignal) WaitCtx(ctx context.Context) bool {
	select {
	case <-cs.done:
		return true
	case <-ctx.Done():
		return false
	}
}

// Done returns the completion channel for use in select statements.
func (cs *CompletionSignal) Done() <-chan struct{} {
	return cs.done
}

// CountdownLatch allows waiting for N operations to complete.
type CountdownLatch struct {
	mu    sync.Mutex
	count int
	done  chan struct{}
}

// NewCountdownLatch creates a latch that waits for count operations.
func NewCountdownLatch(count int) *CountdownLatch {
	return &CountdownLatch{
		count: count,
		done:  make(chan struct{}),
	}
}

// CountDown decrements the latch count.
// When count reaches zero, all waiters are released.
func (cl *CountdownLatch) CountDown() {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if cl.count > 0 {
		cl.count--
		if cl.count == 0 {
			close(cl.done)
		}
	}
}

// Wait waits for the count to reach zero or timeout.
// Returns true if completed, false if timeout.
func (cl *CountdownLatch) Wait(timeout time.Duration) bool {
	select {
	case <-cl.done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// WaitCtx waits for the count to reach zero or context cancellation.
func (cl *CountdownLatch) WaitCtx(ctx context.Context) bool {
	select {
	case <-cl.done:
		return true
	case <-ctx.Done():
		return false
	}
}

// Done returns the completion channel.
func (cl *CountdownLatch) Done() <-chan struct{} {
	return cl.done
}

// GetCount returns the current count (for debugging).
func (cl *CountdownLatch) GetCount() int {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return cl.count
}
