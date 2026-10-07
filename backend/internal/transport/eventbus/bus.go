package eventbus

import (
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
)

// EventBus defines the interface for our internal event system.
type EventBus interface {
	Subscribe(topic contracts.EventType, bufferSize int) *Subscriber
	Unsubscribe(topic contracts.EventType, sub *Subscriber)
	Publish(event contracts.DomainEvent)
	Shutdown()
}

// Subscriber represents a single subscriber to the bus.
type Subscriber struct {
	ID      string
	Channel chan contracts.DomainEvent
	dropped uint64
}

// DroppedCount safely returns the number of messages dropped due to backpressure.
func (s *Subscriber) DroppedCount() uint64 {
	return atomic.LoadUint64(&s.dropped)
}

type inMemoryEventBus struct {
	mu          sync.RWMutex
	subscribers map[contracts.EventType]map[*Subscriber]struct{}
	isShutdown  bool
}

// NewInMemoryEventBus creates a new thread-safe EventBus.
func NewInMemoryEventBus() EventBus {
	return &inMemoryEventBus{
		subscribers: make(map[contracts.EventType]map[*Subscriber]struct{}),
	}
}

// Subscribe adds a subscriber for a specific event type with bounded buffering.
func (b *inMemoryEventBus) Subscribe(topic contracts.EventType, bufferSize int) *Subscriber {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.isShutdown {
		return nil
	}

	sub := &Subscriber{
		ID:      uuid.NewString(),
		Channel: make(chan contracts.DomainEvent, bufferSize),
	}

	if _, ok := b.subscribers[topic]; !ok {
		b.subscribers[topic] = make(map[*Subscriber]struct{})
	}
	b.subscribers[topic][sub] = struct{}{}

	return sub
}

// Unsubscribe removes a subscriber safely and closes its channel.
func (b *inMemoryEventBus) Unsubscribe(topic contracts.EventType, sub *Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.isShutdown {
		return
	}

	if subs, ok := b.subscribers[topic]; ok {
		if _, exists := subs[sub]; exists {
			delete(subs, sub)
			close(sub.Channel)
		}
	}
}

// Publish broadcasts an event to all subscribers of the event's topic.
// Applies bounded buffering (backpressure) policy: if a subscriber's channel is full,
// the event is dropped for that specific subscriber to prevent blocking the worker/publisher.
func (b *inMemoryEventBus) Publish(event contracts.DomainEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.isShutdown {
		return
	}

	subs, ok := b.subscribers[event.Type]
	if !ok {
		return
	}

	for sub := range subs {
		select {
		case sub.Channel <- event:
			// Published successfully
		default:
			// Slow subscriber: channel is full. Drop message to avoid blocking.
			atomic.AddUint64(&sub.dropped, 1)
		}
	}
}

// Shutdown safely closes the bus and all subscriber channels idempotently.
func (b *inMemoryEventBus) Shutdown() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.isShutdown {
		return
	}
	b.isShutdown = true

	for _, subs := range b.subscribers {
		for sub := range subs {
			close(sub.Channel)
		}
	}

	// Clear the references
	b.subscribers = make(map[contracts.EventType]map[*Subscriber]struct{})
}
