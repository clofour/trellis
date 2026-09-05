package server

import (
	"sync"

	"github.com/clofour/trellis/internal/api"
)

// EventBus distributes cluster events to SSE subscribers.
type EventBus struct {
	mu          sync.Mutex
	subscribers map[chan api.ClusterEvent]struct{}
}

func newEventBus() *EventBus {
	return &EventBus{subscribers: make(map[chan api.ClusterEvent]struct{})}
}

func (b *EventBus) subscribe() chan api.ClusterEvent {
	ch := make(chan api.ClusterEvent, 64)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *EventBus) unsubscribe(ch chan api.ClusterEvent) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	b.mu.Unlock()
}

func (b *EventBus) publish(event api.ClusterEvent) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
