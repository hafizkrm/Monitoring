package worker

import (
	"testing"

	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

func TestDefaultCollectorRegistry(t *testing.T) {
	registry := NewCollectorRegistry()
	mockCollector := &MockSNMP{}

	t.Run("Register and Get", func(t *testing.T) {
		registry.Register("snmp", mockCollector)
		
		c, err := registry.Get("snmp")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if c != mockCollector {
			t.Errorf("expected to get the registered collector")
		}
	})

	t.Run("Get Unsupported Protocol", func(t *testing.T) {
		_, err := registry.Get("unknown")
		if err == nil {
			t.Errorf("expected error for unsupported protocol")
		}
		expectedErrStr := "unsupported protocol/device type: unknown"
		if err.Error() != expectedErrStr {
			t.Errorf("expected error '%s', got '%s'", expectedErrStr, err.Error())
		}
	})

	t.Run("DetermineCollectorName Default", func(t *testing.T) {
		device := &models.Device{DeviceType: "ubnt"}
		name := DetermineCollectorName(device)
		if name != "snmp" {
			t.Errorf("expected default collector to be 'snmp', got '%s'", name)
		}
	})

	t.Run("DetermineCollectorName ICMP", func(t *testing.T) {
		device := &models.Device{DeviceType: "icmp_only"}
		name := DetermineCollectorName(device)
		if name != "icmp" {
			t.Errorf("expected collector to be 'icmp', got '%s'", name)
		}
	})
}
