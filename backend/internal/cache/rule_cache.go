package cache

import (
	"context"
	"sync"

	"github.com/hafizkrm/Monitoring/backend/internal/models"
	"github.com/hafizkrm/Monitoring/backend/internal/repository"
)

type ThresholdRuleCache interface {
	GetRulesForDevice(deviceID int) map[string]models.ThresholdRule
	Refresh(ctx context.Context) error
}

type thresholdRuleCache struct {
	repo    repository.ThresholdRepository
	mu      sync.RWMutex
	global  map[string]models.ThresholdRule
	devices map[int]map[string]models.ThresholdRule
}

func NewThresholdRuleCache(repo repository.ThresholdRepository) ThresholdRuleCache {
	return &thresholdRuleCache{
		repo:    repo,
		global:  make(map[string]models.ThresholdRule),
		devices: make(map[int]map[string]models.ThresholdRule),
	}
}

func (c *thresholdRuleCache) Refresh(ctx context.Context) error {
	rules, err := c.repo.GetAllRules(ctx)
	if err != nil {
		return err
	}

	newGlobal := make(map[string]models.ThresholdRule)
	newDevices := make(map[int]map[string]models.ThresholdRule)

	for _, rule := range rules {
		if rule.DeviceID == nil {
			newGlobal[rule.MetricName] = rule
		} else {
			deviceID := *rule.DeviceID
			if _, exists := newDevices[deviceID]; !exists {
				newDevices[deviceID] = make(map[string]models.ThresholdRule)
			}
			newDevices[deviceID][rule.MetricName] = rule
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.global = newGlobal
	c.devices = newDevices

	return nil
}

func (c *thresholdRuleCache) GetRulesForDevice(deviceID int) map[string]models.ThresholdRule {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]models.ThresholdRule)
	// Apply global rules first
	for k, v := range c.global {
		result[k] = v
	}

	// Override with device-specific rules
	if deviceRules, exists := c.devices[deviceID]; exists {
		for k, v := range deviceRules {
			result[k] = v
		}
	}

	return result
}
