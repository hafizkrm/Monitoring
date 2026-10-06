package worker

import (
	"context"
	"fmt"

	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

// MockLogger is a simple implementation of logger.Logger for testing
type MockLogger struct{}

func (m *MockLogger) Debug(message string, fields map[string]interface{}) {}
func (m *MockLogger) Info(message string, fields map[string]interface{})  {}
func (m *MockLogger) Warn(message string, fields map[string]interface{})  {}
func (m *MockLogger) Error(message string, fields map[string]interface{}) {}
func (m *MockLogger) Sync() error                                         { return nil }

type MockRuleCache struct {
	rules map[string]models.ThresholdRule
}

func (m *MockRuleCache) GetRulesForDevice(deviceID int) map[string]models.ThresholdRule {
	if m.rules == nil {
		m.rules = make(map[string]models.ThresholdRule)
	}
	return m.rules
}

func (m *MockRuleCache) Refresh(ctx context.Context) error {
	return nil
}

// MockDatabase is a mock implementation of DatabaseClient
type MockDatabase struct {
	GetAllEnabledDevicesFunc        func(ctx context.Context) ([]models.Device, error)
	UpdateDeviceStatusFunc          func(ctx context.Context, deviceID int, reachability string, snmpStatus string, overallStatus string) error
	InsertDeviceMetricFunc          func(ctx context.Context, metric *models.DeviceMetric) error
	BatchInsertDeviceMetricsFunc    func(ctx context.Context, metrics []*models.DeviceMetric) error
	BatchInsertInterfaceMetricsFunc func(ctx context.Context, metrics []*models.InterfaceMetric) error
	UpdateDeviceLastPolledAtFunc    func(ctx context.Context, deviceID int) error
	InsertPollingLogFunc            func(ctx context.Context, deviceID int, status string, message string, durationMs int) error
	InsertActivityLogFunc           func(ctx context.Context, userId *int64, username string, action string, module string, description string, ipAddress string) error
	CleanupOldDataFunc              func(ctx context.Context, metricsDays int, logsDays int) (map[string]int64, error)
	GetLatestMetricsFunc            func(ctx context.Context) ([]map[string]interface{}, error)
	CreateIncidentFunc              func(ctx context.Context, deviceID int, incidentType string, description string) (int64, error)
	ResolveIncidentFunc             func(ctx context.Context, deviceID int, incidentType string) error
	GetActiveIncidentFunc           func(ctx context.Context, deviceID int, incidentType string) (int64, error)
	GetActiveIncidentsByDeviceFunc  func(ctx context.Context, deviceID int) (map[string]int64, error)
	CreateAlertFunc                 func(ctx context.Context, incidentID interface{}, alertType string, message string) error
}

func (m *MockDatabase) GetAllEnabledDevices(ctx context.Context) ([]models.Device, error) {
	if m.GetAllEnabledDevicesFunc != nil {
		return m.GetAllEnabledDevicesFunc(ctx)
	}
	return nil, nil
}

func (m *MockDatabase) UpdateDeviceStatus(ctx context.Context, deviceID int, reachability string, snmpStatus string, overallStatus string) error {
	if m.UpdateDeviceStatusFunc != nil {
		return m.UpdateDeviceStatusFunc(ctx, deviceID, reachability, snmpStatus, overallStatus)
	}
	return nil
}

func (m *MockDatabase) InsertDeviceMetric(ctx context.Context, metric *models.DeviceMetric) error {
	if m.InsertDeviceMetricFunc != nil {
		return m.InsertDeviceMetricFunc(ctx, metric)
	}
	return nil
}

func (m *MockDatabase) BatchInsertDeviceMetrics(ctx context.Context, metrics []*models.DeviceMetric) error {
	if m.BatchInsertDeviceMetricsFunc != nil {
		return m.BatchInsertDeviceMetricsFunc(ctx, metrics)
	}
	return nil
}

func (m *MockDatabase) BatchInsertInterfaceMetrics(ctx context.Context, metrics []*models.InterfaceMetric) error {
	if m.BatchInsertInterfaceMetricsFunc != nil {
		return m.BatchInsertInterfaceMetricsFunc(ctx, metrics)
	}
	return nil
}

func (m *MockDatabase) UpdateDeviceLastPolledAt(ctx context.Context, deviceID int) error {
	if m.UpdateDeviceLastPolledAtFunc != nil {
		return m.UpdateDeviceLastPolledAtFunc(ctx, deviceID)
	}
	return nil
}

func (m *MockDatabase) GetLatestMetrics(ctx context.Context) ([]map[string]interface{}, error) {
	if m.GetLatestMetricsFunc != nil {
		return m.GetLatestMetricsFunc(ctx)
	}
	return nil, nil
}

func (m *MockDatabase) InsertPollingLog(ctx context.Context, deviceID int, status string, message string, durationMs int) error {
	if m.InsertPollingLogFunc != nil {
		return m.InsertPollingLogFunc(ctx, deviceID, status, message, durationMs)
	}
	return nil
}

func (m *MockDatabase) CleanupOldData(ctx context.Context, metricsDays int, logsDays int) (map[string]int64, error) {
	if m.CleanupOldDataFunc != nil {
		return m.CleanupOldDataFunc(ctx, metricsDays, logsDays)
	}
	return map[string]int64{}, nil
}

func (m *MockDatabase) InsertActivityLog(ctx context.Context, userId *int64, username string, action string, module string, description string, ipAddress string) error {
	if m.InsertActivityLogFunc != nil {
		return m.InsertActivityLogFunc(ctx, userId, username, action, module, description, ipAddress)
	}
	return nil
}

// MockSNMP is a mock implementation of SNMPClient
type MockSNMP struct {
	CollectFunc func(ctx context.Context, device models.Device) (*models.TelemetrySnapshot, error)
	GetNetworkStatsFunc      func(ctx context.Context, ip string) (int, float64, float64, error)
}

func (m *MockSNMP) Collect(ctx context.Context, device models.Device) (*models.TelemetrySnapshot, error) {
	if m.CollectFunc != nil {
		return m.CollectFunc(ctx, device)
	}
	return &models.TelemetrySnapshot{}, nil
}

func (m *MockSNMP) GetNetworkStats(ctx context.Context, ip string) (int, float64, float64, error) {
	if m.GetNetworkStatsFunc != nil {
		return m.GetNetworkStatsFunc(ctx, ip)
	}
	// Default: simulate unreachable (circuit stays broken in tests)
	return 0, 0, 100, nil
}

// MockRegistry is a mock implementation of CollectorRegistry
type MockRegistry struct {
	GetFunc      func(name string) (Collector, error)
	RegisterFunc func(name string, collector Collector)
}

func (m *MockRegistry) Register(name string, collector Collector) {
	if m.RegisterFunc != nil {
		m.RegisterFunc(name, collector)
	}
}

func (m *MockRegistry) Get(name string) (Collector, error) {
	if m.GetFunc != nil {
		return m.GetFunc(name)
	}
	return nil, fmt.Errorf("mock registry default error")
}

func (m *MockDatabase) CreateIncident(ctx context.Context, deviceID int, incidentType string, description string) (int64, error) {
	if m.CreateIncidentFunc != nil {
		return m.CreateIncidentFunc(ctx, deviceID, incidentType, description)
	}
	return 1, nil
}
func (m *MockDatabase) ResolveIncident(ctx context.Context, deviceID int, incidentType string) error {
	if m.ResolveIncidentFunc != nil {
		return m.ResolveIncidentFunc(ctx, deviceID, incidentType)
	}
	return nil
}
func (m *MockDatabase) GetActiveIncident(ctx context.Context, deviceID int, incidentType string) (int64, error) {
	if m.GetActiveIncidentFunc != nil {
		return m.GetActiveIncidentFunc(ctx, deviceID, incidentType)
	}
	return 0, nil
}
func (m *MockDatabase) GetActiveIncidentsByDevice(ctx context.Context, deviceID int) (map[string]int64, error) {
	if m.GetActiveIncidentsByDeviceFunc != nil {
		return m.GetActiveIncidentsByDeviceFunc(ctx, deviceID)
	}
	return make(map[string]int64), nil
}
func (m *MockDatabase) CreateAlert(ctx context.Context, incidentID interface{}, alertType string, message string) error {
	if m.CreateAlertFunc != nil {
		return m.CreateAlertFunc(ctx, incidentID, alertType, message)
	}
	return nil
}
