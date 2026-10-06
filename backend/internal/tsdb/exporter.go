package tsdb

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/hafizkrm/Monitoring/backend/internal/cache"
	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
	"github.com/hafizkrm/Monitoring/backend/internal/transport/eventbus"
)

// TSDBExporter acts as a passive EventBus subscriber that updates Prometheus gauges.
type TSDBExporter struct {
	sub             *eventbus.Subscriber
	parseErrCounter prometheus.Counter
}

// NewTSDBExporter initializes the exporter and registers all metrics to Prometheus.
func NewTSDBExporter(bus eventbus.EventBus) *TSDBExporter {
	// Subscribing to MetricsUpdated event with a reasonable buffer for backpressure isolation
	sub := bus.Subscribe(contracts.DomainEventMetricsUpdated, 100)

	exp := &TSDBExporter{
		sub: sub,
		parseErrCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nms_exporter_parse_errors_total",
			Help: "Total number of payload type assertion or parse errors",
		}),
	}

	prometheus.MustRegister(exp.parseErrCounter)

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
		_, ok := event.Payload.(cache.LatestDeviceMetrics)
		if !ok {
			// No synchronous logging on hot path, just increment error counter
			e.parseErrCounter.Inc()
			continue
		}
		// Metrics are solely persisted in SQL Database. TSDBExporter no longer exports
		// per-device Prometheus gauges due to cardinality limits.
	}
}
