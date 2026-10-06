package worker

import (
	"context"

	"github.com/hafizkrm/Monitoring/backend/internal/models"
	"github.com/hafizkrm/Monitoring/backend/internal/snmp"
)

// SNMP collector abstraction
type SNMPClient interface {
	CollectDeviceMetrics(
		ctx context.Context,
		device models.Device,
	) (*snmp.DeviceMetrics, error)

	// GetNetworkStats performs a fast ICMP ping check (used for CB recovery)
	GetNetworkStats(ctx context.Context, ip string) (latency int, jitter float64, loss float64, err error)
}

// Device repository abstraction
type DeviceRepository interface {
	GetAllEnabledDevices(
		ctx context.Context,
	) ([]models.Device, error)

	UpdateDeviceStatus(
		ctx context.Context,
		deviceID int,
		reachability string,
		snmpStatus string,
		overallStatus string,
	) error
}

// Metrics repository abstraction
type MetricsRepository interface {
	InsertDeviceMetric(
		ctx context.Context,
		metric *models.DeviceMetric,
	) error

	BatchInsertInterfaceMetrics(
		ctx context.Context,
		metrics []*models.InterfaceMetric,
	) error
}

// Incident repository abstraction
type IncidentRepository interface {
	CreateIncident(ctx context.Context, deviceID int, incidentType string, description string) (int64, error)
	ResolveIncident(ctx context.Context, deviceID int, incidentType string) error
	GetActiveIncident(ctx context.Context, deviceID int, incidentType string) (int64, error)
	GetActiveIncidentsByDevice(ctx context.Context, deviceID int) (map[string]int64, error)
	CreateAlert(ctx context.Context, incidentID interface{}, alertType string, message string) error
}

// DatabaseClient combines all worker database operations
type DatabaseClient interface {
	DeviceRepository
	MetricsRepository
	IncidentRepository

	GetAllEnabledDevices(ctx context.Context) ([]models.Device, error)
	UpdateDeviceLastPolledAt(ctx context.Context, deviceID int) error
	InsertPollingLog(ctx context.Context, deviceID int, status string, message string, durationMs int) error
	InsertActivityLog(ctx context.Context, userId *int64, username string, action string, module string, description string, ipAddress string) error
	CleanupOldData(ctx context.Context, metricsDays int, logsDays int) (map[string]int64, error)
	GetLatestMetrics(ctx context.Context) ([]map[string]interface{}, error)
}

// Logger abstraction
type Logger interface {
	Info(
		msg string,
		fields map[string]interface{},
	)

	Warn(
		msg string,
		fields map[string]interface{},
	)

	Error(
		msg string,
		fields map[string]interface{},
	)

	Debug(
		msg string,
		fields map[string]interface{},
	)
}
