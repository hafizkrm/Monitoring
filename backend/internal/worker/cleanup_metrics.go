package worker

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	cleanupExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nms_cleanup_executions_total",
			Help: "Total number of database cleanup executions",
		},
		[]string{"status"},
	)

	cleanupRowsDeletedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nms_cleanup_rows_deleted_total",
			Help: "Total number of rows deleted during cleanup",
		},
		[]string{"table"},
	)

	cleanupDurationSeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "nms_cleanup_duration_seconds",
			Help:    "Duration of database cleanup in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)
)
