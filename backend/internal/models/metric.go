package models

import (
	"time"
)

// DeviceMetric represents CPU, RAM, and system metrics collected from a device
type DeviceMetric struct {
	ID                 int64     `json:"id"`
	DeviceID           int       `json:"device_id"`
	CPUUsage           float64   `json:"cpu_usage"`           // Percentage 0-100
	MemoryUsage        float64   `json:"memory_usage"`        // Percentage 0-100
	MemoryTotal        int64     `json:"memory_total"`        // Bytes
	MemoryUsed         int64     `json:"memory_used"`         // Bytes
	Uptime             int64     `json:"uptime"`              // Seconds
	Status             string    `json:"status"`              // up, down, degraded, unknown
	ReachabilityStatus string    `json:"reachability_status"` // up, down, unknown
	SNMPStatus         string    `json:"snmp_status"`         // up, down, unknown
	SignalStrength     float64   `json:"signal_strength"`     // dBm
	CCQ                float64   `json:"ccq"`                 // Percentage
	TxRate             float64   `json:"tx_rate"`             // Mbps
	RxRate             float64   `json:"rx_rate"`             // Mbps
	LatencyMs          float64   `json:"latency_ms"`          // ms
	Latency            float64   `json:"latency"`             // ms
	PacketLoss         float64   `json:"packet_loss"`         // Percentage
	Jitter             float64   `json:"jitter"`              // ms
	Model              string    `json:"model"`               // e.g. RB750Gr3, CCR1036
	Temperature        float64   `json:"temperature"`
	Voltage            float64   `json:"voltage"`
	CollectedAt        time.Time `json:"collected_at"`
	CreatedAt          time.Time `json:"created_at"`
}

// IsValid checks if metric values are within valid ranges
func (m *DeviceMetric) IsValid() bool {
	if m.CPUUsage < 0 || m.CPUUsage > 100 {
		return false
	}
	if m.MemoryUsage < 0 || m.MemoryUsage > 100 {
		return false
	}
	if m.Status != "up" && m.Status != "down" && m.Status != "degraded" && m.Status != "unknown" {
		return false
	}
	return true
}

// InterfaceMetric represents network interface metrics (bandwidth, errors, etc)
type InterfaceMetric struct {
	ID              int64     `json:"id"`
	DeviceID        int       `json:"device_id"`
	InterfaceIndex  int       `json:"interface_index"`
	InterfaceName   string    `json:"interface_name"`
	InterfaceAlias  string    `json:"interface_alias"`
	InterfaceStatus string    `json:"interface_status"` // up, down
	InOctets        int64     `json:"in_octets"`        // Bytes in
	OutOctets       int64     `json:"out_octets"`       // Bytes out
	InErrors        int64     `json:"in_errors"`
	OutErrors       int64     `json:"out_errors"`
	InDiscards      int64     `json:"in_discards"`
	OutDiscards     int64     `json:"out_discards"`
	InterfaceSpeed  int64     `json:"interface_speed"` // Bits per second
	RxMbps          float64   `json:"rx_mbps"`
	TxMbps          float64   `json:"tx_mbps"`
	CollectedAt     time.Time `json:"collected_at"`
	CreatedAt       time.Time `json:"created_at"`
}

// DeviceStatusHistory tracks status changes for alerting
type DeviceStatusHistory struct {
	ID           int64     `json:"id"`
	DeviceID     int       `json:"device_id"`
	StatusBefore string    `json:"status_before"`
	StatusAfter  string    `json:"status_after"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
}

// PollingLog tracks polling attempts for debugging
type PollingLog struct {
	ID         int64     `json:"id"`
	DeviceID   int       `json:"device_id"`
	Status     string    `json:"status"` // success, timeout, error
	Message    string    `json:"message"`
	ErrorCode  string    `json:"error_code"`
	DurationMS int       `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}
