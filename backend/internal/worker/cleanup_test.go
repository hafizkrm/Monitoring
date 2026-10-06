package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestStartCleanupTask_Metrics(t *testing.T) {
	mockDB := &MockDatabase{}
	mockDB.CleanupOldDataFunc = func(ctx context.Context, metricsDays int, logsDays int) (map[string]int64, error) {
		return map[string]int64{
			"device_metrics": 5000,
			"polling_logs":   200,
		}, nil
	}

	cfg := config.Config{
		Polling: config.PollingConfig{
			CleanupInterval:      "10ms", // Fast for testing
			MetricsRetentionDays: 30,
			LogsRetentionDays:    30,
		},
	}

	mockLogger := &MockLogger{}
	m := NewManager(cfg, mockDB, mockLogger, nil)

	ctx, cancel := context.WithCancel(context.Background())

	// Run cleanup task briefly
	go m.startCleanupTask(ctx)

	// Wait for at least one tick
	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond) // allow shutdown

	// Verify metrics
	successCount := testutil.ToFloat64(cleanupExecutionsTotal.WithLabelValues("success"))
	if successCount < 1 {
		t.Errorf("Expected success execution metric to be at least 1, got %v", successCount)
	}

	deviceMetricsRows := testutil.ToFloat64(cleanupRowsDeletedTotal.WithLabelValues("device_metrics"))
	if deviceMetricsRows < 5000 {
		t.Errorf("Expected at least 5000 rows deleted in metric, got %v", deviceMetricsRows)
	}
}

func TestStartCleanupTask_ErrorMetrics(t *testing.T) {
	mockDB := &MockDatabase{}
	mockDB.CleanupOldDataFunc = func(ctx context.Context, metricsDays int, logsDays int) (map[string]int64, error) {
		return map[string]int64{"device_metrics": 1000}, errors.New("simulated error")
	}

	cfg := config.Config{
		Polling: config.PollingConfig{
			CleanupInterval:      "10ms", // Fast for testing
		},
	}

	mockLogger := &MockLogger{}
	m := NewManager(cfg, mockDB, mockLogger, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go m.startCleanupTask(ctx)

	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)

	// Verify error metrics
	errorCount := testutil.ToFloat64(cleanupExecutionsTotal.WithLabelValues("error"))
	if errorCount < 1 {
		t.Errorf("Expected error execution metric to be at least 1, got %v", errorCount)
	}
}
