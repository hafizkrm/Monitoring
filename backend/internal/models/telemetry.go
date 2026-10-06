package models

import "time"

// TelemetrySnapshot represents a generalized snapshot of device telemetry
// collected by any Collector (SNMP, ICMP, API, etc.)
type TelemetrySnapshot struct {
	DeviceID           int
	Hostname           string
	Uptime             int64
	CPUUsage           float64
	MemoryUsage        float64
	MemoryTotal        int64
	MemoryUsed         int64
	Status             string
	ReachabilityStatus string
	SNMPStatus         string
	CollectedAt        time.Time
	Interfaces         []InterfaceSnapshot
	LatencyMs          int
	Jitter             float64
	PacketLoss         float64
	SignalStrength     float64
	CCQ                float64
	TxRate             float64
	RxRate             float64
	Airtime            float64
	SSID               string
	Frequency          string
	Model              string
	Temperature        float64
	Voltage            float64

	// Capability flags for M12-D Threshold Safety
	HasCPU      bool
	HasMemory   bool
	HasWireless bool
}

// InterfaceSnapshot represents a generalized snapshot of network interface metrics
type InterfaceSnapshot struct {
	DeviceID       int
	InterfaceIndex int
	InterfaceName  string
	InterfaceAlias string
	Status         string
	InOctets       int64
	OutOctets      int64
	InErrors       int64
	OutErrors      int64
	InDiscards     int64
	OutDiscards    int64
	Speed          int64
	RxMbps         float64
	TxMbps         float64
	CollectedAt    time.Time
}
