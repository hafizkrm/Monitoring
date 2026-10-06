package repository

import (
	"context"
	"database/sql"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

// ThresholdRepository defines the interface for managing threshold rules
type ThresholdRepository interface {
	GetRulesForDevice(ctx context.Context, deviceID int) ([]models.ThresholdRule, error)
	GetAllRules(ctx context.Context) ([]models.ThresholdRule, error)
	CreateRule(ctx context.Context, rule *models.ThresholdRule) error
	UpdateRule(ctx context.Context, rule *models.ThresholdRule) error
	DeleteRule(ctx context.Context, id int) error
}

type thresholdRepository struct {
	db *sql.DB
}

// NewThresholdRepository creates a new ThresholdRepository
func NewThresholdRepository(db *sql.DB) ThresholdRepository {
	return &thresholdRepository{db: db}
}

// GetRulesForDevice retrieves rules applicable to a device.
// It fetches device-specific rules and falls back to global rules for metrics not overridden by the device.
func (r *thresholdRepository) GetRulesForDevice(ctx context.Context, deviceID int) ([]models.ThresholdRule, error) {
	query := `
		SELECT id, device_id, metric_name, threshold_value, strike_count, incident_type, is_active, created_at, updated_at
		FROM threshold_rules
		WHERE is_active = 1 AND (device_id = ? OR device_id IS NULL)
		ORDER BY device_id DESC -- Device specific rules come first
	`
	rows, err := r.db.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.ThresholdRule
	seenMetrics := make(map[string]bool)

	for rows.Next() {
		var rule models.ThresholdRule
		err := rows.Scan(
			&rule.ID,
			&rule.DeviceID,
			&rule.MetricName,
			&rule.ThresholdValue,
			&rule.StrikeCount,
			&rule.IncidentType,
			&rule.IsActive,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Because we order by device_id DESC, we process device_id != NULL first.
		// If we already saw the metric for this device, skip the global one.
		if !seenMetrics[rule.MetricName] {
			seenMetrics[rule.MetricName] = true
			rules = append(rules, rule)
		}
	}

	return rules, nil
}

// GetAllRules retrieves all rules (useful for caching or administrative views)
func (r *thresholdRepository) GetAllRules(ctx context.Context) ([]models.ThresholdRule, error) {
	query := `
		SELECT id, device_id, metric_name, threshold_value, strike_count, incident_type, is_active, created_at, updated_at
		FROM threshold_rules
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.ThresholdRule
	for rows.Next() {
		var rule models.ThresholdRule
		err := rows.Scan(
			&rule.ID,
			&rule.DeviceID,
			&rule.MetricName,
			&rule.ThresholdValue,
			&rule.StrikeCount,
			&rule.IncidentType,
			&rule.IsActive,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

func (r *thresholdRepository) CreateRule(ctx context.Context, rule *models.ThresholdRule) error {
	query := `INSERT INTO threshold_rules (device_id, metric_name, threshold_value, strike_count, incident_type, is_active)
	          VALUES (?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, rule.DeviceID, rule.MetricName, rule.ThresholdValue, rule.StrikeCount, rule.IncidentType, rule.IsActive)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		rule.ID = int(id)
	}
	return err
}

func (r *thresholdRepository) UpdateRule(ctx context.Context, rule *models.ThresholdRule) error {
	query := `UPDATE threshold_rules 
	          SET device_id = ?, metric_name = ?, threshold_value = ?, strike_count = ?, incident_type = ?, is_active = ?
	          WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, rule.DeviceID, rule.MetricName, rule.ThresholdValue, rule.StrikeCount, rule.IncidentType, rule.IsActive, rule.ID)
	return err
}

func (r *thresholdRepository) DeleteRule(ctx context.Context, id int) error {
	query := `DELETE FROM threshold_rules WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

