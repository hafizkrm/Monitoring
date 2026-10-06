package cache

import (
	"context"
	"testing"

	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

type mockRuleRepo struct {
	rules []models.ThresholdRule
}

func (m *mockRuleRepo) GetRulesForDevice(ctx context.Context, deviceID int) ([]models.ThresholdRule, error) {
	return nil, nil
}

func (m *mockRuleRepo) GetAllRules(ctx context.Context) ([]models.ThresholdRule, error) {
	return m.rules, nil
}

func (m *mockRuleRepo) CreateRule(ctx context.Context, rule *models.ThresholdRule) error {
	return nil
}

func (m *mockRuleRepo) UpdateRule(ctx context.Context, rule *models.ThresholdRule) error {
	return nil
}

func (m *mockRuleRepo) DeleteRule(ctx context.Context, id int) error {
	return nil
}


func TestThresholdRuleCache(t *testing.T) {
	devID := 5
	globalCPU := models.ThresholdRule{MetricName: "cpu", ThresholdValue: 80, IsActive: true}
	deviceCPU := models.ThresholdRule{DeviceID: &devID, MetricName: "cpu", ThresholdValue: 90, IsActive: true}
	inactiveRAM := models.ThresholdRule{MetricName: "memory", ThresholdValue: 85, IsActive: false}

	repo := &mockRuleRepo{
		rules: []models.ThresholdRule{globalCPU, deviceCPU, inactiveRAM},
	}

	cache := NewThresholdRuleCache(repo)
	err := cache.Refresh(context.Background())
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	rules := cache.GetRulesForDevice(devID)
	
	// Test inactive rule is still returned so processor can evaluate its IsActive flag
	if rule, exists := rules["memory"]; !exists {
		t.Error("expected inactive rule to exist in cache")
	} else if rule.IsActive {
		t.Error("expected rule to be inactive")
	}

	// Test device overrides global
	if cpu, exists := rules["cpu"]; !exists {
		t.Error("expected cpu rule")
	} else if cpu.ThresholdValue != 90 {
		t.Errorf("expected overridden threshold 90, got %f", cpu.ThresholdValue)
	}

	// Test fallback to global for other devices
	otherRules := cache.GetRulesForDevice(99)
	if cpu, exists := otherRules["cpu"]; !exists {
		t.Error("expected global cpu rule for other device")
	} else if cpu.ThresholdValue != 80 {
		t.Errorf("expected global threshold 80, got %f", cpu.ThresholdValue)
	}
}
