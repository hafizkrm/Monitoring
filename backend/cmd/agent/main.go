package main

import (
	"compress/gzip"
	"context"

	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/hafizkrm/Monitoring/backend/internal/database"
	"github.com/hafizkrm/Monitoring/backend/internal/logger"
	"github.com/hafizkrm/Monitoring/backend/internal/snmp"
	"github.com/hafizkrm/Monitoring/backend/internal/worker"
	// New NMS Architecture Packages (Kept Websocket)
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
	nms_websocket "github.com/hafizkrm/Monitoring/backend/internal/transport/websocket"

	"github.com/hafizkrm/Monitoring/backend/internal/api/handlers/devices"
	"github.com/hafizkrm/Monitoring/backend/internal/api/handlers/incidents"
	"github.com/hafizkrm/Monitoring/backend/internal/api/handlers/logs"
	"github.com/hafizkrm/Monitoring/backend/internal/api/handlers/monitoring"
	"github.com/hafizkrm/Monitoring/backend/internal/api/handlers/reports"
	"github.com/hafizkrm/Monitoring/backend/internal/api/handlers/settings"
	"github.com/hafizkrm/Monitoring/backend/internal/api/handlers/users"
	nms_middleware "github.com/hafizkrm/Monitoring/backend/internal/api/middleware"
	"golang.org/x/time/rate"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	// Load .env if it exists
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found, relying on environment variables or config.yaml")
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.InitLogger(cfg.Logger)
	defer log.Sync()

	log.Info("Starting NMS Agent", map[string]interface{}{
		"version": "1.0.0",
		"config":  *configPath,
	})

	// Set encryption key environment variable if not already set, for db initialization
	if cfg.App.GetEncryptionKey() != "" {
		os.Setenv("ENCRYPTION_KEY", cfg.App.GetEncryptionKey())
	}

	// Initialize database
	db, err := database.NewDatabase(cfg.Database, log)
	if err != nil {
		log.Error("Failed to initialize database", map[string]interface{}{
			"error": err,
		})
		os.Exit(1)
	}
	defer db.Close()

	// Run database migrations to ensure all core tables exist (P0 - Synchronous & Fatal on failure)
	if err := database.Migrate(db.DB); err != nil {
		log.Error("Database migration failed", map[string]interface{}{"error": err})
		os.Exit(1)
	}

	// Initialize repositories / sync after migration is successful
	if err := db.SyncOfflineAlerts(context.Background()); err != nil {
		log.Error("Failed initial SyncOfflineAlerts", map[string]interface{}{"error": err})
	}

	log.Info("Connected to database", map[string]interface{}{
		"database": cfg.Database.Database,
		"host":     cfg.Database.Host,
		"port":     cfg.Database.Port,
	})

	// Initialize SNMP client
	snmpClient := snmp.NewClient(cfg, log)

	// Initialize worker manager
	manager := worker.NewManager(*cfg, db, log, snmpClient)

	// Initialize Rate Limiter globally
	limiter := nms_middleware.NewIPRateLimiter(rate.Limit(100), 200) // 100 req/s, burst 200

	// Initialize Chi Router
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	corsOriginsStr := os.Getenv("CORS_ORIGINS")
	corsOrigins := []string{"http://localhost", "http://localhost:8080", "http://localhost:5173", "http://127.0.0.1:5173", "http://127.0.0.1:8080"}
	if corsOriginsStr != "" {
		corsOrigins = strings.Split(corsOriginsStr, ",")
		for i := range corsOrigins {
			corsOrigins[i] = strings.TrimSpace(corsOrigins[i])
		}
	}

	// Secure CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// P2-48: PPROF & Goroutine Leaks Profiling
	// Expose profiling data to identify goroutine leaks under load. Protected by Admin role.
	r.Group(func(r chi.Router) {
		r.Use(nms_middleware.AuthMiddleware(cfg))
		r.Use(nms_middleware.RequireRole("admin"))
		r.Mount("/debug", middleware.Profiler())
	})

	// ==========================================
	// WebSocket Real-time Hub
	// ==========================================
	wsHub := nms_websocket.NewHub()
	go wsHub.Run()

	// PHASE 1: WebSocket Integration Bridge (Event-Driven)
	manager.SetWSBroadcast(func(msg contracts.WSEventEnvelope) {
		wsHub.BroadcastMessage(msg)
	})

	r.With(nms_middleware.RateLimitMiddleware(limiter), nms_middleware.AuthMiddleware(cfg)).Get("/ws", func(w http.ResponseWriter, req *http.Request) {
		nms_websocket.ServeWS(wsHub, w, req)
	})
	log.Info("Sprint 1 Core Platform Initialized", nil)
	// ==========================================

	// Prometheus TSDB Exporter Endpoint
	r.Handle("/metrics", promhttp.Handler())

	// Rate Limited API Group
	r.Group(func(r chi.Router) {
		r.Use(nms_middleware.RateLimitMiddleware(limiter))

		// Public route
		usersHandler := users.NewUsersHandler(db.DB, cfg.App.GetJWTSecret())
		r.Post("/api/login", usersHandler.Login)
		r.Post("/api/logout", usersHandler.Logout)
		r.Post("/api/refresh", usersHandler.Refresh)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(nms_middleware.AuthMiddleware(cfg))

			r.Get("/api/metrics", monitoring.MetricsHandler(db))
			r.Get("/api/agent/health", monitoring.AgentHealthHandler(db))
			r.Get("/api/metrics/history", monitoring.MetricsHistoryHandler(db))
			r.Get("/api/interfaces", devices.InterfacesHandler(db))

			r.Get("/api/v2/logs", logs.LogsHandlerV2(db))
			r.Get("/api/v2/activity-logs", logs.ActivityLogsHandlerV2(db))
			r.With(nms_middleware.RequireRole("admin")).Post("/api/v2/activity-logs", logs.PostActivityLogHandlerV2(db))

			r.Get("/api/inventory/stats", devices.InventoryStatsHandler(db))
			r.Get("/api/inventory", devices.InventoryHandler(db))
			r.Get("/api/reliability", monitoring.ReliabilityHandler(db))
			r.Get("/api/top-interfaces", monitoring.TopInterfacesHandler(db))
			r.Get("/api/bandwidth/history", monitoring.BandwidthHistoryHandler(db, cfg.App.GetTSDBUrl()))

			// Phase 3: TSDB (Prometheus) Endpoints
			r.Get("/api/tsdb/device-history", monitoring.TSDBDeviceHistoryHandler(db, cfg.App.GetTSDBUrl()))
			r.Get("/api/tsdb/query", monitoring.TSDBQueryHandler(cfg.App.GetTSDBUrl(), cfg.PromQL))
			r.Get("/api/tsdb/status", monitoring.TSDBStatusHandler(cfg.App.GetTSDBUrl()))

			r.With(nms_middleware.RequireRole("admin")).Post("/api/devices", devices.AddDeviceHandler(db))
			r.With(nms_middleware.RequireRole("admin")).Post("/api/devices/delete", devices.DeleteDeviceHandler(db))
			r.With(nms_middleware.RequireRole("admin")).Post("/api/devices/update", devices.UpdateDeviceHandler(db))
			r.With(nms_middleware.RequireRole("admin")).Get("/api/worker/metrics", worker.WorkerMetricsHandler(manager))

			r.With(nms_middleware.RequireRole("admin")).Get("/api/tools/ping", devices.PingHandler)
			r.With(nms_middleware.RequireRole("admin")).Get("/api/tools/trace", devices.TraceHandler)
			r.Get("/api/reports", nms_middleware.GzipMiddleware(reports.ReportsHandler(db)))

			r.Get("/api/incidents", incidents.GetActiveIncidents(db))
			r.Get("/api/alerts", incidents.GetAlerts(db))
			r.With(nms_middleware.RequireRole("admin")).Post("/api/alerts/read", incidents.MarkAlertRead(db))
			r.With(nms_middleware.RequireRole("admin")).Post("/api/alerts/resolve-by-ip", incidents.ResolveAlertsByIP(db))

			r.Get("/api/vpn/users", monitoring.VPNUsersHandler(cfg))

			r.With(nms_middleware.RequireRole("admin")).Get("/api/users", usersHandler.GetAllUsers)
			r.With(nms_middleware.RequireRole("admin")).Post("/api/users", usersHandler.AddUser)
			r.With(nms_middleware.RequireRole("admin")).Put("/api/users", usersHandler.UpdateUser)
			r.With(nms_middleware.RequireRole("admin")).Delete("/api/users", usersHandler.DeleteUser)

			settingsHandler := settings.NewSettingsHandler(db.DB)
			r.With(nms_middleware.RequireRole("admin")).Get("/api/settings", settingsHandler.GetAllSettings)
			r.With(nms_middleware.RequireRole("admin")).Post("/api/settings", settingsHandler.UpsertSetting)
			r.With(nms_middleware.RequireRole("admin")).Put("/api/settings", settingsHandler.UpsertSetting)
		})
	})

	// Handle favicon to prevent 404
	r.Get("/favicon.ico", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Ensure that it serves frontend correctly
	// Make frontend paths robust whether run from root or backend dir
	frontendDistPath := "frontend/dist"
	if _, err := os.Stat(frontendDistPath); os.IsNotExist(err) {
		frontendDistPath = "../frontend/dist"
	}

	frontendPublicPath := "frontend/public"
	if _, err := os.Stat(frontendPublicPath); os.IsNotExist(err) {
		frontendPublicPath = "../frontend/public"
	}

	frontendSrcPath := "frontend/src"
	if _, err := os.Stat(frontendSrcPath); os.IsNotExist(err) {
		frontendSrcPath = "../frontend/src"
	}
	r.Handle("/src/*", http.StripPrefix("/src/", staticHandler(http.FileServer(http.Dir(frontendSrcPath)))))

	if _, err := os.Stat(frontendDistPath); err == nil {
		r.Handle("/*", staticHandler(http.FileServer(http.Dir(frontendDistPath))))
	} else {
		r.Handle("/*", staticHandler(http.FileServer(http.Dir(frontendPublicPath))))
	}

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Start HTTP server before manager so API is available even if polling encounters issues
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		log.Info("Starting HTTP dashboard server", map[string]interface{}{"address": addr})
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Dashboard HTTP server failed", map[string]interface{}{"error": err})
		}
	}()

	if err := manager.Start(ctx); err != nil {
		log.Error("Failed to start manager (retrying in background)", map[string]interface{}{
			"error": err,
		})
	}

	// Start VPN Logger polling
	worker.StartVPNLogger(ctx, cfg, db, log)

	log.Info("Agent started successfully", map[string]interface{}{
		"workers": cfg.Polling.MaxWorkers,
	})

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan

	log.Info("Received shutdown signal, stopping agent...", nil)
	manager.Stop()
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Failed to shutdown HTTP server cleanly", map[string]interface{}{"error": err})
	}

	log.Info("Agent stopped", nil)
}

type gzipResponseWriter struct {
	http.ResponseWriter
	Writer     *gzip.Writer
	statusCode int
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	if code >= 400 {
		w.Header().Del("Content-Encoding")
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if w.statusCode >= 400 {
		return w.ResponseWriter.Write(b)
	}
	return w.Writer.Write(b)
}

func staticHandler(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".html") || r.URL.Path == "/" || r.URL.Path == "/login" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		} else {
			// Set cache control for 1 year for assets like CSS/JS
			w.Header().Set("Cache-Control", "public, max-age=31536000")
		}

		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			h.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		// Optional: also tell proxies that the response might vary by encoding
		w.Header().Add("Vary", "Accept-Encoding")

		gz := gzip.NewWriter(w)
		defer gz.Close()

		gzw := &gzipResponseWriter{ResponseWriter: w, Writer: gz}
		h.ServeHTTP(gzw, r)
	}
}

