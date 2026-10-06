package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestThresholdRepository_GetAllRules(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewThresholdRepository(db)

	rows := sqlmock.NewRows([]string{"id", "device_id", "metric_name", "threshold_value", "strike_count", "incident_type", "is_active", "created_at", "updated_at"}).
		AddRow(1, nil, "cpu", 80.0, 1, "high_cpu", true, time.Now(), time.Now()).
		AddRow(2, nil, "memory", 85.0, 1, "high_ram", true, time.Now(), time.Now())

	mock.ExpectQuery("SELECT id, device_id, metric_name, threshold_value, strike_count, incident_type, is_active, created_at, updated_at FROM threshold_rules").
		WillReturnRows(rows)

	rules, err := repo.GetAllRules(context.Background())
	if err != nil {
		t.Errorf("error was not expected while getting rules: %s", err)
	}
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].MetricName != "cpu" {
		t.Errorf("expected metric name to be cpu, got %s", rules[0].MetricName)
	}
}

func TestThresholdRepository_GetRulesForDevice(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewThresholdRepository(db)

	// Simulate one global rule and one device-specific rule for the same metric
	deviceID := 5
	rows := sqlmock.NewRows([]string{"id", "device_id", "metric_name", "threshold_value", "strike_count", "incident_type", "is_active", "created_at", "updated_at"}).
		AddRow(2, deviceID, "cpu", 90.0, 1, "high_cpu", true, time.Now(), time.Now()). // Device-specific
		AddRow(1, nil, "cpu", 80.0, 1, "high_cpu", true, time.Now(), time.Now())       // Global fallback

	mock.ExpectQuery("SELECT id, device_id, metric_name, threshold_value, strike_count, incident_type, is_active, created_at, updated_at FROM threshold_rules WHERE is_active = 1 AND \\(device_id = \\? OR device_id IS NULL\\) ORDER BY device_id DESC").
		WithArgs(deviceID).
		WillReturnRows(rows)

	rules, err := repo.GetRulesForDevice(context.Background(), deviceID)
	if err != nil {
		t.Errorf("error was not expected while getting rules for device: %s", err)
	}

	// Should deduplicate, returning only the device-specific one
	if len(rules) != 1 {
		t.Errorf("expected 1 rule (deduplicated), got %d", len(rules))
	}
	if rules[0].ThresholdValue != 90.0 {
		t.Errorf("expected overridden threshold value 90.0, got %f", rules[0].ThresholdValue)
	}
}
