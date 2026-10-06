package tsdb

import (
	"log"
	"math"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/hafizkrm/Monitoring/backend/internal/cache"
	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
	"github.com/hafizkrm/Monitoring/backend/internal/transport/eventbus"
)

// TSDBExporter acts as a passive EventBus subscriber that updates Prometheus gauges.
type TSDBExporter struct {
	sub             *eventbus.Subscriber
	cpuGauge        *prometheus.GaugeVec
	memGauge        *prometheus.GaugeVec
	latGauge        *prometheus.GaugeVec
	lossGauge       *prometheus.GaugeVec
	txGauge         *prometheus.GaugeVec
	rxGauge         *prometheus.GaugeVec
	statGauge       *prometheus.GaugeVec
	parseErrCounter prometheus.Counter
}

// NewTSDBExporter initializes the exporter and registers all metrics to Prometheus.
func NewTSDBExporter(bus eventbus.EventBus) *TSDBExporter {
	// Subscribing to MetricsUpdated event with a reasonable buffer for backpressure isolation
	sub := bus.Subscribe(contracts.DomainEventMetricsUpdated, 100)

	// ip_address is removed from labels to maintain stable device identity
	labels := []string{"device_id", "device_name", "device_type"}

	exp := &TSDBExporter{
		sub: sub,
		cpuGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "nms_device_cpu_usage_percent",
			Help: "CPU usage percentage of the device",
		}, labels),
		memGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "nms_device_memory_usage_percent",
			Help: "Memory usage percentage of the device",
		}, labels),
		latGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "nms_device_latency_seconds",
			Help: "Ping latency in seconds",
		}, labels),
		lossGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "nms_device_packet_loss_percent",
			Help: "Packet loss percentage",
		}, labels),
		txGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "nms_device_tx_bytes_per_second",
			Help: "Transmit rate in bytes per second",
		}, labels),
		rxGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "nms_device_rx_bytes_per_second",
			Help: "Receive rate in bytes per second",
		}, labels),
		statGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "nms_device_status",
			Help: "Device status (1 = Online, 0 = Offline)",
		}, labels),
		parseErrCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nms_exporter_parse_errors_total",
			Help: "Total number of payload type assertion or parse errors",
		}),
	}

	prometheus.MustRegister(exp.cpuGauge, exp.memGauge, exp.latGauge, exp.lossGauge, exp.txGauge, exp.rxGauge, exp.statGauge, exp.parseErrCounter)

	// Phase 3: Prometheus Observability - Export dropped count for TSDB Exporter
	prometheus.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Name: "nms_eventbus_dropped_messages_total",
			Help: "Total dropped messages by eventbus subscriber",
			ConstLabels: prometheus.Labels{"component": "tsdb_exporter"},
		},
		func() float64 {
			return float64(sub.DroppedCount())
		},
	))

	return exp
}

// Start begins processing domain events in a blocking loop.
func (e *TSDBExporter) Start() {
	log.Println("TSDB Prometheus Exporter is running...")
	for event := range e.sub.Channel {
		metrics, ok := event.Payload.(cache.LatestDeviceMetrics)
		if !ok {
			// No synchronous logging on hot path, just increment error counter
			e.parseErrCounter.Inc()
			continue
		}
		e.updateMetrics(metrics)
	}
}

// updateMetrics updates the internal prometheus.GaugeVec structures. (Thread-safe)
func (e *TSDBExporter) updateMetrics(m cache.LatestDeviceMetrics) {
	devID := strconv.Itoa(m.DeviceID)

	statusVal := 0.0
	cpuVal := m.CPUUsage
	memVal := m.MemoryUsage
	latVal := m.Latency / 1000.0                   // ms to seconds
	lossVal := m.PacketLoss
	txVal := (m.TxRate * 1000000.0) / 8.0         // Mbps to bytes/sec
	rxVal := (m.RxRate * 1000000.0) / 8.0         // Mbps to bytes/sec

	if m.Status == "online" || m.Status == "degraded" {
		statusVal = 1.0
	} else if m.Status == "down" || m.Status == "offline" {
		statusVal = 0.0
		// Offline telemetry NaN semantics
		cpuVal = math.NaN()
		memVal = math.NaN()
		latVal = math.NaN()
		lossVal = math.NaN()
		txVal = math.NaN()
		rxVal = math.NaN()
	}

	lbls := prometheus.Labels{
		"device_id":   devID,
		"device_name": m.Name,
		"device_type": m.DeviceType,
	}

	e.cpuGauge.With(lbls).Set(cpuVal)
	e.memGauge.With(lbls).Set(memVal)
	e.latGauge.With(lbls).Set(latVal)
	e.lossGauge.With(lbls).Set(lossVal)
	e.txGauge.With(lbls).Set(txVal)
	e.rxGauge.With(lbls).Set(rxVal)
	e.statGauge.With(lbls).Set(statusVal)
}
