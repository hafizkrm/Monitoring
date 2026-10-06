package contracts

import "time"

// WSEventEnvelope adalah standar baku (envelope) untuk semua pesan WebSocket
// yang dikirim dari Backend ke Frontend.
type WSEventEnvelope struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	DeviceID  string      `json:"device_id,omitempty"`
	Payload   interface{} `json:"payload"`
}

// Konstanta nama-nama event agar tidak ada typo di Backend
const (
	EventDeviceConnected    = "device.connected"
	EventDeviceDisconnected = "device.disconnected"
	EventMetricsUpdated     = "metrics.updated"
	EventAlertCreated       = "alert.created"
	EventLogCreated         = "log.created"
)
