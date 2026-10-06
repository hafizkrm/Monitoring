package worker

import (
	"context"
	"testing"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
	"github.com/hafizkrm/Monitoring/backend/internal/transport/websocket"
)

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

	hub := websocket.NewHub()
	go hub.Run()

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

	wm := NewManager(cfg, mockDB, &MockLogger{}, mockSNMP)
	
	// Simulate active WebSocket broadcast stream
	wsMessages := 0
	wm.SetWSBroadcast(func(e contracts.WSEventEnvelope) {
		wsMessages++
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
