package cache

import (
	"sync"
	"time"
)

// LatestDeviceMetrics represents the most recent metrics for a single device,
// combining both device metrics and aggregate interface metrics.
type LatestDeviceMetrics struct {
	DeviceID    int       `json:"device_id"`
	Name        string    `json:"name"`
	IPAddress   string    `json:"ip_address"`
	DeviceType  string    `json:"device_type"`
	Status      string    `json:"status"`
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage float64   `json:"memory_usage"`
	Latency     float64   `json:"latency"`
	PacketLoss  float64   `json:"packet_loss"`
	TxRate      float64   `json:"tx_rate"`
	RxRate      float64   `json:"rx_rate"`
	Jitter      float64   `json:"jitter"`
	Uptime      int64     `json:"uptime"`
	CollectedAt time.Time `json:"collected_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type MetricsCache struct {
	mu      sync.RWMutex
	metrics map[int]*LatestDeviceMetrics
}

var (
	GlobalMetricsCache *MetricsCache
	once               sync.Once
)

func GetMetricsCache() *MetricsCache {
	once.Do(func() {
		GlobalMetricsCache = &MetricsCache{
			metrics: make(map[int]*LatestDeviceMetrics),
		}
	})
	return GlobalMetricsCache
}

func (c *MetricsCache) Update(m *LatestDeviceMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics[m.DeviceID] = m
}

func (c *MetricsCache) GetDevice(deviceID int) *LatestDeviceMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if m, ok := c.metrics[deviceID]; ok {
		// Return a copy so the caller can modify it without holding the lock
		copy := *m
		return &copy
	}
	return nil
}

func (c *MetricsCache) GetDeviceByIP(ip string) *LatestDeviceMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, m := range c.metrics {
		if m.IPAddress == ip {
			copy := *m
			return &copy
		}
	}
	return nil
}

func (c *MetricsCache) GetAll() []map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]map[string]interface{}, 0, len(c.metrics))
	for _, m := range c.metrics {
		result = append(result, map[string]interface{}{
			"id":           m.DeviceID,
			"name":         m.Name,
			"ip_address":   m.IPAddress,
			"device_type":  m.DeviceType,
			"status":       m.Status,
			"cpu_usage":    m.CPUUsage,
			"memory_usage": m.MemoryUsage,
			"latency":      m.Latency,
			"packet_loss":  m.PacketLoss,
			"tx_rate":      m.TxRate,
			"rx_rate":      m.RxRate,
			"jitter":       m.Jitter,
			"uptime":       m.Uptime,
			"collected_at": m.CollectedAt,
			"updated_at":   m.UpdatedAt,
			"created_at":   m.CreatedAt,
			"time":         m.CollectedAt.Format(time.RFC3339),
			"ip":           m.IPAddress, // some frontend expects "ip"
		})
	}
	return result
}
