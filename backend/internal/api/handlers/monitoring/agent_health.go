package monitoring

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/yourusername/viscod/internal/database"
)

type AppMetrics struct {
	NumGoroutine int    `json:"num_goroutine"`
	AllocMB      uint64 `json:"alloc_mb"`
	TotalAllocMB uint64 `json:"total_alloc_mb"`
	SysMB        uint64 `json:"sys_mb"`
	NumGC        uint32 `json:"num_gc"`
}

type DBMetrics struct {
	ActiveConnections int    `json:"active_connections"`
	MaxOpenConns      int    `json:"max_open_conns"`
	Status            string `json:"status"`
}

type AgentHealthResponse struct {
	Status string     `json:"status"`
	App    AppMetrics `json:"app_metrics"`
	DB     DBMetrics  `json:"db_metrics"`
}

func AgentHealthHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// 1. App Metrics
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		appMetrics := AppMetrics{
			NumGoroutine: runtime.NumGoroutine(),
			AllocMB:      memStats.Alloc / 1024 / 1024,
			TotalAllocMB: memStats.TotalAlloc / 1024 / 1024,
			SysMB:        memStats.Sys / 1024 / 1024,
			NumGC:        memStats.NumGC,
		}

		// 2. DB Metrics
		dbMetrics := DBMetrics{
			Status: "disconnected",
		}

		if db != nil && db.DB != nil {
			stats := db.DB.Stats()
			dbMetrics.ActiveConnections = stats.InUse
			dbMetrics.MaxOpenConns = stats.MaxOpenConnections
			if err := db.DB.Ping(); err == nil {
				dbMetrics.Status = "connected"
			} else {
				dbMetrics.Status = "error"
			}
		}

		// Overall Status
		status := "UP"
		if dbMetrics.Status != "connected" {
			status = "DEGRADED"
		}

		response := AgentHealthResponse{
			Status: status,
			App:    appMetrics,
			DB:     dbMetrics,
		}

		_ = json.NewEncoder(w).Encode(response)
	}
}
