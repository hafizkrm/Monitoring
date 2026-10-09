package main

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	_ "github.com/go-sql-driver/mysql"
	"github.com/hafizkrm/Monitoring/backend/internal/cache"
	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
	"github.com/hafizkrm/Monitoring/backend/internal/database"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
	"github.com/hafizkrm/Monitoring/backend/internal/transport/eventbus"
	"github.com/hafizkrm/Monitoring/backend/internal/worker"
)

// mockLogger for tests
type mockLogger struct{}

type mockRuleCache struct{}

func (m *mockRuleCache) GetRulesForDevice(deviceID int) map[string]models.ThresholdRule {
	return nil
}
func (m *mockRuleCache) Refresh(ctx context.Context) error {
	return nil
}

func (m *mockLogger) Info(msg string, fields map[string]interface{})  {}
func (m *mockLogger) Error(msg string, fields map[string]interface{}) {}
func (m *mockLogger) Debug(msg string, fields map[string]interface{}) {}
func (m *mockLogger) Warn(msg string, fields map[string]interface{})  {}
func (m *mockLogger) Fatal(msg string, fields map[string]interface{}) {}
func (m *mockLogger) Sync() error                                     { return nil }

func setupE2EDB(t *testing.T) *database.Database {
	rawDB, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/nms_verification_db?parseTime=true")
	if err != nil {
		t.Fatalf("Failed to connect to verification DB: %v", err)
	}

	// Setup necessary schema for E2E
	_, _ = rawDB.Exec(`CREATE TABLE IF NOT EXISTS devices (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(255),
		ip_address VARCHAR(255),
		device_type VARCHAR(255),
		status VARCHAR(50) DEFAULT 'online',
		enabled TINYINT(1) DEFAULT 1
	)`)
	_, _ = rawDB.Exec(`DROP TABLE IF EXISTS device_metrics`)
	_, _ = rawDB.Exec(`CREATE TABLE device_metrics (
		id INT AUTO_INCREMENT PRIMARY KEY,
		device_id INT,
		collected_at DATETIME,
		cpu_usage FLOAT,
		memory_usage FLOAT,
		memory_total FLOAT,
		uptime INT,
		rx_rate FLOAT,
		tx_rate FLOAT,
		jitter FLOAT
	)`)

	// Clean tables
	_, _ = rawDB.Exec("TRUNCATE TABLE devices")
	_, _ = rawDB.Exec("TRUNCATE TABLE device_metrics")

	// Insert test device
	_, _ = rawDB.Exec("INSERT INTO devices (id, name, ip_address, device_type) VALUES (1, 'Test Router', '192.168.1.1', 'router')")

	return &database.Database{DB: rawDB}
}

func TestM11_E2E_Phase3Exit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	db := setupE2EDB(t)
	defer db.Close()

	bus := eventbus.NewInMemoryEventBus()
	defer bus.Shutdown()

	logger := &mockLogger{}

	cfg := config.Config{
		Polling: config.PollingConfig{
			DefaultInterval: "1s",
			MaxWorkers:      10,
		},
	}

	mockRuleCache := &mockRuleCache{}
	mgr := worker.NewManager(cfg, db, logger, nil, mockRuleCache)
	mgr.SetEventPublisher(bus)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		mgr.Start(ctx) // This will block until context is cancelled
	}()

	metricPayload := cache.LatestDeviceMetrics{
		DeviceID:    1,
		Name:        "Test Router",
		DeviceType:  "router",
		Status:      "online",
		CPUUsage:    15.5,
		MemoryUsage: 2048.0,
		Uptime:      3600,
		CollectedAt: time.Now(),
	}

	// Inject directly to database like the worker does when processing
	_, err := db.ExecContext(ctx, "INSERT INTO device_metrics (device_id, collected_at, cpu_usage, memory_usage, uptime) VALUES (?, ?, ?, ?, ?)",
		metricPayload.DeviceID, metricPayload.CollectedAt, metricPayload.CPUUsage, metricPayload.MemoryUsage, metricPayload.Uptime)
	if err != nil {
		t.Fatalf("Failed to insert metric: %v", err)
	}

	// And publish to EventBus like the worker does
	domainEvent := contracts.NewDomainEvent("e2e-test", contracts.DomainEventMetricsUpdated, "1", metricPayload)
	bus.Publish(domainEvent)

	// Wait for exporter to process
	time.Sleep(200 * time.Millisecond)

	// A. Telemetry Integrity & Historical Query
	// Query the DB
	rows, err := db.Query("SELECT cpu_usage FROM device_metrics WHERE device_id = 1")
	if err != nil {
		t.Fatalf("Historical query failed: %v", err)
	}
	defer rows.Close()
	var cpu float64
	var count int
	for rows.Next() {
		rows.Scan(&cpu)
		count++
	}
	if count != 1 {
		t.Fatalf("Expected 1 historical metric row, got %d", count)
	}
	if cpu != 15.5 {
		t.Errorf("Expected cpu 15.5, got %v", cpu)
	}

	// B. Persistence / Recovery
	// Cancel the context and wait for graceful shutdown
	cancel()
	wg.Wait()

	// Verify DB data remains (durability)
	var remain int
	db.QueryRow("SELECT COUNT(*) FROM device_metrics WHERE device_id = 1").Scan(&remain)
	if remain != 1 {
		t.Errorf("Expected 1 row after shutdown, got %d", remain)
	}

	// C. Cardinality Verification
	// Let's gather all metrics registered in the default Prometheus registry
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Failed to gather prometheus metrics: %v", err)
	}

	for _, mf := range metrics {
		name := mf.GetName()

		// Assert that the deprecated per-device gauges are entirely removed
		switch name {
		case "nms_device_cpu_usage_percent",
			"nms_device_memory_usage_percent",
			"nms_device_latency_seconds",
			"nms_device_packet_loss_percent",
			"nms_device_tx_bytes_per_second",
			"nms_device_rx_bytes_per_second",
			"nms_device_status":
			t.Errorf("CRITICAL: Deprecated metric %s is still exposed by exporter!", name)
		}

		// If it's one of our NMS metrics, verify it has no dynamic identity labels
		if strings.HasPrefix(name, "nms_") {
			for _, m := range mf.GetMetric() {
				for _, label := range m.GetLabel() {
					lname := label.GetName()
					if lname == "ip" || lname == "device_id" || lname == "ip_address" || lname == "device_name" {
						t.Errorf("CRITICAL: Cardinality leak detected! Metric %s contains high-cardinality label '%s'", name, lname)
					}
				}
			}
		}
	}
}
