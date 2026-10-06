package worker

import (
	"context"
	"testing"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/models"
	"github.com/hafizkrm/Monitoring/backend/internal/snmp"
)

func TestProcessor_validateMetrics(t *testing.T) {
	p := &Processor{
		logger: &MockLogger{},
	} // Minimal processor for testing validateMetrics

	tests := []struct {
		name    string
		metrics *snmp.DeviceMetrics
		wantErr bool
	}{
		{
			name: "Valid metrics",
			metrics: &snmp.DeviceMetrics{
				CPUUsage:    50.0,
				MemoryUsage: 50.0,
			},
			wantErr: false,
		},
		{
			name: "CPU Usage too high (anomaly logged but no error returned currently)",
			metrics: &snmp.DeviceMetrics{
				CPUUsage:    150.0,
				MemoryUsage: 50.0,
			},
			wantErr: false, // current implementation only warns
		},
		{
			name: "Negative memory usage (anomaly logged)",
			metrics: &snmp.DeviceMetrics{
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

	p := NewProcessor(&snmp.Client{}, mockDB, mockDB, mockDB, &MockLogger{})

	metrics := &snmp.DeviceMetrics{
		DeviceID:    1,
		CPUUsage:    45.0,
		MemoryUsage: 60.0,
		Interfaces: []snmp.InterfaceMetrics{
			{InterfaceIndex: 1, InterfaceName: "eth0", Status: "up"},
		},
		CollectedAt: time.Now(),
	}

	err := p.ProcessMetrics(ctx, metrics)
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
