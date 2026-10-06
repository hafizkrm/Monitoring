package contracts

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventType is a strong type for all domain events.
type EventType string

const (
	// Add predefined domain event types here
	DomainEventDeviceConnected    EventType = "device.connected"
	DomainEventDeviceDisconnected EventType = "device.disconnected"
	DomainEventMetricsUpdated     EventType = "metrics.updated"
	DomainEventAlertCreated       EventType = "alert.created"
	DomainEventLogCreated         EventType = "log.created"
)

// DomainEvent is the standard event envelope for all internal domain events.
// It defines WHAT happened, independent of HOW it is transmitted (e.g., WebSocket).
type DomainEvent struct {
	EventID   string      `json:"event_id"`
	Timestamp time.Time   `json:"timestamp"`
	Source    string      `json:"source"` // e.g., "snmp-worker", "device-manager"
	Type      EventType   `json:"type"`
	DeviceID  string      `json:"device_id,omitempty"`
	Payload   interface{} `json:"payload"`
}

// NewDomainEvent creates a new standard DomainEvent with an auto-generated UUID and current timestamp.
func NewDomainEvent(source string, eventType EventType, deviceID string, payload interface{}) DomainEvent {
	return DomainEvent{
		EventID:   uuid.NewString(),
		Timestamp: time.Now().UTC(),
		Source:    source,
		Type:      eventType,
		DeviceID:  deviceID,
		Payload:   payload,
	}
}

// ToJSON serializes the domain event to JSON.
func (e *DomainEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes a domain event from JSON.
func FromJSON(data []byte) (*DomainEvent, error) {
	var event DomainEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// EventPublisher defines how internal components publish domain events.
type EventPublisher interface {
	Publish(event DomainEvent)
}
