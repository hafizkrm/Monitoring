package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// GetAvailabilityReport generates availability report
func (db *Database) GetAvailabilityReport(ctx context.Context, start time.Time, end time.Time) (map[string]interface{}, error) {
	query := `
		SELECT d.name, d.ip_address, d.device_type, d.polling_interval,
		       SUM(CASE WHEN pl.status = 'success' OR pl.status = 'up' THEN pl.duration_ms ELSE 0 END) as uptime_ms,
		       SUM(CASE WHEN pl.status != 'success' AND pl.status != 'up' THEN 1 ELSE 0 END) as downtime_count,
		       COUNT(pl.id) as total_polls
		FROM devices d
		LEFT JOIN polling_logs pl ON d.id = pl.device_id AND pl.created_at >= ? AND pl.created_at <= ?
		WHERE d.enabled = 1
		GROUP BY d.id, d.name, d.ip_address, d.device_type, d.polling_interval
		ORDER BY d.name ASC`

	ctx, cancel := context.WithTimeout(ctx, db.config.ConnectionTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query availability report: %w", err)
	}
	defer rows.Close()

	var items []map[string]interface{}
	var totalUptimePct float64
	var count int
	var compliantCount int
	var violatedCount int

	for rows.Next() {
		var name, ip, deviceType sql.NullString
		var pollingInterval, uptimeMs, downtimeCount, totalPolls int
		if err := rows.Scan(&name, &ip, &deviceType, &pollingInterval, &uptimeMs, &downtimeCount, &totalPolls); err != nil {
			continue
		}

		if pollingInterval <= 0 {
			pollingInterval = 60 // Default fallback
		}
		downtimeSeconds := downtimeCount * pollingInterval
		uptimeSeconds := (totalPolls - downtimeCount) * pollingInterval
		if uptimeSeconds < 0 {
			uptimeSeconds = 0
		}

		availability := 0.0
		if totalPolls > 0 {
			availability = float64(totalPolls-downtimeCount) * 100.0 / float64(totalPolls)
		}

		uptimeStr := formatDurationHuman(uptimeSeconds)
		downtimeStr := formatDurationHuman(downtimeSeconds)

		slaStatus := "COMPLIANT"
		if availability < 99.0 {
			slaStatus = "VIOLATED"
			violatedCount++
		} else {
			compliantCount++
		}

		items = append(items, map[string]interface{}{
			"name":             name.String,
			"ip":               ip.String,
			"type":             deviceType.String,
			"uptime_str":       uptimeStr,
			"downtime_str":     downtimeStr,
			"downtime_seconds": downtimeSeconds,
			"down_count":       downtimeCount,
			"availability_pct": fmt.Sprintf("%.1f", availability),
			"sla_target":       "99.0%",
			"sla_status":       slaStatus,
		})

		totalUptimePct += availability
		count++
	}

	avgAvailability := 0.0
	if count > 0 {
		avgAvailability = totalUptimePct / float64(count)
	}

	summary := map[string]interface{}{
		"Total Devices":     count,
		"Average SLA (%)":   fmt.Sprintf("%.2f%%", avgAvailability),
		"SLA Met (>=99%)":   fmt.Sprintf("%d Devices", compliantCount),
		"SLA Missed (<99%)": fmt.Sprintf("%d Devices", violatedCount),
	}

	return map[string]interface{}{
		"summary": summary,
		"items":   items,
	}, nil
}

// GetPerformanceReport generates performance report
func (db *Database) GetPerformanceReport(ctx context.Context, start time.Time, end time.Time) (map[string]interface{}, error) {
	query := `
		SELECT d.name, d.ip_address,
		       IFNULL(AVG(m.cpu_usage), 0) as avg_cpu,
		       IFNULL(MAX(m.cpu_usage), 0) as max_cpu,
		       IFNULL(AVG(m.memory_usage), 0) as avg_ram,
		       IFNULL(MAX(m.memory_usage), 0) as max_ram,
		       IFNULL(AVG(m.tx_rate + m.rx_rate), 0) * 1000000 as avg_traffic
		FROM devices d
		LEFT JOIN device_metrics m ON d.id = m.device_id AND m.collected_at >= ? AND m.collected_at <= ?
		WHERE d.enabled = 1
		GROUP BY d.id, d.name, d.ip_address
		ORDER BY d.name ASC`

	ctx, cancel := context.WithTimeout(ctx, db.config.ConnectionTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query performance report: %w", err)
	}
	defer rows.Close()

	var items []map[string]interface{}
	var sumCpu, sumRam float64
	var count int

	for rows.Next() {
		var name, ip sql.NullString
		var avgCpu, maxCpu, avgRam, maxRam, avgTraffic sql.NullFloat64
		if err := rows.Scan(&name, &ip, &avgCpu, &maxCpu, &avgRam, &maxRam, &avgTraffic); err != nil {
			continue
		}

		items = append(items, map[string]interface{}{
			"name":        name.String,
			"ip":          ip.String,
			"avg_cpu":     fmt.Sprintf("%.1f", avgCpu.Float64),
			"max_cpu":     fmt.Sprintf("%.1f", maxCpu.Float64),
			"avg_ram":     fmt.Sprintf("%.1f", avgRam.Float64),
			"max_ram":     fmt.Sprintf("%.1f", maxRam.Float64),
			"avg_traffic": formatBandwidth(avgTraffic.Float64),
		})

		sumCpu += avgCpu.Float64
		sumRam += avgRam.Float64
		count++
	}

	summary := map[string]interface{}{
		"Total Devices": count,
		"Average CPU":   "0%",
		"Average RAM":   "0%",
	}
	if count > 0 {
		summary["Average CPU"] = fmt.Sprintf("%.1f%%", sumCpu/float64(count))
		summary["Average RAM"] = fmt.Sprintf("%.1f%%", sumRam/float64(count))
	}

	return map[string]interface{}{
		"summary": summary,
		"items":   items,
	}, nil
}

// GetIncidentsReport generates incidents report
func (db *Database) GetIncidentsReport(ctx context.Context, start time.Time, end time.Time) (map[string]interface{}, error) {
	query := `
		SELECT a.created_at, COALESCE(NULLIF(d.name, ''), 'Unknown') as device_name, COALESCE(a.type, 'warning') as severity, a.message, COALESCE(i.status, 'active') as status
		FROM alerts a
		LEFT JOIN incidents i ON a.incident_id = i.id
		LEFT JOIN devices d ON i.device_id = d.id
		WHERE a.created_at >= ? AND a.created_at <= ?
		ORDER BY a.created_at DESC
		LIMIT 1000`

	ctx, cancel := context.WithTimeout(ctx, db.config.ConnectionTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, query, start, end)
	if err != nil {
		return map[string]interface{}{
			"summary": map[string]interface{}{"Total Incidents": 0},
			"items":   []interface{}{},
		}, nil
	}
	defer rows.Close()

	var items []map[string]interface{}
	var count int
	critCount := 0
	warnCount := 0

	for rows.Next() {
		var createdAt time.Time
		var devName, severity, message, status sql.NullString
		if err := rows.Scan(&createdAt, &devName, &severity, &message, &status); err != nil {
			continue
		}

		sev := severity.String
		switch sev {
		case "critical":
			critCount++
		case "warning":
			warnCount++
		}

		items = append(items, map[string]interface{}{
			"created_at":  createdAt.Format("2006-01-02 15:04:05"),
			"device_name": devName.String,
			"severity":    sev,
			"message":     message.String,
			"status":      status.String,
		})
		count++
	}

	summary := map[string]interface{}{
		"Total Incidents Logged": count,
		"Critical Incidents":     critCount,
		"Warning Incidents":      warnCount,
	}

	return map[string]interface{}{
		"summary": summary,
		"items":   items,
	}, nil
}

// GetDownFrequencyReport generates top disruption report
func (db *Database) GetDownFrequencyReport(ctx context.Context, start time.Time, end time.Time) (map[string]interface{}, error) {
	query := `
		SELECT d.name, d.ip_address, d.device_type,
		       COUNT(pl.id) as down_count,
		       MAX(pl.created_at) as last_down
		FROM devices d
		JOIN polling_logs pl ON d.id = pl.device_id AND pl.created_at >= ? AND pl.created_at <= ?
		WHERE d.enabled = 1 AND (LOWER(pl.status) IN ('down', 'failed', 'timeout', 'error') OR LOWER(pl.message) LIKE '%down%')
		GROUP BY d.id, d.name, d.ip_address, d.device_type
		ORDER BY down_count DESC
		LIMIT 50`

	ctx, cancel := context.WithTimeout(ctx, db.config.ConnectionTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, query, start, end)
	if err != nil {
		return map[string]interface{}{
			"summary": map[string]interface{}{"Total Devices Disrupted": 0},
			"items":   []interface{}{},
		}, nil
	}
	defer rows.Close()

	var items []map[string]interface{}
	var totalDisruptions int

	for rows.Next() {
		var name, ip, deviceType sql.NullString
		var downCount int
		var lastDown sql.NullTime
		if err := rows.Scan(&name, &ip, &deviceType, &downCount, &lastDown); err != nil {
			continue
		}

		lastDownStr := ""
		if lastDown.Valid {
			lastDownStr = lastDown.Time.Format("2006-01-02 15:04:05")
		}

		items = append(items, map[string]interface{}{
			"name":       name.String,
			"ip":         ip.String,
			"type":       deviceType.String,
			"down_count": downCount,
			"last_down":  lastDownStr,
		})
		totalDisruptions += downCount
	}

	summary := map[string]interface{}{
		"Frequent Down Devices": len(items),
		"Total Down Events":     totalDisruptions,
	}

	return map[string]interface{}{
		"summary": summary,
		"items":   items,
	}, nil
}
