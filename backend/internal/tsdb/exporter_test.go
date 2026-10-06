package tsdb

import (
	"math"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/hafizkrm/Monitoring/backend/internal/cache"
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

	// 1. Deterministic Unit Tests
	payloadOnline := cache.LatestDeviceMetrics{
		DeviceID:    1,
		Name:        "Router A",
		IPAddress:   "192.168.1.1",
		DeviceType:  "router",
		Status:      "online",
		CPUUsage:    45.5,
		MemoryUsage: 0.0, // online actual zero
		Latency:     100.0, // ms
		TxRate:      1.0,   // Mbps
		RxRate:      10.0,  // Mbps
	}

	evt1 := contracts.NewDomainEvent("test", contracts.DomainEventMetricsUpdated, "1", payloadOnline)
	bus.Publish(evt1)
	time.Sleep(10 * time.Millisecond) // wait for processing

	// Validate Online Metrics
	if val := getGaugeValue(exporter.latGauge, "1", "Router A", "router"); val != 0.1 {
		t.Errorf("Latency conversion failed: expected 0.1, got %v", val)
	}
	if val := getGaugeValue(exporter.txGauge, "1", "Router A", "router"); val != 125000.0 {
		t.Errorf("Tx conversion failed: expected 125000, got %v", val)
	}
	if val := getGaugeValue(exporter.rxGauge, "1", "Router A", "router"); val != 1250000.0 {
		t.Errorf("Rx conversion failed: expected 1250000, got %v", val)
	}
	if val := getGaugeValue(exporter.memGauge, "1", "Router A", "router"); val != 0.0 {
		t.Errorf("Online actual zero failed: expected 0.0, got %v", val)
	}
	if val := getGaugeValue(exporter.statGauge, "1", "Router A", "router"); val != 1.0 {
		t.Errorf("Online status failed: expected 1.0, got %v", val)
	}

	// 2. Offline -> NaN Tests
	payloadOffline := cache.LatestDeviceMetrics{
		DeviceID:    2,
		Name:        "Router B",
		IPAddress:   "10.0.0.1",
		DeviceType:  "router",
		Status:      "offline",
		CPUUsage:    99.9, // stale
		TxRate:      5.0,  // stale
	}
	evt2 := contracts.NewDomainEvent("test", contracts.DomainEventMetricsUpdated, "2", payloadOffline)
	bus.Publish(evt2)
	time.Sleep(10 * time.Millisecond)

	if val := getGaugeValue(exporter.cpuGauge, "2", "Router B", "router"); !math.IsNaN(val) {
		t.Errorf("Offline CPU failed: expected NaN, got %v", val)
	}
	if val := getGaugeValue(exporter.txGauge, "2", "Router B", "router"); !math.IsNaN(val) {
		t.Errorf("Offline Tx failed: expected NaN, got %v", val)
	}
	if val := getGaugeValue(exporter.statGauge, "2", "Router B", "router"); val != 0.0 {
		t.Errorf("Offline status failed: expected 0.0, got %v", val)
	}

	// 3. Label Validation: IP Address must not be a label, changing IP should not create new series
	payloadIPChanged := payloadOnline
	payloadIPChanged.IPAddress = "192.168.1.99"
	evt3 := contracts.NewDomainEvent("test", contracts.DomainEventMetricsUpdated, "1", payloadIPChanged)
	bus.Publish(evt3)
	time.Sleep(10 * time.Millisecond)

	metricCount := getMetricSeriesCount(exporter.cpuGauge)
	if metricCount != 2 { // Device 1 and Device 2 only
		t.Errorf("Label validation failed: expected 2 series, got %d (IP change created new series)", metricCount)
	}

	// 4. Parse Error Counter
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

func getGaugeValue(vec *prometheus.GaugeVec, deviceID, deviceName, deviceType string) float64 {
	m := &dto.Metric{}
	vec.WithLabelValues(deviceID, deviceName, deviceType).Write(m)
	return m.GetGauge().GetValue()
}

func getMetricSeriesCount(vec *prometheus.GaugeVec) int {
	ch := make(chan prometheus.Metric, 100)
	go func() {
		vec.Collect(ch)
		close(ch)
	}()
	count := 0
	for range ch {
		count++
	}
	return count
}

func getCounterValue(counter prometheus.Counter) float64 {
	m := &dto.Metric{}
	counter.Write(m)
	return m.GetCounter().GetValue()
}
