package tsdb

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
	"github.com/hafizkrm/Monitoring/backend/internal/transport/eventbus"
)

func TestTSDBExporter_ConversionsAndSemantics(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	bus := eventbus.NewInMemoryEventBus()
	exporter := NewTSDBExporter(bus)
	go exporter.Start()

	// Parse Error Counter
	initialErrors := getCounterValue(exporter.parseErrCounter)
	evtErr := contracts.NewDomainEvent("test", contracts.DomainEventMetricsUpdated, "1", "invalid_payload_type")
	bus.Publish(evtErr)
	time.Sleep(10 * time.Millisecond)

	newErrors := getCounterValue(exporter.parseErrCounter)
	if newErrors != initialErrors+1 {
		t.Errorf("Parse error counter failed: expected %v, got %v", initialErrors+1, newErrors)
	}

	bus.Shutdown()
}

func getCounterValue(counter prometheus.Counter) float64 {
	m := &dto.Metric{}
	counter.Write(m)
	return m.GetCounter().GetValue()
}
