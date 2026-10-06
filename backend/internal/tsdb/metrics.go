package tsdb

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Device Level Metrics
	DeviceCPU = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nms_device_cpu_usage_percent",
			Help: "CPU usage percentage of the device",
		},
		[]string{"device_id", "ip", "hostname", "vendor"},
	)

	DeviceMemory = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nms_device_memory_usage_percent",
			Help: "Memory usage percentage of the device",
		},
		[]string{"device_id", "ip", "hostname", "vendor"},
	)

	DevicePingLatency = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nms_device_ping_latency_ms",
			Help: "ICMP Ping latency in milliseconds",
		},
		[]string{"device_id", "ip", "hostname", "vendor"},
	)

	DevicePacketLoss = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nms_device_packet_loss_percent",
			Help: "ICMP packet loss percentage",
		},
		[]string{"device_id", "ip", "hostname", "vendor"},
	)

	DeviceUptime = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nms_device_uptime_seconds",
			Help: "Device uptime in seconds",
		},
		[]string{"device_id", "ip", "hostname", "vendor"},
	)

	DeviceRxRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nms_device_rx_rate_mbps",
			Help: "Total device inbound traffic in Mbps",
		},
		[]string{"device_id", "ip", "hostname", "vendor", "device_type"},
	)

	DeviceTxRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nms_device_tx_rate_mbps",
			Help: "Total device outbound traffic in Mbps",
		},
		[]string{"device_id", "ip", "hostname", "vendor", "device_type"},
	)

	// Interface Level Metrics
	InterfaceInBps = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nms_interface_in_bytes_per_second",
			Help: "Inbound traffic in bytes per second",
		},
		[]string{"device_id", "ip", "interface_name"},
	)

	InterfaceOutBps = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nms_interface_out_bytes_per_second",
			Help: "Outbound traffic in bytes per second",
		},
		[]string{"device_id", "ip", "interface_name"},
	)

	InterfaceStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nms_interface_status",
			Help: "Interface status (1 = up, 0 = down)",
		},
		[]string{"device_id", "ip", "interface_name"},
	)

	// Worker/Internal Metrics
	WorkerPollDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nms_worker_poll_duration_seconds",
			Help:    "Time taken to poll a device",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"}, // success or failure
	)

	WorkerActiveQueue = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "nms_worker_active_queue_size",
			Help: "Number of devices currently in the priority queue",
		},
	)
)
