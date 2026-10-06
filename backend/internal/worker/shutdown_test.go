package worker

import (
	"context"
	"testing"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

type mockEventPublisher struct {
	onPublish func(e contracts.DomainEvent)
}

func (m *mockEventPublisher) Publish(e contracts.DomainEvent) {
	if m.onPublish != nil {
		m.onPublish(e)
	}
}

func TestGracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping graceful shutdown test in short mode")
	}

	cfg := config.Config{
		Polling: config.PollingConfig{
			DefaultInterval: "100ms", // Fast interval for testing
			MaxWorkers:      20,
		},
	}


	mockDB := &MockDatabase{
		GetAllEnabledDevicesFunc: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{
				{ID: 1, IPAddress: "192.168.1.1"},
				{ID: 2, IPAddress: "192.168.1.2"},
			}, nil
		},
		GetLatestMetricsFunc: func(ctx context.Context) ([]map[string]interface{}, error) {
			return nil, nil
		},
	}

	mockSNMP := &MockSNMP{}
	mockRegistry := &MockRegistry{
		GetFunc: func(name string) (Collector, error) { return mockSNMP, nil },
	}

	wm := NewManager(cfg, mockDB, &MockLogger{}, mockRegistry)
	
	// Simulate active WebSocket broadcast stream
	wsMessages := 0
	wm.SetEventPublisher(&mockEventPublisher{
		onPublish: func(e contracts.DomainEvent) {
			wsMessages++
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	
	managerFinished := make(chan bool)
	go func() {
		wm.Start(ctx)
		managerFinished <- true
	}()

	time.Sleep(500 * time.Millisecond) // Let it poll for 500ms
	
	t.Logf("Initiating Graceful Shutdown. Broadcasted %d WS messages so far.", wsMessages)
	
	cancel() // Interrupt!
	
	select {
	case <-managerFinished:
		t.Log("Manager shutdown cleanly.")
	case <-time.After(2 * time.Second):
		t.Fatal("Manager did not shut down in time! Goroutines leaked or blocked.")
	}
}
