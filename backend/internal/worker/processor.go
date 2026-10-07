package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/cache"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

type Processor struct {
	registry     CollectorRegistry
	deviceRepo   DeviceRepository
	metricRepo   MetricsRepository
	incidentRepo IncidentRepository
	logger       Logger
	workerCount  int

	// For thresholds
	ruleCache       cache.ThresholdRuleCache
	mu              sync.Mutex
	triggerStrikes  map[string]int
	recoveryStrikes map[string]int
	incidentFired   map[string]bool
}

func NewProcessor(
	registry CollectorRegistry,
	deviceRepo DeviceRepository,
	metricRepo MetricsRepository,
	incidentRepo IncidentRepository,
	logger Logger,
	ruleCache cache.ThresholdRuleCache,
) *Processor {

	return &Processor{
		registry:        registry,
		deviceRepo:      deviceRepo,
		metricRepo:      metricRepo,
		incidentRepo:    incidentRepo,
		logger:          logger,
		ruleCache:       ruleCache,
		workerCount:     10,
		triggerStrikes:  make(map[string]int),
		recoveryStrikes: make(map[string]int),
		incidentFired:   make(map[string]bool),
	}
}

func (p *Processor) validateMetrics(metrics *models.TelemetrySnapshot) error {
	if metrics == nil {
		return errors.New("metrics cannot be nil")
	}

	if metrics.CPUUsage < 0 || metrics.CPUUsage > 100 {
		p.logger.Warn("anomalous cpu usage", map[string]interface{}{
			"cpu_usage": metrics.CPUUsage,
		})
	}

	if metrics.MemoryUsage < 0 || metrics.MemoryUsage > 100 {
		p.logger.Warn("anomalous memory usage", map[string]interface{}{
			"memory_usage": metrics.MemoryUsage,
		})
	}

	if metrics.Status != "up" && metrics.Status != "down" && metrics.Status != "degraded" {
		p.logger.Warn("unexpected device status", map[string]interface{}{
			"status": metrics.Status,
		})
	}

	return nil
}

func (p *Processor) ProcessMetrics(ctx context.Context, device models.Device, metrics *models.TelemetrySnapshot) error {
	if metrics == nil {
		return errors.New("metrics cannot be nil")
	}

	if err := p.validateMetrics(metrics); err != nil {
		return err
	}

	// 1. Update status in devices table (Debounce/Optimization)
	// Only update DB if the status was not already 'up' to reduce write amplification.
	needsStatusUpdate := true
	if existingCache := cache.GetMetricsCache().GetDevice(metrics.DeviceID); existingCache != nil && existingCache.Status == "up" {
		needsStatusUpdate = false
	}

	if needsStatusUpdate {
		if err := p.deviceRepo.UpdateDeviceStatus(ctx, metrics.DeviceID, "up", "up", "up"); err != nil {
			p.logger.Error("failed to update device status", map[string]interface{}{
				"device_id": metrics.DeviceID,
				"error":     err,
			})
		}
	}

	// 2. Evaluate thresholds and auto-recovery for offline incidents
	p.evaluateThresholds(ctx, device, metrics)

	if err := p.metricRepo.InsertDeviceMetric(
		ctx,
		&models.DeviceMetric{
			DeviceID:           metrics.DeviceID,
			CPUUsage:           metrics.CPUUsage,
			MemoryUsage:        metrics.MemoryUsage,
			MemoryTotal:        metrics.MemoryTotal,
			MemoryUsed:         metrics.MemoryUsed,
			Uptime:             metrics.Uptime,
			Status:             metrics.Status,
			ReachabilityStatus: metrics.ReachabilityStatus,
			SNMPStatus:         metrics.SNMPStatus,
			SignalStrength:     metrics.SignalStrength,
			CCQ:                metrics.CCQ,
			TxRate:             metrics.TxRate,
			RxRate:             metrics.RxRate,
			Model:              metrics.Model,
			Temperature:        metrics.Temperature,
			Voltage:            metrics.Voltage,
			CollectedAt:        metrics.CollectedAt,
			CreatedAt:          time.Now(),
		},
	); err != nil {
		return err
	}

	if len(metrics.Interfaces) > 0 {
		ifaceMetrics := make([]*models.InterfaceMetric, 0, len(metrics.Interfaces))
		for _, iface := range metrics.Interfaces {
			ifaceMetrics = append(ifaceMetrics, &models.InterfaceMetric{
				DeviceID:        metrics.DeviceID,
				InterfaceIndex:  iface.InterfaceIndex,
				InterfaceName:   iface.InterfaceName,
				InterfaceAlias:  iface.InterfaceAlias,
				InterfaceStatus: iface.Status,
				InOctets:        iface.InOctets,
				OutOctets:       iface.OutOctets,
				InErrors:        iface.InErrors,
				OutErrors:       iface.OutErrors,
				InDiscards:      iface.InDiscards,
				OutDiscards:     iface.OutDiscards,
				InterfaceSpeed:  iface.Speed,
				RxMbps:          iface.RxMbps,
				TxMbps:          iface.TxMbps,
				CollectedAt:     iface.CollectedAt,
				CreatedAt:       time.Now(),
			})
		}

		if err := p.metricRepo.BatchInsertInterfaceMetrics(ctx, ifaceMetrics); err != nil {
			return err
		}
	}

	return nil
}

// ProcessOfflineEvent handles threshold logic for completely offline devices without writing dummy metrics to TSDB.
func (p *Processor) ProcessOfflineEvent(ctx context.Context, device models.Device, overallStatus string) {
	// The incoming ctx is often already timed out/exhausted by the failed SNMP/Ping collector.
	// We MUST use a fresh context for database operations to ensure incidents can actually be created.
	bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dummyMetrics := &models.TelemetrySnapshot{
		DeviceID: device.ID,
		Status:   overallStatus,
	}
	p.evaluateThresholds(bgCtx, device, dummyMetrics)
}

func (p *Processor) evaluateThresholds(ctx context.Context, device models.Device, metrics *models.TelemetrySnapshot) {
	deviceID := device.ID
	// P0-13: Database Query Explosion Fix
	// Load all active incidents ONCE instead of query per interface/threshold
	activeIncidents, err := p.incidentRepo.GetActiveIncidentsByDevice(ctx, deviceID)
	if err != nil {
		p.logger.Error("Failed to fetch active incidents", map[string]interface{}{"error": err, "device_id": deviceID})
		activeIncidents = make(map[string]int64)
	}

	// Helper function for incidents
	handleIncident := func(condition bool, incType string, titleDown string, alertDown string, severity string, reqTrigger int, reqRecovery int) {
		incKey := fmt.Sprintf("%d_%s", deviceID, incType)
		_, exists := activeIncidents[incType]

		p.mu.Lock()
		defer p.mu.Unlock()

		if condition {
			p.recoveryStrikes[incKey] = 0
			p.triggerStrikes[incKey]++

			if !exists && p.triggerStrikes[incKey] >= reqTrigger {
				if !p.incidentFired[incKey] {
					incID, _ := p.incidentRepo.CreateIncident(ctx, deviceID, incType, titleDown)
					if incID > 0 {
						p.incidentRepo.CreateAlert(ctx, incID, severity, alertDown)
						activeIncidents[incType] = incID
					}
					p.incidentFired[incKey] = true
				}
			} else if exists {
				// Sync state if already exists (e.g. agent restart)
				p.incidentFired[incKey] = true
			}
		} else {
			p.triggerStrikes[incKey] = 0

			if exists {
				p.recoveryStrikes[incKey]++
				if p.recoveryStrikes[incKey] >= reqRecovery {
					p.incidentRepo.ResolveIncident(ctx, deviceID, incType)
					p.incidentRepo.CreateAlert(ctx, activeIncidents[incType], "success", fmt.Sprintf("RECOVERED: %s is back to normal", titleDown))
					delete(activeIncidents, incType)
					p.recoveryStrikes[incKey] = 0
					p.incidentFired[incKey] = false
				}
			} else if p.incidentFired[incKey] {
				// Manually resolved by user. We still need recovery strikes to reset the state.
				p.recoveryStrikes[incKey]++
				if p.recoveryStrikes[incKey] >= reqRecovery {
					p.recoveryStrikes[incKey] = 0
					p.incidentFired[incKey] = false
				}
			}
		}
	}

	// 1. Offline Recovery Check & Topology Awareness
	isDeviceUp := metrics.Status == "up" || metrics.Status == "degraded"
	isParentOffline := false
	if device.ParentIP != "" {
		importCache := cache.GetMetricsCache()
		if parentMetrics := importCache.GetDeviceByIP(device.ParentIP); parentMetrics != nil {
			if parentMetrics.Status == "down" || parentMetrics.Status == "offline" || parentMetrics.Status == "timeout" {
				isParentOffline = true
			}
		}
	}

	titleOffline := fmt.Sprintf("Device %s (%s) is offline", device.Name, device.IPAddress)
	alertOffline := fmt.Sprintf("Device %s (%s) disconnected (Offline)", device.Name, device.IPAddress)

	offlineCondition := !isDeviceUp && !isParentOffline
	handleIncident(offlineCondition, "offline", titleOffline, alertOffline, "danger", 2, 3)

	// 1.5 SNMP Degraded Check
	// If device is offline, it supersedes SNMP Degraded, so we force condition=false for degraded when offline.
	isDegraded := metrics.Status == "degraded" && isDeviceUp
	handleIncident(isDegraded, "snmp_degraded", fmt.Sprintf("Device %s (%s) is reachable via ping but SNMP failed (Degraded)", device.Name, device.IPAddress), fmt.Sprintf("SNMP connection to device %s (%s) disrupted (Degraded)", device.Name, device.IPAddress), "warning", 2, 2)

	rules := p.ruleCache.GetRulesForDevice(deviceID)

	// 2. CPU Usage
	if metrics.HasCPU {
		cpuRule, ok := rules["cpu"]
		if !ok {
			cpuRule = models.ThresholdRule{ThresholdValue: 80.0, IsActive: true}
		}
		if cpuRule.IsActive {
			handleIncident(
				metrics.CPUUsage >= cpuRule.ThresholdValue,
				"high_cpu",
				fmt.Sprintf("High CPU usage (%.1f%%) on %s", metrics.CPUUsage, device.Name),
				fmt.Sprintf("High CPU usage detected: %.1f%% on %s (%s)", metrics.CPUUsage, device.Name, device.IPAddress),
				"warning",
				2, 3, // 2 strikes to trigger, 3 to recover
			)
		}
	}

	// 3. Memory Usage
	if metrics.HasMemory {
		ramRule, ok := rules["memory"]
		if !ok {
			ramRule = models.ThresholdRule{ThresholdValue: 85.0, IsActive: true}
		}
		if ramRule.IsActive {
			handleIncident(
				metrics.MemoryUsage >= ramRule.ThresholdValue,
				"high_ram",
				fmt.Sprintf("High memory usage (%.1f%%) on %s", metrics.MemoryUsage, device.Name),
				fmt.Sprintf("High memory usage detected: %.1f%% on %s (%s)", metrics.MemoryUsage, device.Name, device.IPAddress),
				"warning",
				2, 3,
			)
		}
	}

	// 4. Ping Latency
	latRule, ok := rules["latency"]
	if !ok {
		latRule = models.ThresholdRule{ThresholdValue: 80.0, StrikeCount: 2, IsActive: true}
	}
	if latRule.IsActive {
		handleIncident(
			metrics.LatencyMs >= int(latRule.ThresholdValue),
			"high_latency",
			fmt.Sprintf("High ping latency (%d ms) on %s", metrics.LatencyMs, device.Name),
			fmt.Sprintf("High ping latency detected: %d ms on %s (%s)", metrics.LatencyMs, device.Name, device.IPAddress),
			"warning",
			latRule.StrikeCount, 3, // Use user-defined strike count for trigger
		)
	}

	// 5. Packet Loss > 10% (Warning)
	handleIncident(
		metrics.PacketLoss >= 10.0,
		"packet_loss",
		fmt.Sprintf("High packet loss (%.1f%%) on %s (%s)", metrics.PacketLoss, device.Name, device.IPAddress),
		fmt.Sprintf("Packet loss detected: %.1f%% on %s (%s)", metrics.PacketLoss, device.Name, device.IPAddress),
		"warning",
		2, 3,
	)

	// 6. Interface down checks (Batch checked in memory)
	for _, iface := range metrics.Interfaces {
		if iface.InterfaceName == "" {
			continue
		}
		incType := "iface_down_" + iface.InterfaceName
		handleIncident(
			iface.Status == "down",
			incType,
			fmt.Sprintf("Interface %s is down on %s", iface.InterfaceName, device.Name),
			fmt.Sprintf("Interface %s disconnected (Down) on %s (%s)", iface.InterfaceName, device.Name, device.IPAddress),
			"warning",
			2, 3,
		)
	}
}
