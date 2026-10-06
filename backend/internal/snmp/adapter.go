package snmp

import (
	"context"

	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

// SNMPCollectorAdapter wraps the existing Client to satisfy the worker.Collector interface
// without actually importing the worker package. (Implicit satisfaction via Duck Typing)
type SNMPCollectorAdapter struct {
	client *Client
}

// NewSNMPCollectorAdapter creates a new adapter
func NewSNMPCollectorAdapter(client *Client) *SNMPCollectorAdapter {
	return &SNMPCollectorAdapter{
		client: client,
	}
}

// Collect calls the existing SNMP polling logic and maps it to models.TelemetrySnapshot
func (a *SNMPCollectorAdapter) Collect(ctx context.Context, device models.Device) (*models.TelemetrySnapshot, error) {
	metrics, err := a.client.CollectDeviceMetrics(ctx, device)
	if err != nil {
		return nil, err
	}

	snapshot := &models.TelemetrySnapshot{
		DeviceID:           metrics.DeviceID,
		Hostname:           metrics.Hostname,
		Uptime:             metrics.Uptime,
		CPUUsage:           metrics.CPUUsage,
		MemoryUsage:        metrics.MemoryUsage,
		MemoryTotal:        metrics.MemoryTotal,
		MemoryUsed:         metrics.MemoryUsed,
		Status:             metrics.Status,
		ReachabilityStatus: metrics.ReachabilityStatus,
		SNMPStatus:         metrics.SNMPStatus,
		CollectedAt:        metrics.CollectedAt,
		LatencyMs:          metrics.LatencyMs,
		Jitter:             metrics.Jitter,
		PacketLoss:         metrics.PacketLoss,
		SignalStrength:     metrics.SignalStrength,
		CCQ:                metrics.CCQ,
		TxRate:             metrics.TxRate,
		RxRate:             metrics.RxRate,
		Airtime:            metrics.Airtime,
		SSID:               metrics.SSID,
		Frequency:          metrics.Frequency,
		Model:              metrics.Model,
		Temperature:        metrics.Temperature,
		Voltage:            metrics.Voltage,
		HasCPU:             true,
		HasMemory:          true,
		HasWireless:        true,
	}

	if len(metrics.Interfaces) > 0 {
		snapshot.Interfaces = make([]models.InterfaceSnapshot, len(metrics.Interfaces))
		for i, iface := range metrics.Interfaces {
			snapshot.Interfaces[i] = models.InterfaceSnapshot{
				DeviceID:       iface.DeviceID,
				InterfaceIndex: iface.InterfaceIndex,
				InterfaceName:  iface.InterfaceName,
				InterfaceAlias: iface.InterfaceAlias,
				Status:         iface.Status,
				InOctets:       iface.InOctets,
				OutOctets:      iface.OutOctets,
				InErrors:       iface.InErrors,
				OutErrors:      iface.OutErrors,
				InDiscards:     iface.InDiscards,
				OutDiscards:    iface.OutDiscards,
				Speed:          iface.Speed,
				RxMbps:         iface.RxMbps,
				TxMbps:         iface.TxMbps,
				CollectedAt:    iface.CollectedAt,
			}
		}
	}

	return snapshot, nil
}

// GetNetworkStats delegates to the existing client
func (a *SNMPCollectorAdapter) GetNetworkStats(ctx context.Context, ip string) (int, float64, float64, error) {
	return a.client.GetNetworkStats(ctx, ip)
}
