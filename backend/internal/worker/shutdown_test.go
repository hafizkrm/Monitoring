package worker

import (
	"context"
	"testing"
	"time"

	"github.com/yourusername/viscod/internal/config"
	"github.com/yourusername/viscod/internal/contracts"
	"github.com/yourusername/viscod/internal/transport/websocket"
)


func TestGracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping graceful shutdown test in short mode")
	}

	cfg := config.Config{
		Polling: config.PollingConfig{
			DefaultInterval: "1s",
			MaxWorkers:      10,
		},
	}

	hub := websocket.NewHub()
	go hub.Run()

	mockDB := &MockDatabase{
		GetLatestMetricsFunc: func(ctx context.Context) ([]map[string]interface{}, error) {
			return nil, nil
		},
	}

	wm := NewManager(cfg, mockDB, &MockLogger{}, nil)
	wm.SetWSBroadcast(func(e contracts.WSEventEnvelope) {})

	ctx, cancel := context.WithCancel(context.Background())
	go wm.Start(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
}
