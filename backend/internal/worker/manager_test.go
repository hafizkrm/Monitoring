package worker

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

func TestManager_CircuitBreaker(t *testing.T) {
	m := &Manager{
		cb:         NewCircuitBreaker(5, 60*time.Second),
		lastUptime: make(map[int]int64),
		logger:     &MockLogger{},
		mu:         sync.RWMutex{},
	}

	deviceID := 1

	tests := []struct {
		name         string
		action       func()
		expectBroken bool
	}{
		{
			name:         "Initial State",
			action:       func() {},
			expectBroken: false,
		},
		{
			name: "Under Threshold (4 failures)",
			action: func() {
				for i := 0; i < 4; i++ {
					m.cb.RecordFailure(deviceID)
				}
			},
			expectBroken: false,
		},
		{
			name: "Threshold Reached (5 failures)",
			action: func() {
				m.cb.RecordFailure(deviceID)
			},
			expectBroken: true,
		},
		{
			name: "Reset After Failure",
			action: func() {
				m.cb.Reset(deviceID)
			},
			expectBroken: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.action()
			if m.cb.IsBroken(deviceID) != tt.expectBroken {
				t.Errorf("isBroken() = %v, want %v", m.cb.IsBroken(deviceID), tt.expectBroken)
			}
		})
	}
}

func TestManager_processJob(t *testing.T) {
	job := Job{
		Device: &models.Device{
			ID:        1,
			Name:      "test-device",
			IPAddress: "192.168.1.1",
		},
		ScheduledAt: time.Now(),
	}

	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		var pollingLogCalled bool
		mockDB := &MockDatabase{
			UpdateDeviceLastPolledAtFunc: func(ctx context.Context, deviceID int) error { return nil },
			InsertPollingLogFunc: func(ctx context.Context, deviceID int, status, message string, durationMs int) error {
				pollingLogCalled = true
				return nil
			},
			InsertDeviceMetricFunc: func(ctx context.Context, metric *models.DeviceMetric) error { return nil },
		}
		mockSNMP := &MockSNMP{
			CollectFunc: func(ctx context.Context, device models.Device) (*models.TelemetrySnapshot, error) {
				return &models.TelemetrySnapshot{DeviceID: device.ID, Status: "up", CollectedAt: time.Now()}, nil
			},
		}
		mockRegistry := &MockRegistry{
			GetFunc: func(name string) (Collector, error) { return mockSNMP, nil },
		}
		log := &MockLogger{}
		m := &Manager{
			db:         mockDB,
			registry:   mockRegistry,
			logger:     log,
			processor:  NewProcessor(mockRegistry, mockDB, mockDB, mockDB, log, &MockRuleCache{}),
			cb:         NewCircuitBreaker(5, 60*time.Second),
			lastUptime: make(map[int]int64),
			mu:         sync.RWMutex{},
			scheduler:  &Scheduler{pendingJobs: make(map[int]struct{})},
		}

		m.processJob(ctx, job, 1)

		if !pollingLogCalled {
			t.Error("expected InsertPollingLog to be called")
		}
		if m.cb.IsBroken(job.Device.ID) {
			t.Error("expected circuit to be closed after success")
		}
	})

	t.Run("SNMP Failure", func(t *testing.T) {
		ctx := context.Background()
		mockDB := &MockDatabase{
			InsertPollingLogFunc: func(ctx context.Context, deviceID int, status, message string, durationMs int) error { return nil },
		}
		mockSNMP := &MockSNMP{
			CollectFunc: func(ctx context.Context, device models.Device) (*models.TelemetrySnapshot, error) {
				return nil, fmt.Errorf("snmp timeout")
			},
		}
		mockRegistry := &MockRegistry{
			GetFunc: func(name string) (Collector, error) { return mockSNMP, nil },
		}
		log := &MockLogger{}
		m := &Manager{
			db:         mockDB,
			registry:   mockRegistry,
			logger:     log,
			processor:  NewProcessor(mockRegistry, mockDB, mockDB, mockDB, log, &MockRuleCache{}),
			cb:         NewCircuitBreaker(5, 60*time.Second),
			lastUptime: make(map[int]int64),
			mu:         sync.RWMutex{},
			scheduler:  &Scheduler{pendingJobs: make(map[int]struct{})},
		}

		m.processJob(ctx, job, 1)

		if m.cb.GetFailures(job.Device.ID) != 1 {
			t.Errorf("expected failure count to be 1, got %d", m.cb.GetFailures(job.Device.ID))
		}
	})
}

func TestManager_StartStop(t *testing.T) {
	// Create a short-lived context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	mockDB := &MockDatabase{
		GetAllEnabledDevicesFunc: func(ctx context.Context) ([]models.Device, error) {
			return nil, nil
		},
	}
	mockSNMP := &MockSNMP{}
	mockRegistry := &MockRegistry{
		GetFunc: func(name string) (Collector, error) { return mockSNMP, nil },
	}
	log := &MockLogger{}

	m := NewManager(config.Config{
		Polling: config.PollingConfig{
			MaxWorkers: 2,
		},
	}, mockDB, log, mockRegistry, &MockRuleCache{})

	// Test Start
	err := m.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !m.started.Load() {
		t.Error("expected manager to be running")
	}

	// Wait a bit for goroutines to start
	time.Sleep(50 * time.Millisecond)

	// Test Stop
	m.Stop()

	if m.started.Load() {
		t.Error("expected manager to be stopped")
	}
}
