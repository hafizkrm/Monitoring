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
	ruleCache      cache.ThresholdRuleCache
	mu             sync.Mutex
	latencyStrikes map[int]int
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
		registry:       registry,
		deviceRepo:     deviceRepo,
		metricRepo:     metricRepo,
		incidentRepo:   incidentRepo,
		logger:         logger,
		ruleCache:      ruleCache,
		workerCount:    10,
		latencyStrikes: make(map[int]int),
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

func (p *Processor) ProcessMetrics(ctx context.Context, metrics *models.TelemetrySnapshot) error {
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
	p.evaluateThresholds(ctx, metrics.DeviceID, metrics)

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

func (p *Processor) evaluateThresholds(ctx context.Context, deviceID int, metrics *models.TelemetrySnapshot) {
	// P0-13: Database Query Explosion Fix
	// Load all active incidents ONCE instead of query per interface/threshold
	activeIncidents, err := p.incidentRepo.GetActiveIncidentsByDevice(ctx, deviceID)
	if err != nil {
		p.logger.Error("Failed to fetch active incidents", map[string]interface{}{"error": err, "device_id": deviceID})
		activeIncidents = make(map[string]int64)
	}

	// Helper function for incidents
	handleIncident := func(condition bool, incType string, titleDown string, alertDown string) {
		_, exists := activeIncidents[incType]
		if condition {
			// Trigger incident if not exists
			if !exists {
				incID, _ := p.incidentRepo.CreateIncident(ctx, deviceID, incType, titleDown)
				if incID > 0 {
					p.incidentRepo.CreateAlert(ctx, incID, "warning", alertDown)
				}
			}
		} else {
			// Resolve incident if exists
			if exists {
				p.incidentRepo.ResolveIncident(ctx, deviceID, incType)
			}
		}
	}

	// 1. Offline Recovery Check
	isDeviceUp := metrics.Status == "up" || metrics.Status == "degraded"
	_, isOffline := activeIncidents["offline"]

	if isDeviceUp {
		if isOffline {
			p.incidentRepo.ResolveIncident(ctx, deviceID, "offline")
		}
	} else {
		if !isOffline {
			incID, _ := p.incidentRepo.CreateIncident(ctx, deviceID, "offline", "Perangkat terdeteksi terputus (Offline)")
			if incID > 0 {
				p.incidentRepo.CreateAlert(ctx, incID, "danger", "Perangkat terputus (Offline)")
			}
		}
		// Also implicitly resolve SNMP degraded since offline supersedes it
		if _, isDegraded := activeIncidents["snmp_degraded"]; isDegraded {
			p.incidentRepo.ResolveIncident(ctx, deviceID, "snmp_degraded")
		}
	}

	// 1.5 SNMP Degraded Check
	switch metrics.Status {
	case "degraded":
		if _, isDegraded := activeIncidents["snmp_degraded"]; !isDegraded {
			incID, _ := p.incidentRepo.CreateIncident(ctx, deviceID, "snmp_degraded", "Perangkat dapat di-ping tetapi SNMP gagal (Degraded)")
			if incID > 0 {
				p.incidentRepo.CreateAlert(ctx, incID, "warning", "Koneksi SNMP ke perangkat terganggu (Degraded)")
			}
		}
	case "up":
		if _, isDegraded := activeIncidents["snmp_degraded"]; isDegraded {
			p.incidentRepo.ResolveIncident(ctx, deviceID, "snmp_degraded")
		}
	}

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
				fmt.Sprintf("Penggunaan CPU tinggi (%.1f%%)", metrics.CPUUsage),
				fmt.Sprintf("Terdeteksi penggunaan CPU tinggi: %.1f%%", metrics.CPUUsage),
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
				fmt.Sprintf("Penggunaan Memori tinggi (%.1f%%)", metrics.MemoryUsage),
				fmt.Sprintf("Terdeteksi penggunaan Memori tinggi: %.1f%%", metrics.MemoryUsage),
			)
		}
	}

	// 4. Ping Latency
	latRule, ok := rules["latency"]
	if !ok {
		latRule = models.ThresholdRule{ThresholdValue: 80.0, StrikeCount: 2, IsActive: true}
	}
	if latRule.IsActive {
		var strikeCount int
		p.mu.Lock()
		if metrics.LatencyMs >= int(latRule.ThresholdValue) {
			p.latencyStrikes[deviceID]++
			strikeCount = p.latencyStrikes[deviceID]
		} else {
			p.latencyStrikes[deviceID] = 0
			strikeCount = 0
		}
		p.mu.Unlock()

		handleIncident(
			metrics.LatencyMs >= int(latRule.ThresholdValue) && strikeCount >= latRule.StrikeCount,
			"high_latency",
			fmt.Sprintf("Latensi ping tinggi (%d ms)", metrics.LatencyMs),
			fmt.Sprintf("Terdeteksi latensi ping tinggi: %d ms", metrics.LatencyMs),
		)
	}

	// 5. Packet Loss > 10% (Warning)
	// (Keeping packet_loss hardcoded as it wasn't requested to be moved to dynamic rules)
	handleIncident(
		metrics.PacketLoss >= 10.0,
		"packet_loss",
		fmt.Sprintf("Kehilangan paket tinggi (%.1f%%)", metrics.PacketLoss),
		fmt.Sprintf("Terdeteksi kehilangan paket: %.1f%%", metrics.PacketLoss),
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
			"Antarmuka "+iface.InterfaceName+" terputus",
			"Antarmuka "+iface.InterfaceName+" terputus (Down)",
		)
	}
}
