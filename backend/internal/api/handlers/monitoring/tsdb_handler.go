package monitoring

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/hafizkrm/Monitoring/backend/internal/database"
	"github.com/hafizkrm/Monitoring/backend/internal/tsdb"
	"golang.org/x/time/rate"
	"github.com/prometheus/prometheus/promql/parser"
)

// TSDBDeviceHistoryHandler returns per-device metrics history from Prometheus.
// Query params: device_id (required), duration (optional, default "30m", e.g. "1h", "6h", "24h")
func TSDBDeviceHistoryHandler(db *database.Database, tsdbURL string) http.HandlerFunc {
	tsdbClient := tsdb.NewPrometheusClient(tsdbURL)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		deviceIDStr := r.URL.Query().Get("device_id")
		if deviceIDStr == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "device_id parameter required"})
			return
		}
		
		devID, err := strconv.Atoi(deviceIDStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid device_id"})
			return
		}

		durationStr := r.URL.Query().Get("duration")
		if durationStr == "" {
			durationStr = "30m"
		}
		dur, err := time.ParseDuration(durationStr)
		if err != nil {
			dur = 30 * time.Minute
		}

		history, err := tsdbClient.FetchDeviceMetricsHistory(r.Context(), deviceIDStr, dur)
		if err != nil || len(history) == 0 {
			if err != nil {
				log.Printf("[TSDB] Device history query failed for device_id %s: %v, falling back to SQL", deviceIDStr, err)
			}
			// Fallback to SQL
			sqlHistory, sqlErr := db.GetMetricsHistoryByDeviceID(r.Context(), devID)
			if sqlErr != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"source": "sql",
				"data":   sqlHistory,
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"source": "tsdb",
			"data":   history,
		})
	}
}

// TSDBQueryHandler proxies a read-only PromQL instant query to Prometheus.
// Query params: query (required) - the PromQL query string.
func TSDBQueryHandler(tsdbURL string, cfg config.PromQLConfig) http.HandlerFunc {
	tsdbClient := tsdb.NewPrometheusClient(tsdbURL)
	
	// Create rate limiter
	limit := rate.Every(cfg.RateLimitWindow / time.Duration(cfg.RateLimitRequests))
	limiter := rate.NewLimiter(limit, cfg.RateLimitRequests)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// P1: Restrict arbitrary PromQL to admins only
		if role, ok := r.Context().Value("role").(string); !ok || role != "admin" {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Arbitrary PromQL is restricted to admins"})
			return
		}

		// P1: Rate Limiting
		if !limiter.Allow() {
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			return
		}

		query := r.URL.Query().Get("query")
		if query == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "query parameter required"})
			return
		}

		// P1: Query Range Limit Validation
		expr, parseErr := parser.NewParser(parser.Options{}).ParseExpr(query)
		if parseErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid PromQL query"})
			return
		}

		var maxRangeExceeded bool
		parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
			if sel, ok := node.(*parser.MatrixSelector); ok {
				if sel.Range > cfg.MaxRange {
					maxRangeExceeded = true
				}
			}
			if sub, ok := node.(*parser.SubqueryExpr); ok {
				if sub.Range > cfg.MaxRange {
					maxRangeExceeded = true
				}
			}
			return nil
		})

		if maxRangeExceeded {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "query exceeds maximum allowed time range"})
			return
		}

		// P1: Strict timeout (5s) to prevent indefinite blocking from heavy queries
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		result, err := tsdbClient.QueryInstant(ctx, query)
		if err != nil {
			log.Printf("[TSDB] Query failed: %v", err)
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "TSDB query failed: " + err.Error()})
			return
		}

		// P1: Result-size Limit
		if result != nil && result.Data.Result != nil {
			if len(result.Data.Result) > cfg.MaxResultSize {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "result size exceeds allowed limit"})
				return
			}
		}

		_ = json.NewEncoder(w).Encode(result)
	}
}

// TSDBStatusHandler returns TSDB (Prometheus) health status
func TSDBStatusHandler(tsdbURL string) http.HandlerFunc {
	tsdbClient := tsdb.NewPrometheusClient(tsdbURL)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		_, err := tsdbClient.QueryInstant(ctx, "up")
		status := "connected"
		if err != nil {
			status = "disconnected"
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   status,
			"endpoint": tsdbClient.BaseURL,
		})
	}
}
