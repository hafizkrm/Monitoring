package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/cache"
	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/hafizkrm/Monitoring/backend/internal/logger"
	"github.com/hafizkrm/Monitoring/backend/internal/tsdb"
	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
)

type Manager struct {
	config config.Config
	logger logger.Logger
	db     DatabaseClient

	snmpClient SNMPClient
	processor  *Processor
	scheduler  *Scheduler

	workers  int
	started  atomic.Bool
	stopChan chan struct{}
	jobQueue chan Job
	wg       sync.WaitGroup
	cancel   context.CancelFunc

	// circuit breaker
	cb         *CircuitBreaker
	lastUptime map[int]int64
	mu         sync.RWMutex

	eventPublisher contracts.EventPublisher
}

func NewManager(
	cfg config.Config,
	db DatabaseClient,
	log logger.Logger,
	snmpClient SNMPClient,
) *Manager {

	workers := cfg.Polling.MaxWorkers

	if workers <= 0 {
		workers = 10
	}

	queueSize := 10000 // Enterprise scale default
	jobQueue := make(chan Job, queueSize)

	processor := NewProcessor(
		snmpClient,
		db,
		db,
		db,
		log,
	)

	scheduler := NewScheduler(
		cfg,
		db,
		log,
		jobQueue,
	)

	return &Manager{
		config:     cfg,
		logger:     log,
		db:         db,
		snmpClient: snmpClient,
		processor:  processor,
		scheduler:  scheduler,
		workers:    workers,
		stopChan:   make(chan struct{}),
		jobQueue:   jobQueue,
		cb:         NewCircuitBreaker(5, 30*time.Second),
		lastUptime: map[int]int64{},
	}
}

func (m *Manager) SetEventPublisher(ep contracts.EventPublisher) {
	m.eventPublisher = ep
}

func (m *Manager) Start(parentCtx context.Context) error {
	if m.started.Swap(true) {
		return fmt.Errorf("manager already running")
	}

	ctx, cancel := context.WithCancel(parentCtx)
	m.cancel = cancel

	// Pre-populate metrics cache so frontend doesn't falsely assume missing devices are offline
	if dbMetrics, err := m.db.GetLatestMetrics(ctx); err == nil {
		for _, dm := range dbMetrics {
			idVal := dm["id"]
			var id int
			if v, ok := idVal.(float64); ok { id = int(v) } else if v, ok := idVal.(int64); ok { id = int(v) } else if v, ok := idVal.(int); ok { id = v }
			
			if id > 0 {
				existing := cache.GetMetricsCache().GetDevice(id)
				if existing == nil {
					name, _ := dm["name"].(string)
					ip, _ := dm["ip_address"].(string)
					devType, _ := dm["device_type"].(string)
					status, _ := dm["status"].(string)
					cpu, _ := dm["cpu_usage"].(float64)
					mem, _ := dm["memory_usage"].(float64)
					tx, _ := dm["tx_rate"].(float64)
					rx, _ := dm["rx_rate"].(float64)
					lat, _ := dm["latency"].(float64)
					
					var uptime int64
					if u, ok := dm["uptime"].(float64); ok { uptime = int64(u) } else if u, ok := dm["uptime"].(int64); ok { uptime = u }

					cache.GetMetricsCache().Update(&cache.LatestDeviceMetrics{
						DeviceID:    id,
						Name:        name,
						IPAddress:   ip,
						DeviceType:  devType,
						Status:      status,
						CPUUsage:    cpu,
						MemoryUsage: mem,
						TxRate:      tx,
						RxRate:      rx,
						Latency:     lat,
						Uptime:      uptime,
						UpdatedAt:   time.Now(),
					})
				}
			}
		}
		m.logger.Info("Pre-populated metrics cache", map[string]interface{}{
			"dbMetricsCount": len(dbMetrics),
			"cacheCount":     len(cache.GetMetricsCache().GetAll()),
		})
	}

	for i := 0; i < m.workers; i++ {
		m.wg.Add(1)

		go m.worker(
			ctx,
			i,
		)
	}

	m.wg.Add(1)

	go func() {
		defer m.wg.Done()

		m.scheduler.Start(ctx)
	}()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.startCleanupTask(ctx)
	}()

	m.logger.Info("manager started", map[string]interface{}{
		"workers": m.workers,
	})

	return nil
}

func (m *Manager) Stop() {
	if !m.started.Load() {
		return
	}

	m.cancel() // cancel derived ctx
	close(m.stopChan)

	m.scheduler.Stop()

	m.wg.Wait()

	m.started.Store(false)

	m.logger.Info("manager stopped", nil)
}

func (m *Manager) worker(
	ctx context.Context,
	workerID int,
) {

	defer m.wg.Done()

	for {

		select {

		case <-ctx.Done():
			return

		case <-m.stopChan:
			return

		case job, ok := <-m.jobQueue:

			if !ok {
				return
			}

			m.processJob(
				ctx,
				job,
				workerID,
			)
		}
	}
}

func (m *Manager) processJob(
	ctx context.Context,
	job Job,
	workerID int,
) {
	device := job.Device
	defer m.scheduler.markDone(device.ID)

	// Backpressure: drop stale jobs to prevent starvation cascading
	if time.Since(job.ScheduledAt) > 90*time.Second {
		m.logger.Warn("job dropped due to queue starvation", map[string]interface{}{
			"device": device.Name,
		})
		return
	}

	// P0-7: Overall poll timeout ~15s (reduced from 45s)
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if m.cb.IsBroken(device.ID) {
		// Even when circuit is broken, try a fast ICMP ping to detect recovery early
		pingCtx, pingCancel := context.WithTimeout(ctx, 6*time.Second)
		defer pingCancel()
		latency, _, loss, pingErr := m.snmpClient.GetNetworkStats(pingCtx, device.IPAddress)
		if pingErr == nil && loss < 100 && latency > 0 {
			// Device is back! Reset CB and update status immediately
			m.cb.Reset(device.ID)
			m.logger.Info("device recovered (circuit reset via ping)", map[string]interface{}{
				"device":  device.Name,
				"latency": latency,
			})
			ctxDb, cancelDb := context.WithTimeout(context.Background(), 5*time.Second)
			_ = m.db.UpdateDeviceStatus(ctxDb, device.ID, "up", "unknown", "degraded")
			_ = m.db.InsertPollingLog(ctxDb, device.ID, "success", "device recovered (ping ok, circuit reset)", latency)
			_ = m.db.InsertActivityLog(ctxDb, nil, "System", "RECOVERY", "Monitoring",
				fmt.Sprintf("[%s] %s - Perangkat kembali online (circuit breaker direset via ping)", device.IPAddress, device.Name),
				"")
			cancelDb()
			// Fall through to normal polling below
		} else {
			m.logger.Warn("device circuit broken", map[string]interface{}{
				"device": device.Name,
			})
			ctxDb, cancelDb := context.WithTimeout(context.Background(), 5*time.Second)
			_ = m.db.UpdateDeviceStatus(ctxDb, device.ID, "down", "down", "down") // P1-18: Ensure status matches circuit breaker
			_ = m.db.InsertPollingLog(ctxDb, device.ID, "error", "device unreachable (circuit broken)", 0)
			_ = m.db.InsertActivityLog(ctxDb, nil, "System", "DOWNTIME", "Monitoring",
				fmt.Sprintf("[%s] %s - Perangkat tidak dapat dijangkau (circuit breaker aktif)", device.IPAddress, device.Name),
				"")
			cancelDb()
			return
		}
	}

	start := time.Now()

	metrics, err := m.snmpClient.CollectDeviceMetrics(
		ctx,
		*device,
	)

	// TSDB: Record worker poll duration
	duration := time.Since(start).Seconds()

	if err != nil {
		tsdb.WorkerPollDuration.WithLabelValues("error").Observe(duration)
		m.cb.RecordFailure(device.ID)
		m.logger.Error("polling failure", map[string]interface{}{"device": device.ID, "error": err})

		// SNMP polling failed. But is it completely DOWN or just SNMP DEGRADED?
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer pingCancel()
		pingLatency, _, pingLoss, pingErr := m.snmpClient.GetNetworkStats(pingCtx, device.IPAddress)
		
		reachability := "down"
		overallStatus := "down"
		
		if pingErr == nil && pingLoss < 100 && pingLatency > 0 {
			reachability = "up"
			overallStatus = "degraded"
		}

		ctxDb, cancelDb := context.WithTimeout(context.Background(), 5*time.Second)
		_ = m.db.UpdateDeviceStatus(ctxDb, device.ID, reachability, "down", overallStatus)
		_ = m.db.InsertPollingLog(ctxDb, device.ID, "error", fmt.Sprintf("polling failed: %v", err), int(time.Since(start).Milliseconds()))
		_ = m.db.InsertActivityLog(ctxDb, nil, "System", "DOWNTIME", "Monitoring",
			fmt.Sprintf("[%s] %s - Polling gagal: %v", device.IPAddress, device.Name, err),
			"")
		cancelDb()

		// Update cache so the frontend WebSocket gets the offline status without erasing Uptime/Bandwidth!
		existing := cache.GetMetricsCache().GetDevice(device.ID)
		isChanged := false
		if existing == nil {
			isChanged = true
			existing = &cache.LatestDeviceMetrics{
				DeviceID:   device.ID,
				Name:       device.Name,
				IPAddress:  device.IPAddress,
				DeviceType: string(device.DeviceType),
			}
		}
		if existing.Status != overallStatus {
			isChanged = true
		}
		existing.Status = overallStatus
		existing.UpdatedAt = time.Now()
		cache.GetMetricsCache().Update(existing)

		if isChanged && m.eventPublisher != nil {
			// Phase 4: Event-driven broadcast for offline status change via EventBus
			m.eventPublisher.Publish(contracts.NewDomainEvent(
				"snmp-worker",
				contracts.DomainEventMetricsUpdated,
				fmt.Sprintf("%v", device.ID),
				*existing,
			))
		}

		return
	}

	// If device was previously broken/failing, log recovery
	prevFailures := m.cb.GetFailures(device.ID)
	if prevFailures >= 3 {
		ctxDb, cancelDb := context.WithTimeout(context.Background(), 5*time.Second)
		_ = m.db.InsertActivityLog(ctxDb, nil, "System", "RECOVERY", "Monitoring",
			fmt.Sprintf("[%s] %s - Perangkat kembali online (recovery)", device.IPAddress, device.Name),
			"")
		cancelDb()
	}
	m.cb.Reset(device.ID)

	m.mu.Lock()
	prevUptime, hasPrevUptime := m.lastUptime[device.ID]
	m.lastUptime[device.ID] = metrics.Uptime
	m.mu.Unlock()

	if hasPrevUptime && prevUptime > 0 && metrics.Uptime > 0 && metrics.Uptime < prevUptime {
		m.logger.Warn("device reboot detected", map[string]interface{}{"device": device.Name})
		ctxDb, cancelDb := context.WithTimeout(context.Background(), 5*time.Second)
		_ = m.db.InsertPollingLog(ctxDb, device.ID, "error", "device rebooted (uptime reset)", 0)
		_ = m.db.InsertActivityLog(ctxDb, nil, "System", "REBOOT", "Monitoring",
			fmt.Sprintf("[%s] %s - Perangkat mengalami reboot (uptime reset)", device.IPAddress, device.Name),
			"")
		cancelDb()
	}

	// processor now saves everything internally
	err = m.processor.ProcessMetrics(
		ctx,
		metrics,
	)

	if err == nil {
		// PHASE 7: Update real-time metrics cache
		newMetrics := &cache.LatestDeviceMetrics{
			DeviceID:    device.ID,
			Name:        device.Name,
			IPAddress:   device.IPAddress,
			DeviceType:  device.DeviceType,
			Status:      metrics.Status,
			CPUUsage:    metrics.CPUUsage,
			MemoryUsage: metrics.MemoryUsage,
			Latency:     float64(metrics.LatencyMs),
			PacketLoss:  metrics.PacketLoss,
			TxRate:      metrics.TxRate,
			RxRate:      metrics.RxRate,
			Jitter:      metrics.Jitter,
			Uptime:      metrics.Uptime,
			CollectedAt: metrics.CollectedAt,
			UpdatedAt:   time.Now(),
			CreatedAt:   time.Now(),
		}

		isChanged := false
		if existing := cache.GetMetricsCache().GetDevice(device.ID); existing != nil {
			if newMetrics.Uptime == 0 && (metrics.Status == "degraded" || metrics.Status == "down") {
				newMetrics.Uptime = existing.Uptime
				newMetrics.CPUUsage = existing.CPUUsage
				newMetrics.MemoryUsage = existing.MemoryUsage
				newMetrics.TxRate = existing.TxRate
				newMetrics.RxRate = existing.RxRate
			}
			// Delta checking: Only broadcast if values meaningfully changed
			diffCPU := existing.CPUUsage - newMetrics.CPUUsage
			if diffCPU < 0 { diffCPU = -diffCPU }
			
			diffMem := existing.MemoryUsage - newMetrics.MemoryUsage
			if diffMem < 0 { diffMem = -diffMem }
			
			diffLat := float64(existing.Latency) - float64(newMetrics.Latency)
			if diffLat < 0 { diffLat = -diffLat }

			if existing.Status != newMetrics.Status ||
				diffCPU > 1.0 ||
				diffMem > 1.0 ||
				diffLat > 2.0 ||
				existing.TxRate != newMetrics.TxRate ||
				existing.RxRate != newMetrics.RxRate {
				isChanged = true
			}
		} else {
			isChanged = true
		}

		cache.GetMetricsCache().Update(newMetrics)

		if isChanged && m.eventPublisher != nil {
			// Phase 4: Event-driven broadcast for metrics change via EventBus
			m.eventPublisher.Publish(contracts.NewDomainEvent(
				"snmp-worker",
				contracts.DomainEventMetricsUpdated,
				fmt.Sprintf("%v", device.ID),
				*newMetrics,
			))
		}

		// PHASE 3 (TSDB): Export metrics to Prometheus
		devIDStr := fmt.Sprintf("%d", device.ID)
		devTypeStr := string(device.DeviceType)
		tsdb.DeviceCPU.WithLabelValues(devIDStr, device.IPAddress, device.Hostname, string(device.Vendor)).Set(metrics.CPUUsage)
		tsdb.DeviceMemory.WithLabelValues(devIDStr, device.IPAddress, device.Hostname, string(device.Vendor)).Set(metrics.MemoryUsage)
		tsdb.DevicePingLatency.WithLabelValues(devIDStr, device.IPAddress, device.Hostname, string(device.Vendor)).Set(float64(metrics.LatencyMs))
		tsdb.DevicePacketLoss.WithLabelValues(devIDStr, device.IPAddress, device.Hostname, string(device.Vendor)).Set(metrics.PacketLoss)
		tsdb.DeviceUptime.WithLabelValues(devIDStr, device.IPAddress, device.Hostname, string(device.Vendor)).Set(float64(metrics.Uptime))
		tsdb.DeviceRxRate.WithLabelValues(devIDStr, device.IPAddress, device.Hostname, string(device.Vendor), devTypeStr).Set(metrics.RxRate)
		tsdb.DeviceTxRate.WithLabelValues(devIDStr, device.IPAddress, device.Hostname, string(device.Vendor), devTypeStr).Set(metrics.TxRate)
		tsdb.WorkerPollDuration.WithLabelValues("success").Observe(duration)
		
		for _, iface := range metrics.Interfaces {
			ifaceStatus := 0.0
			if iface.Status == "up" {
				ifaceStatus = 1.0
			}
			tsdb.InterfaceStatus.WithLabelValues(devIDStr, device.IPAddress, iface.InterfaceName).Set(ifaceStatus)
			tsdb.InterfaceInBps.WithLabelValues(devIDStr, device.IPAddress, iface.InterfaceName).Set(iface.RxMbps)
			tsdb.InterfaceOutBps.WithLabelValues(devIDStr, device.IPAddress, iface.InterfaceName).Set(iface.TxMbps)
		}
	}

	if err != nil {

		m.logger.Error("collect failed", map[string]interface{}{
			"device": device.Name,
			"error":  err,
		})

		// PHASE 3: Track failed polls in TSDB too
		tsdb.WorkerPollDuration.WithLabelValues("failure").Observe(duration)

		return
	}

	if err := m.db.UpdateDeviceLastPolledAt(ctx, device.ID); err != nil {
		m.logger.Error("update last polled failed", map[string]interface{}{
			"device": device.Name,
			"error":  err,
		})
	}

	if err := m.db.InsertPollingLog(ctx, device.ID, "success", "polling succeeded", metrics.LatencyMs); err != nil {
		m.logger.Error("insert polling log failed", map[string]interface{}{
			"device": device.Name,
			"error":  err,
		})
	}

	m.logger.Info("job completed", map[string]interface{}{
		"device":   device.Name,
		"worker":   workerID,
		"latency":  metrics.LatencyMs,
		"duration": time.Since(start).String(),
	})
}

func (m *Manager) startCleanupTask(
	ctx context.Context,
) {

	ticker := time.NewTicker(
		m.config.GetCleanupInterval(),
	)

	defer ticker.Stop()

	for {

		select {

		case <-ctx.Done():
			return

		case <-ticker.C:

			m.logger.Info(
				"starting cleanup task",
				nil,
			)

			// Perform actual cleanup - remove data older than retention days
			if err := m.db.CleanupOldData(ctx, m.config.Polling.MetricsRetentionDays, m.config.Polling.LogsRetentionDays); err != nil {
				m.logger.Error(
					"cleanup task failed",
					map[string]interface{}{
						"error": err,
					},
				)
			} else {
				m.logger.Info(
					"cleanup task completed",
					nil,
				)
			}
		}
	}
}

type WorkerMetrics struct {
	QueueLength    int `json:"queue_length"`
	Workers        int `json:"workers"`
	BrokenCircuits int `json:"broken_circuits"`
}

func (m *Manager) GetMetrics() WorkerMetrics {
	return WorkerMetrics{
		QueueLength:    len(m.jobQueue),
		Workers:        m.workers,
		BrokenCircuits: m.cb.GetBrokenCount(),
	}
}

func WorkerMetricsHandler(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := m.GetMetrics()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	}
}
