package monitoring

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/cache"
	"github.com/hafizkrm/Monitoring/backend/internal/database"
	"github.com/hafizkrm/Monitoring/backend/internal/tsdb"
)

var (
	topInterfacesCache      []map[string]interface{}
	topInterfacesCacheTime  time.Time
	topInterfacesCacheMutex sync.Mutex
)

func MetricsHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		// Use a fresh context with generous timeout, independent of request context
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Use memory cache for real-time separation (Phase 6 & 7)
		metrics := cache.GetMetricsCache().GetAll()

		// Fallback to database if cache is empty (e.g. just started)
		if len(metrics) == 0 {
			dbMetrics, err := db.GetLatestMetrics(ctx)
			if err != nil {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
				return
			}
			metrics = dbMetrics
		}

		if metrics == nil {
			metrics = []map[string]interface{}{}
		}

		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			// Ignore errors caused by client disconnecting early
			errStr := err.Error()
			if !strings.Contains(errStr, "wsasend") &&
				!strings.Contains(errStr, "broken pipe") &&
				!strings.Contains(errStr, "connection was aborted") {
				log.Printf("MetricsHandler encoding error: %v", err)
			}
		}
	}
}

func MetricsHistoryHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		ip := r.URL.Query().Get("ip")
		if ip == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "ip parameter required"})
			return
		}

		history, err := db.GetMetricsHistoryByIP(r.Context(), ip)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
			return
		}

		if err := json.NewEncoder(w).Encode(history); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func ReliabilityHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		rel, err := db.GetUptimeReliability(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
			return
		}
		_ = json.NewEncoder(w).Encode(rel)
	}
}
func TopInterfacesHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		topInterfacesCacheMutex.Lock()
		if time.Since(topInterfacesCacheTime) < 15*time.Second && topInterfacesCache != nil {
			cachedResults := topInterfacesCache
			topInterfacesCacheMutex.Unlock()
			_ = json.NewEncoder(w).Encode(cachedResults)
			return
		}
		topInterfacesCacheMutex.Unlock()

		results, err := db.GetTopInterfacesBandwidth(r.Context())
		if err != nil {
			log.Printf("Failed to fetch top interfaces bandwidth: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
			return
		}

		topInterfacesCacheMutex.Lock()
		topInterfacesCache = results
		topInterfacesCacheTime = time.Now()
		topInterfacesCacheMutex.Unlock()

		_ = json.NewEncoder(w).Encode(results)
	}
}

func BandwidthHistoryHandler(db *database.Database, tsdbURL string) http.HandlerFunc {
	tsdbClient := tsdb.NewPrometheusClient(tsdbURL)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		durationStr := r.URL.Query().Get("duration")
		if durationStr == "" {
			durationStr = "30m"
		}

		// Phase 3: Attempt to fetch from TSDB (Prometheus)
		history, err := tsdbClient.FetchGlobalBandwidthHistory(r.Context(), durationStr)
		if err != nil || len(history) == 0 {
			if err != nil {
				log.Printf("TSDB fetch failed, falling back to SQL: %v", err)
			}
			history, err = db.GetGlobalBandwidthHistory(r.Context(), durationStr)
			if err != nil {
				log.Printf("Failed to fetch global bandwidth history: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
				return
			}
		}
		
		_ = json.NewEncoder(w).Encode(history)
	}
}
