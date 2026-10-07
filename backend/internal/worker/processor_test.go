package worker

import (
	"context"
	"testing"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

func TestProcessor_validateMetrics(t *testing.T) {
	p := &Processor{
		logger: &MockLogger{},
	} // Minimal processor for testing validateMetrics

	tests := []struct {
		name    string
		metrics *models.TelemetrySnapshot
		wantErr bool
	}{
		{
			name: "Valid metrics",
			metrics: &models.TelemetrySnapshot{
				CPUUsage:    50.0,
				MemoryUsage: 50.0,
			},
			wantErr: false,
		},
		{
			name: "CPU Usage too high (anomaly logged but no error returned currently)",
			metrics: &models.TelemetrySnapshot{
				CPUUsage:    150.0,
				MemoryUsage: 50.0,
			},
			wantErr: false, // current implementation only warns
		},
		{
			name: "Negative memory usage (anomaly logged)",
			metrics: &models.TelemetrySnapshot{
				CPUUsage:    50.0,
				MemoryUsage: -10.0,
			},
			wantErr: false, // current implementation only warns
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := p.validateMetrics(tt.metrics); (err != nil) != tt.wantErr {
				t.Errorf("Processor.validateMetrics() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProcessor_ProcessMetrics(t *testing.T) {
	ctx := context.Background()

	metricsSaved := false
	interfacesSaved := false

	mockDB := &MockDatabase{
		InsertDeviceMetricFunc: func(ctx context.Context, metric *models.DeviceMetric) error {
			metricsSaved = true
			return nil
		},
		BatchInsertInterfaceMetricsFunc: func(ctx context.Context, metrics []*models.InterfaceMetric) error {
			interfacesSaved = true
			return nil
		},
	}

	mockRegistry := &MockRegistry{
		GetFunc: func(name string) (Collector, error) { return &MockSNMP{}, nil },
	}
	p := NewProcessor(mockRegistry, mockDB, mockDB, mockDB, &MockLogger{}, &MockRuleCache{})

	metrics := &models.TelemetrySnapshot{
		DeviceID:    1,
		CPUUsage:    45.0,
		MemoryUsage: 60.0,
		Interfaces: []models.InterfaceSnapshot{
			{InterfaceIndex: 1, InterfaceName: "eth0", Status: "up"},
		},
		CollectedAt: time.Now(),
	}

	err := p.ProcessMetrics(ctx, models.Device{ID: 1}, metrics)
	if err != nil {
		t.Fatalf("ProcessMetrics failed: %v", err)
	}

	if !metricsSaved {
		t.Error("expected device metrics to be saved")
	}
	if !interfacesSaved {
		t.Error("expected interface metrics to be saved")
	}
}

func TestProcessor_EvaluateThresholds(t *testing.T) {
	ctx := context.Background()
	var createdIncidents []string

	mockDB := &MockDatabase{
		CreateIncidentFunc: func(ctx context.Context, deviceID int, incidentType string, description string) (int64, error) {
			createdIncidents = append(createdIncidents, incidentType)
			return 1, nil
		},
		GetActiveIncidentsByDeviceFunc: func(ctx context.Context, deviceID int) (map[string]int64, error) {
			return make(map[string]int64), nil
		},
		CreateAlertFunc: func(ctx context.Context, incidentID interface{}, severity string, message string) error {
			return nil
		},
	}

	mockRegistry := &MockRegistry{}
	p := NewProcessor(mockRegistry, mockDB, mockDB, mockDB, &MockLogger{}, &MockRuleCache{})

	t.Run("SNMP High CPU and RAM", func(t *testing.T) {
		createdIncidents = nil // reset
		metrics := &models.TelemetrySnapshot{
			DeviceID:    1,
			HasCPU:      true,
			CPUUsage:    95.0, // above 80
			HasMemory:   true,
			MemoryUsage: 90.0, // above 85
			Status:      "up",
		}

		p.evaluateThresholds(ctx, models.Device{ID: metrics.DeviceID}, metrics)

		if len(createdIncidents) != 2 {
			t.Errorf("expected 2 incidents (cpu, ram), got %d: %v", len(createdIncidents), createdIncidents)
		}
	})

	t.Run("ICMP Unavailable CPU and RAM", func(t *testing.T) {
		createdIncidents = nil // reset
		metrics := &models.TelemetrySnapshot{
			DeviceID:    2,
			HasCPU:      false,
			CPUUsage:    0.0,
			HasMemory:   false,
			MemoryUsage: 0.0,
			Status:      "up",
		}

		p.evaluateThresholds(ctx, models.Device{ID: metrics.DeviceID}, metrics)

		if len(createdIncidents) > 0 {
			t.Errorf("expected 0 incidents for ICMP, got %d: %v", len(createdIncidents), createdIncidents)
		}
	})
}
