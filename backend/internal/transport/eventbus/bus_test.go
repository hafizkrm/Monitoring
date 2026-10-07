package eventbus

import (
	"sync"
	"testing"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
)

func TestEventBus_SinglePubSub(t *testing.T) {
	bus := NewInMemoryEventBus()
	defer bus.Shutdown()

	sub := bus.Subscribe(contracts.DomainEventMetricsUpdated, 10)

	event := contracts.NewDomainEvent("test", contracts.DomainEventMetricsUpdated, "dev-1", "payload")
	bus.Publish(event)

	select {
	case received := <-sub.Channel:
		if received.EventID != event.EventID {
			t.Errorf("Expected event %s, got %s", event.EventID, received.EventID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for event")
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewInMemoryEventBus()
	defer bus.Shutdown()

	sub1 := bus.Subscribe(contracts.DomainEventLogCreated, 10)
	sub2 := bus.Subscribe(contracts.DomainEventLogCreated, 10)

	event := contracts.NewDomainEvent("test", contracts.DomainEventLogCreated, "dev-1", "payload")
	bus.Publish(event)

	for i, sub := range []*Subscriber{sub1, sub2} {
		select {
		case <-sub.Channel:
			// OK
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Timeout waiting for event on sub %d", i+1)
		}
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := NewInMemoryEventBus()
	defer bus.Shutdown()

	sub := bus.Subscribe(contracts.DomainEventAlertCreated, 10)
	bus.Unsubscribe(contracts.DomainEventAlertCreated, sub)

	// Channel should be closed
	_, ok := <-sub.Channel
	if ok {
		t.Fatal("Expected channel to be closed after Unsubscribe")
	}
}

func TestEventBus_SlowSubscriber(t *testing.T) {
	bus := NewInMemoryEventBus()
	defer bus.Shutdown()

	// Buffer size 1 to simulate a slow subscriber
	slowSub := bus.Subscribe(contracts.DomainEventDeviceConnected, 1)
	fastSub := bus.Subscribe(contracts.DomainEventDeviceConnected, 10)

	// Fill the slow sub
	bus.Publish(contracts.NewDomainEvent("test", contracts.DomainEventDeviceConnected, "dev-1", "msg1"))
	// This one should be dropped by slowSub but received by fastSub
	bus.Publish(contracts.NewDomainEvent("test", contracts.DomainEventDeviceConnected, "dev-1", "msg2"))

	// Check slowSub (should have msg1, and not block publishing)
	msg1 := <-slowSub.Channel
	if msg1.Payload != "msg1" {
		t.Errorf("Expected msg1 on slowSub, got %v", msg1.Payload)
	}

	// Check fastSub (should have both)
	fmsg1 := <-fastSub.Channel
	fmsg2 := <-fastSub.Channel
	if fmsg1.Payload != "msg1" || fmsg2.Payload != "msg2" {
		t.Errorf("Expected msg1 and msg2 on fastSub")
	}
}

func TestEventBus_Concurrent(t *testing.T) {
	bus := NewInMemoryEventBus()
	defer bus.Shutdown()

	var wg sync.WaitGroup

	// Concurrent subscribers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sub := bus.Subscribe(contracts.DomainEventMetricsUpdated, 100)
			time.Sleep(10 * time.Millisecond)
			bus.Unsubscribe(contracts.DomainEventMetricsUpdated, sub)
		}()
	}

	// Concurrent publishers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(contracts.NewDomainEvent("test", contracts.DomainEventMetricsUpdated, "dev-1", "payload"))
		}()
	}

	wg.Wait()
}

func TestEventBus_ShutdownIdempotent(t *testing.T) {
	bus := NewInMemoryEventBus()
	sub := bus.Subscribe(contracts.DomainEventMetricsUpdated, 10)

	bus.Shutdown()
	bus.Shutdown() // Should not panic

	_, ok := <-sub.Channel
	if ok {
		t.Fatal("Expected subscriber channel to be closed on shutdown")
	}

	// Subscribing after shutdown should return nil or not panic
	sub2 := bus.Subscribe(contracts.DomainEventMetricsUpdated, 10)
	if sub2 != nil {
		t.Fatal("Expected nil subscriber after shutdown")
	}

	// Publishing after shutdown should not panic
	bus.Publish(contracts.NewDomainEvent("test", contracts.DomainEventMetricsUpdated, "dev-1", "payload"))
}
