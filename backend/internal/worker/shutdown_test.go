package worker

import (
	"context"
	"testing"
	"time"

	"github.com/yourusername/viscod/internal/config"
	"github.com/yourusername/viscod/internal/contracts"
	"github.com/yourusername/viscod/internal/models"
	"github.com/yourusername/viscod/internal/transport/websocket"
)

type MockDB struct {}
func (m *MockDB) SaveMetrics(ctx context.Context, data map[string]interface{}) error { return nil }
func (m *MockDB) GetLatestMetrics(ctx context.Context) ([]map[string]interface{}, error) { return nil, nil }
func (m *MockDB) LogIncident(ctx context.Context, deviceID int, severity string, description string) error { return nil }
func (m *MockDB) ResolveIncident(ctx context.Context, deviceID int) error { return nil }
func (m *MockDB) BatchInsertInterfaceMetrics(ctx context.Context, metrics []*models.InterfaceMetric) error { return nil }
func (m *MockDB) CleanupOldData(ctx context.Context, retentionDays int, limit int) error { return nil }

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

	wm := NewManager(cfg, &MockDB{}, nil, nil)
	wm.SetWSBroadcast(func(e contracts.WSEventEnvelope) {})

	ctx, cancel := context.WithCancel(context.Background())
	go wm.Start(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
}
