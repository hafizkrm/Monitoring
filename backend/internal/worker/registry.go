package worker

import (
	"fmt"
	"sync"

	"strings"

	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

// DefaultCollectorRegistry implements CollectorRegistry
type DefaultCollectorRegistry struct {
	mu         sync.RWMutex
	collectors map[string]Collector
}

// NewCollectorRegistry creates a new instance of DefaultCollectorRegistry
func NewCollectorRegistry() *DefaultCollectorRegistry {
	return &DefaultCollectorRegistry{
		collectors: make(map[string]Collector),
	}
}

// Register adds a new collector to the registry under a specific protocol/type name
func (r *DefaultCollectorRegistry) Register(name string, collector Collector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.collectors[name] = collector
}

// Get retrieves a registered collector by its name
func (r *DefaultCollectorRegistry) Get(name string) (Collector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if c, ok := r.collectors[name]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("unsupported protocol/device type: %s", name)
}

// DetermineCollectorName determines which collector to use for a given device.
// This provides the routing logic for M12-B and M12-C.
func DetermineCollectorName(device *models.Device) string {
	if strings.EqualFold(device.DeviceType, "icmp") || strings.EqualFold(device.DeviceType, "icmp_only") {
		return "icmp"
	}
	return "snmp"
}
