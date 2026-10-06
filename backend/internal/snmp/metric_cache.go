package snmp

import (
	"sync"
	"time"
)

type DeviceMetricCache struct {
	Metrics   *DeviceMetrics
	LastSys   time.Time
	LastIface time.Time
	LastWire  time.Time
	mu        sync.RWMutex
}

type MetricCacheStore struct {
	cache sync.Map
}

func (s *MetricCacheStore) Get(deviceID int) *DeviceMetricCache {
	if val, ok := s.cache.Load(deviceID); ok {
		return val.(*DeviceMetricCache)
	}
	newCache := &DeviceMetricCache{
		Metrics: &DeviceMetrics{DeviceID: deviceID},
	}
	s.cache.Store(deviceID, newCache)
	return newCache
}
