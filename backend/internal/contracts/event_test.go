package contracts

import (
	"encoding/json"
	"testing"
)

func TestNewDomainEvent(t *testing.T) {
	source := "test-worker"
	payload := map[string]string{"key": "value"}
	
	event := NewDomainEvent(source, DomainEventMetricsUpdated, "dev-1", payload)

	if event.EventID == "" {
		t.Error("Expected EventID to be populated")
	}
	if event.Timestamp.IsZero() {
		t.Error("Expected Timestamp to be populated")
	}
	if event.Source != source {
		t.Errorf("Expected Source %s, got %s", source, event.Source)
	}
	if event.Type != DomainEventMetricsUpdated {
		t.Errorf("Expected Type %s, got %s", DomainEventMetricsUpdated, event.Type)
	}
	
	p, ok := event.Payload.(map[string]string)
	if !ok || p["key"] != "value" {
		t.Errorf("Expected Payload key=value, got %v", event.Payload)
	}
}

func TestDomainEvent_Serialization(t *testing.T) {
	source := "test-source"
	payload := map[string]interface{}{"metric": 123.45}
	
	event := NewDomainEvent(source, DomainEventAlertCreated, "dev-2", payload)

	// Test Serialization
	data, err := event.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize to JSON: %v", err)
	}

	// Test Deserialization
	deserialized, err := FromJSON(data)
	if err != nil {
		t.Fatalf("Failed to deserialize from JSON: %v", err)
	}

	if deserialized.EventID != event.EventID {
		t.Errorf("Expected EventID %s, got %s", event.EventID, deserialized.EventID)
	}
	
	// Compare timestamp (Unix milliseconds or standard precision)
	if deserialized.Timestamp.Unix() != event.Timestamp.Unix() {
		t.Errorf("Expected Timestamp %v, got %v", event.Timestamp.Unix(), deserialized.Timestamp.Unix())
	}
	
	if deserialized.Source != event.Source {
		t.Errorf("Expected Source %s, got %s", event.Source, deserialized.Source)
	}
	if deserialized.Type != event.Type {
		t.Errorf("Expected Type %s, got %s", event.Type, deserialized.Type)
	}

	// Deserializing JSON interface{} mapping results in map[string]interface{}
	pBytes, _ := json.Marshal(deserialized.Payload)
	var expectedPayloadMap map[string]interface{}
	json.Unmarshal(pBytes, &expectedPayloadMap)
	
	if expectedPayloadMap["metric"] != 123.45 {
		t.Errorf("Expected payload metric to be 123.45, got %v", expectedPayloadMap["metric"])
	}
}
