package models

import "time"

// ThresholdRule defines a dynamic rule for evaluating metrics
type ThresholdRule struct {
	ID             int       `json:"id"`
	DeviceID       *int      `json:"device_id"` // nil means global rule
	MetricName     string    `json:"metric_name"`
	ThresholdValue float64   `json:"threshold_value"`
	StrikeCount    int       `json:"strike_count"`
	IncidentType   string    `json:"incident_type"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
