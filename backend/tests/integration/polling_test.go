package integration

import (
	"context"
	"testing"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/hafizkrm/Monitoring/backend/internal/database"
	"github.com/hafizkrm/Monitoring/backend/internal/logger"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
	"github.com/hafizkrm/Monitoring/backend/internal/worker"
)

// MockRuleCache for integration test
type MockRuleCache struct{}

func (m *MockRuleCache) GetRulesForDevice(deviceID int) map[string]models.ThresholdRule {
	return nil
}

func (m *MockRuleCache) Refresh(ctx context.Context) error {
	return nil
}

// MockCollector for integration test
type MockCollector struct{}

func (m *MockCollector) Collect(ctx context.Context, device models.Device) (*models.TelemetrySnapshot, error) {
	return &models.TelemetrySnapshot{
		DeviceID:    device.ID,
		Status:      "up",
		CPUUsage:    12.5,
		MemoryUsage: 45.0,
		MemoryTotal: 16 * 1024 * 1024 * 1024,
		MemoryUsed:  8 * 1024 * 1024 * 1024,
		Uptime:      3600,
		CollectedAt: time.Now(),
		Interfaces: []models.InterfaceSnapshot{
			{
				DeviceID:       device.ID,
				InterfaceIndex: 1,
				InterfaceName:  "eth0",
				Status:         "up",
				InOctets:       1000,
				OutOctets:      2000,
				Speed:          1000000000,
				CollectedAt:    time.Now(),
			},
		},
	}, nil
}

func (m *MockCollector) GetNetworkStats(ctx context.Context, ip string) (int, float64, float64, error) {
	return 10, 1.0, 0, nil
}

// MockRegistry for integration test
type MockRegistry struct {
	collector worker.Collector
}

func (m *MockRegistry) Register(name string, collector worker.Collector) {
}

func (m *MockRegistry) Get(name string) (worker.Collector, error) {
	return m.collector, nil
}


func TestFullPollingFlow(t *testing.T) {
	// Skip if no real database is available or configured
	// For this task, we assume the environment might not have a ready MySQL
	// but we'll write the test code anyway.
	t.Skip("Skipping integration test that requires real MySQL connection")

	cfg := config.Config{
		Database: config.DatabaseConfig{
			Host:              "localhost",
			Port:              3306,
			User:              "root",
			Password:          "",
			Database:          "monitoring_test",
			ConnectionTimeout: "5s",
		},
		Polling: config.PollingConfig{
			MaxWorkers:          2,
			DefaultInterval:     "1s",
			HealthCheckInterval: "500ms",
		},
	}

	log := logger.InitLogger(config.LoggerConfig{Level: "DEBUG", OutputPath: "./var/log/test.log"})

	db, err := database.NewDatabase(cfg.Database, log)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer db.Close()

	// 1. Setup: Clean and add a test device
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, "DELETE FROM interface_metrics")
	_, _ = db.ExecContext(ctx, "DELETE FROM device_metrics")
	_, _ = db.ExecContext(ctx, "DELETE FROM devices")

	_, err = db.ExecContext(ctx,
		"INSERT INTO devices (id, name, ip_address, enabled, polling_interval) VALUES (?, ?, ?, ?, ?)",
		1, "TestRouter", "127.0.0.1", 1, 1)
	if err != nil {
		t.Fatalf("Failed to insert test device: %v", err)
	}

	// 2. Start Manager
	mockRegistry := &MockRegistry{collector: &MockCollector{}}
	mockRuleCache := &MockRuleCache{}
	mgr := worker.NewManager(cfg, db, log, mockRegistry, mockRuleCache)

	runCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := mgr.Start(runCtx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// 3. Wait for polling to happen
	time.Sleep(2 * time.Second)
	mgr.Stop()

	// 4. Verify: Check if data was saved
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM device_metrics WHERE device_id = 1").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query metrics count: %v", err)
	}

	if count == 0 {
		t.Error("Expected at least one metric record, got 0")
	}

	var ifCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM interface_metrics WHERE device_id = 1").Scan(&ifCount)
	if err != nil {
		t.Fatalf("Failed to query interface metrics count: %v", err)
	}

	if ifCount == 0 {
		t.Error("Expected at least one interface metric record, got 0")
	}
}
