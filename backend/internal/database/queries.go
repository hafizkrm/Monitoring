package database

import (
	"context"
	"database/sql"
	"fmt"
	"hash/crc32"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
	"github.com/hafizkrm/Monitoring/backend/internal/utils"
)

func formatDurationHuman(seconds int) string {
	if seconds <= 0 {
		return "0m"
	}
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	mins := (seconds % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	if mins > 0 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%ds", seconds)
}

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

// GetLogsV2 returns paginated polling logs with status and date range filter support
func (db *Database) GetLogsV2(ctx context.Context, limit, offset int, search, statusFilter, rangeFilter string) ([]map[string]interface{}, int, error) {
	baseQuery := `FROM polling_logs pl LEFT JOIN devices d ON pl.device_id = d.id`
	whereConditions := []string{}
	args := []interface{}{}

	if search != "" {
		whereConditions = append(whereConditions, `(d.name LIKE ? OR d.ip_address LIKE ? OR pl.status LIKE ? OR pl.message LIKE ?)`)
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern, pattern, pattern)
	}

	if statusFilter != "" {
		switch strings.ToLower(statusFilter) {
		case "down", "offline", "failed":
			whereConditions = append(whereConditions, `(LOWER(pl.status) IN ('down', 'offline', 'failed', 'timeout', 'error') OR LOWER(pl.message) LIKE '%down%')`)
		case "up", "online", "success":
			whereConditions = append(whereConditions, `(LOWER(pl.status) IN ('up', 'online', 'success'))`)
		}
	}

	if rangeFilter != "" {
		switch rangeFilter {
		case "24h":
			whereConditions = append(whereConditions, `pl.created_at >= NOW() - INTERVAL 24 HOUR`)
		case "7d":
			whereConditions = append(whereConditions, `pl.created_at >= NOW() - INTERVAL 7 DAY`)
		case "30d":
			whereConditions = append(whereConditions, `pl.created_at >= NOW() - INTERVAL 30 DAY`)
		}
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = " WHERE " + strings.Join(whereConditions, " AND ")
	}

	countQuery := `SELECT COUNT(pl.id) ` + baseQuery + whereClause
	var total int
	_ = db.QueryRowContext(ctx, countQuery, args...).Scan(&total)

	dataQuery := `SELECT pl.created_at, COALESCE(d.name, 'Unknown'), COALESCE(d.ip_address, ''), pl.status, pl.duration_ms, COALESCE(pl.message, '') ` + baseQuery + whereClause + ` ORDER BY pl.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return []map[string]interface{}{}, 0, err
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var createdAt time.Time
		var name, ip, status, msg sql.NullString
		var durationMs sql.NullFloat64
		if err := rows.Scan(&createdAt, &name, &ip, &status, &durationMs, &msg); err == nil {
			items = append(items, map[string]interface{}{
				"timestamp":   createdAt.Format("2006-01-02 15:04:05"),
				"device_name": name.String,
				"ip_address":  ip.String,
				"status":      status.String,
				"duration_ms": durationMs.Float64,
				"message":     msg.String,
			})
		}
	}
	return items, total, nil
}

// GetActivityLogsV2 returns paginated activity logs with range filter support
func (db *Database) GetActivityLogsV2(ctx context.Context, limit, offset int, search, logType, rangeFilter string) ([]map[string]interface{}, int, error) {
	baseQuery := `FROM activity_logs`
	whereConditions := []string{}
	args := []interface{}{}

	if search != "" {
		whereConditions = append(whereConditions, `(username LIKE ? OR action LIKE ? OR module LIKE ? OR description LIKE ?)`)
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if logType != "" {
		switch logType {
		case "user":
			whereConditions = append(whereConditions, `username != 'System'`)
		case "system":
			whereConditions = append(whereConditions, `username = 'System'`)
		}
	}
	if rangeFilter != "" {
		switch rangeFilter {
		case "24h":
			whereConditions = append(whereConditions, `created_at >= NOW() - INTERVAL 24 HOUR`)
		case "7d":
			whereConditions = append(whereConditions, `created_at >= NOW() - INTERVAL 7 DAY`)
		case "30d":
			whereConditions = append(whereConditions, `created_at >= NOW() - INTERVAL 30 DAY`)
		}
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = " WHERE " + strings.Join(whereConditions, " AND ")
	}

	countQuery := `SELECT COUNT(id) ` + baseQuery + whereClause
	var total int
	_ = db.QueryRowContext(ctx, countQuery, args...).Scan(&total)

	dataQuery := `SELECT id, username, action, module, description, ip_address, created_at ` + baseQuery + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return []map[string]interface{}{}, 0, err
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var id int64
		var username, action, module, desc, ip sql.NullString
		var createdAt time.Time

		if err := rows.Scan(&id, &username, &action, &module, &desc, &ip, &createdAt); err == nil {
			items = append(items, map[string]interface{}{
				"id":          id,
				"username":    username.String,
				"action":      action.String,
				"module":      module.String,
				"description": desc.String,
				"ip_address":  ip.String,
				"created_at":  createdAt.Format("2006-01-02 15:04:05"),
			})
		}
	}

	return items, total, nil
}

// InsertActivityLog inserts a new activity log
func (db *Database) InsertActivityLog(ctx context.Context, userID *int64, user, action, module, description, ipAddress string) error {
	if user == "" {
		user = "System"
	}
	query := `INSERT INTO activity_logs (username, action, module, description, ip_address) VALUES (?, ?, ?, ?, ?)`
	_, err := db.ExecContext(ctx, query, user, action, module, description, ipAddress)
	return err
}

// GetLatestMetrics returns latest metrics for active devices
func (db *Database) GetLatestMetrics(ctx context.Context) ([]map[string]interface{}, error) {
	items := make([]map[string]interface{}, 0)

	query := `
		SELECT d.id, d.name, d.ip_address, d.device_type, COALESCE(m.status, d.status, 'up') AS status,
		       COALESCE(m.cpu_usage, 0), COALESCE(m.memory_usage, 0), COALESCE(NULLIF(m.latency, 0), 1.5), COALESCE(m.packet_loss, 0),
		       COALESCE(NULLIF(m.tx_rate, 0), im_summary.total_tx, 0) AS tx_rate,
		       COALESCE(NULLIF(m.rx_rate, 0), im_summary.total_rx, 0) AS rx_rate,
		       COALESCE(m.jitter, 0), COALESCE(m.uptime, 0), m.collected_at, d.updated_at, d.created_at
		FROM devices d
		LEFT JOIN (
			SELECT m1.*
			FROM device_metrics m1
			INNER JOIN (
				SELECT device_id, MAX(id) AS max_id
				FROM device_metrics
				WHERE collected_at >= NOW() - INTERVAL 1 HOUR
				GROUP BY device_id
			) m2 ON m1.id = m2.max_id
		) m ON m.device_id = d.id
		LEFT JOIN (
			SELECT im1.device_id, SUM(im1.rx_mbps) AS total_rx, SUM(im1.tx_mbps) AS total_tx
			FROM interface_metrics im1
			INNER JOIN (
				SELECT device_id, interface_name, MAX(collected_at) AS max_time
				FROM interface_metrics
				WHERE collected_at >= NOW() - INTERVAL 10 MINUTE
				GROUP BY device_id, interface_name
			) latest_if ON im1.device_id = latest_if.device_id AND im1.interface_name = latest_if.interface_name AND im1.collected_at = latest_if.max_time
			GROUP BY im1.device_id
		) im_summary ON im_summary.device_id = d.id
		WHERE COALESCE(d.enabled, 1) = 1
		ORDER BY d.id ASC`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("[ERROR] GetLatestMetrics query error: %v", err)
	} else {
		defer rows.Close()

		for rows.Next() {
			var rawID string
			var name, ip, devType, status sql.NullString
			var cpu, ram, latency, loss, tx, rx, jitter sql.NullFloat64
			var uptime sql.NullInt64
			var collectedAt, updatedAt, createdAt sql.NullTime

			if scanErr := rows.Scan(&rawID, &name, &ip, &devType, &status, &cpu, &ram, &latency, &loss, &tx, &rx, &jitter, &uptime, &collectedAt, &updatedAt, &createdAt); scanErr != nil {
				log.Printf("[ERROR] GetLatestMetrics scan error: %v", scanErr)
				continue
			}

			st := status.String
			if st == "" {
				st = "UP"
			}
			latVal := latency.Float64
			if latVal <= 0 {
				latVal = 1.5
			}
			jitterVal := jitter.Float64

			numID := 0
			if pId, pErr := strconv.Atoi(rawID); pErr == nil {
				numID = pId
			} else if rawID != "" {
				numID = int(crc32.ChecksumIEEE([]byte(rawID)) & 0x7fffffff)
			}

			collTime := ""
			if collectedAt.Valid {
				collTime = collectedAt.Time.Format("2006-01-02 15:04:05")
			} else if updatedAt.Valid {
				collTime = updatedAt.Time.Format("2006-01-02 15:04:05")
			} else if createdAt.Valid {
				collTime = createdAt.Time.Format("2006-01-02 15:04:05")
			}

			items = append(items, map[string]interface{}{
				"id":           numID,
				"name":         name.String,
				"ip_address":   ip.String,
				"device_type":  devType.String,
				"status":       st,
				"cpu_usage":    cpu.Float64,
				"memory_usage": ram.Float64,
				"latency_ms":   latVal,
				"jitter_ms":    jitterVal,
				"packet_loss":  loss.Float64,
				"tx_rate":      tx.Float64,
				"rx_rate":      rx.Float64,
				"uptime":       uptime.Int64,
				"collected_at": collTime,
			})
		}
	}

	// Fallback if no device metrics were found
	if len(items) == 0 {
		fallbackQuery := `SELECT id, name, ip_address, device_type, COALESCE(status, 'up') FROM devices WHERE COALESCE(enabled, 1) = 1`
		fRows, fErr := db.QueryContext(ctx, fallbackQuery)
		if fErr == nil {
			defer fRows.Close()
			for fRows.Next() {
				var rawID, name, ip, devType, status string
				if fRows.Scan(&rawID, &name, &ip, &devType, &status) == nil {
					numID := 0
					if pId, pErr := strconv.Atoi(rawID); pErr == nil {
						numID = pId
					} else if rawID != "" {
						numID = int(crc32.ChecksumIEEE([]byte(rawID)) & 0x7fffffff)
					}
					items = append(items, map[string]interface{}{
						"id":           numID,
						"raw_id":       rawID,
						"name":         name,
						"ip":           ip,
						"ip_address":   ip,
						"type":         devType,
						"device_type":  devType,
						"status":       status,
						"cpu_usage":    0.0,
						"cpu":          0.0,
						"memory_usage": 0.0,
						"memory":       0.0,
						"ram":          0.0,
						"latency_ms":   1.5,
						"latency":      1.5,
						"packet_loss":  0.0,
						"tx_rate":      0.0,
						"rx_rate":      0.0,
						"uptime":       0,
					})
				}
			}
		}
	}

	return items, nil
}

// GetMetricsHistoryByDeviceID returns metric history for a specific device ID
func (db *Database) GetMetricsHistoryByDeviceID(ctx context.Context, devID int) ([]map[string]interface{}, error) {
	query := `
		SELECT COALESCE(m.cpu_usage, 0), COALESCE(m.memory_usage, 0), COALESCE(m.latency, 0), COALESCE(m.packet_loss, 0), COALESCE(m.tx_rate, 0), COALESCE(m.rx_rate, 0), COALESCE(m.jitter, 0), m.collected_at
		FROM device_metrics m
		WHERE m.device_id = ?
		ORDER BY m.collected_at DESC
		LIMIT 60`

	rows, err := db.QueryContext(ctx, query, devID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var cpu, ram, latency, loss, tx, rx, jitter sql.NullFloat64
		var collectedAt time.Time
		if err := rows.Scan(&cpu, &ram, &latency, &loss, &tx, &rx, &jitter, &collectedAt); err == nil {
			items = append(items, map[string]interface{}{
				"timestamp":    collectedAt.Format("15:04:05"),
				"cpu_usage":    cpu.Float64,
				"memory_usage": ram.Float64,
				"latency_ms":   latency.Float64,
				"packet_loss":  loss.Float64,
				"jitter_ms":    jitter.Float64,
				"tx_rate":      tx.Float64,
				"rx_rate":      rx.Float64,
			})
		}
	}

	if items == nil {
		items = []map[string]interface{}{}
	}
	return items, nil
}

// GetUptimeReliability returns uptime reliability map
func (db *Database) GetUptimeReliability(ctx context.Context) (map[string]float64, error) {
	query := `
		SELECT d.ip_address,
		       COUNT(pl.id) as total,
		       SUM(CASE WHEN pl.status = 'success' OR pl.status = 'up' THEN 1 ELSE 0 END) as up_count
		FROM devices d
		LEFT JOIN polling_logs pl ON d.id = pl.device_id
		WHERE d.enabled = 1
		GROUP BY d.ip_address`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return map[string]float64{}, nil
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var ip string
		var total, upCount int
		if err := rows.Scan(&ip, &total, &upCount); err == nil && total > 0 {
			result[ip] = (float64(upCount) / float64(total)) * 100.0
		}
	}
	return result, nil
}

// GetTopInterfacesBandwidth returns top non-radio interface bandwidth usage
func (db *Database) GetTopInterfacesBandwidth(ctx context.Context) ([]map[string]interface{}, error) {
	// First, try getting non-zero interface metrics from interface_metrics (excluding Radio devices)
	queryIf := `
		SELECT 
			d.name AS device_name, 
			d.ip_address, 
			COALESCE(im.interface_name, 'ether1') AS interface_name, 
			COALESCE(im.interface_alias, '') AS interface_alias, 
			COALESCE(im.rx_mbps, 0) AS rx_mbps, 
			COALESCE(im.tx_mbps, 0) AS tx_mbps, 
			(COALESCE(im.rx_mbps, 0) + COALESCE(im.tx_mbps, 0)) AS total_rate, 
			im.collected_at
		FROM interface_metrics im
		JOIN devices d ON (d.id = im.device_id OR d.ip_address = im.device_id)
		INNER JOIN (
			SELECT im2.device_id, im2.interface_name, MAX(im2.collected_at) as max_time
			FROM interface_metrics im2
			WHERE im2.collected_at >= NOW() - INTERVAL 10 MINUTE
			GROUP BY im2.device_id, im2.interface_name
		) latest ON im.device_id = latest.device_id AND im.interface_name = latest.interface_name AND im.collected_at = latest.max_time
		WHERE d.enabled = 1 
		  AND LOWER(d.device_type) NOT LIKE '%radio%'
		  AND (COALESCE(im.rx_mbps, 0) + COALESCE(im.tx_mbps, 0)) > 0
		ORDER BY total_rate DESC
		LIMIT 5`

	rows, err := db.QueryContext(ctx, queryIf)
	if err == nil {
		defer rows.Close()
		var results []map[string]interface{}
		for rows.Next() {
			var devName, ip, ifName, ifAlias sql.NullString
			var rx, tx, total sql.NullFloat64
			var collectedAt sql.NullTime

			if err := rows.Scan(&devName, &ip, &ifName, &ifAlias, &rx, &tx, &total, &collectedAt); err == nil {
				results = append(results, map[string]interface{}{
					"device_name":     devName.String,
					"ip_address":      ip.String,
					"interface_name":  ifName.String,
					"interface_alias": ifAlias.String,
					"rx_mbps":         rx.Float64,
					"tx_mbps":         tx.Float64,
					"total_rate":      total.Float64,
					"collected_at":    collectedAt.Time.Format("2006-01-02 15:04:05"),
				})
			}
		}
		if len(results) > 0 {
			return results, nil
		}
	}

	// Fallback to top non-radio devices (Routers, Switches, Firewalls, APs) and label interface dynamically based on device vendor
	queryDev := `
		SELECT 
			d.name AS device_name, 
			d.ip_address, 
			CASE 
				WHEN LOWER(d.name) LIKE '%cisco%' OR LOWER(d.device_type) LIKE '%switch%' THEN 'Gi0/1'
				ELSE 'ether1'
			END AS interface_name, 
			'' AS interface_alias, 
			COALESCE(m.rx_rate, 0) AS rx_mbps, 
			COALESCE(m.tx_rate, 0) AS tx_mbps, 
			(COALESCE(m.rx_rate, 0) + COALESCE(m.tx_rate, 0)) AS total_rate, 
			m.collected_at
		FROM devices d
		JOIN (
			SELECT m1.device_id, m1.rx_rate, m1.tx_rate, m1.collected_at
			FROM device_metrics m1
			INNER JOIN (
				SELECT device_id, MAX(id) AS max_id
				FROM device_metrics
				GROUP BY device_id
			) m2 ON m1.id = m2.max_id
		) m ON m.device_id = d.id
		WHERE d.enabled = 1 
		  AND LOWER(d.device_type) NOT LIKE '%radio%'
		ORDER BY total_rate DESC, d.name ASC
		LIMIT 5`

	rowsDev, err := db.QueryContext(ctx, queryDev)
	if err != nil {
		return nil, err
	}
	defer rowsDev.Close()

	var results []map[string]interface{}
	for rowsDev.Next() {
		var devName, ip, ifName, ifAlias sql.NullString
		var rx, tx, total sql.NullFloat64
		var collectedAt sql.NullTime

		if err := rowsDev.Scan(&devName, &ip, &ifName, &ifAlias, &rx, &tx, &total, &collectedAt); err == nil {
			results = append(results, map[string]interface{}{
				"device_name":     devName.String,
				"ip_address":      ip.String,
				"interface_name":  ifName.String,
				"interface_alias": ifAlias.String,
				"rx_mbps":         rx.Float64,
				"tx_mbps":         tx.Float64,
				"total_rate":      total.Float64,
				"collected_at":    collectedAt.Time.Format("2006-01-02 15:04:05"),
			})
		}
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, nil
}

// ProbeSNMPVendor probes target IP via fast SNMP Get to auto-detect manufacturer (Ruijie, TP-Link, Ubiquiti, MikroTik, Cisco)
func ProbeSNMPVendor(ctx context.Context, ip string) string {
	snmpClient := &gosnmp.GoSNMP{
		Target:             ip,
		Port:               161,
		Community:          "public",
		Version:            gosnmp.Version2c,
		Timeout:            350 * time.Millisecond,
		Retries:            0,
		MaxOids:            5,
		ExponentialTimeout: false,
	}

	if err := snmpClient.Connect(); err != nil {
		return "Generic"
	}
	defer func() {
		if snmpClient.Conn != nil {
			snmpClient.Conn.Close()
		}
	}()

	// Query sysDescr (.1.3.6.1.2.1.1.1.0) and sysObjectID (.1.3.6.1.2.1.1.2.0)
	oids := []string{".1.3.6.1.2.1.1.1.0", ".1.3.6.1.2.1.1.2.0"}
	pkt, err := snmpClient.Get(oids)
	if err != nil || pkt == nil || len(pkt.Variables) == 0 {
		return "Generic"
	}

	var sysDescr, sysObjectID string
	for _, pdu := range pkt.Variables {
		var valStr string
		switch v := pdu.Value.(type) {
		case string:
			valStr = v
		case []byte:
			valStr = string(v)
		default:
			valStr = fmt.Sprintf("%v", v)
		}

		if strings.HasPrefix(pdu.Name, ".1.3.6.1.2.1.1.1") {
			sysDescr = strings.ToLower(valStr)
		} else if strings.HasPrefix(pdu.Name, ".1.3.6.1.2.1.1.2") {
			sysObjectID = valStr
		}
	}

	// Check Enterprise OID or SysDescr strings
	switch {
	case strings.Contains(sysObjectID, ".1.3.6.1.4.1.4881") || strings.Contains(sysDescr, "ruijie") || strings.Contains(sysDescr, "reyee"):
		return "Ruijie"
	case strings.Contains(sysObjectID, ".1.3.6.1.4.1.11863") || strings.Contains(sysObjectID, ".1.3.6.1.4.1.43224") ||
		strings.Contains(sysDescr, "tp-link") || strings.Contains(sysDescr, "tplink") || strings.Contains(sysDescr, "pharos") || strings.Contains(sysDescr, "omada") || strings.Contains(sysDescr, "jetstream"):
		return "TP-Link"
	case strings.Contains(sysObjectID, ".1.3.6.1.4.1.41112") || strings.Contains(sysDescr, "ubnt") || strings.Contains(sysDescr, "ubiquiti") || strings.Contains(sysDescr, "airmax") || strings.Contains(sysDescr, "unifi"):
		return "Ubiquiti"
	case strings.Contains(sysObjectID, ".1.3.6.1.4.1.14988") || strings.Contains(sysDescr, "mikrotik") || strings.Contains(sysDescr, "routeros") || strings.Contains(sysDescr, "routerboard"):
		return "MikroTik"
	case strings.Contains(sysObjectID, ".1.3.6.1.4.1.9") || strings.Contains(sysDescr, "cisco"):
		return "Cisco"
	default:
		return "Generic"
	}
}

// AddDeviceWithBrand adds a new device and auto-detects vendor via SNMP probe (Option B)
func (db *Database) AddDeviceWithBrand(ctx context.Context, ip, name, devType string) (string, error) {
	// Check if IP address already exists to prevent duplicate entries
	var existingName string
	errCheck := db.QueryRowContext(ctx, `SELECT name FROM devices WHERE ip_address = ? LIMIT 1`, ip).Scan(&existingName)
	if errCheck == nil {
		return "", fmt.Errorf("device with IP %s is already registered (%s)", ip, existingName)
	}

	vStr := "Generic"
	finalType := devType
	now := time.Now()

	// Check schema of devices.id to ensure we don't hit Duplicate entry '' for key 'PRIMARY'
	var dataType, extra string
	_ = db.QueryRowContext(ctx, `
		SELECT DATA_TYPE, COALESCE(EXTRA, '')
		FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() 
		  AND TABLE_NAME = 'devices' 
		  AND COLUMN_NAME = 'id'
	`).Scan(&dataType, &extra)

	var err error
	var insertedID interface{}
	if strings.Contains(strings.ToLower(extra), "auto_increment") {
		query := `INSERT INTO devices (name, hostname, ip_address, device_type, type, vendor, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)`
		res, execErr := db.ExecContext(ctx, query, name, name, ip, finalType, finalType, vStr, now, now)
		err = execErr
		if err == nil && res != nil {
			if lastID, lErr := res.LastInsertId(); lErr == nil && lastID > 0 {
				insertedID = lastID
			}
		}
	} else if strings.ToLower(dataType) == "int" || strings.ToLower(dataType) == "bigint" {
		var maxID sql.NullInt64
		_ = db.QueryRowContext(ctx, `SELECT MAX(id) FROM devices`).Scan(&maxID)
		newID := int64(1)
		if maxID.Valid {
			newID = maxID.Int64 + 1
		}
		query := `INSERT INTO devices (id, name, hostname, ip_address, device_type, type, vendor, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`
		_, err = db.ExecContext(ctx, query, newID, name, name, ip, finalType, finalType, vStr, now, now)
		insertedID = newID
	} else {
		uniqueID := fmt.Sprintf("dev_%d", now.UnixNano())
		query := `INSERT INTO devices (id, name, hostname, ip_address, device_type, type, vendor, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`
		_, err = db.ExecContext(ctx, query, uniqueID, name, name, ip, finalType, finalType, vStr, now, now)
		insertedID = uniqueID
	}

	if err == nil {
		if insertedID != nil {
			// Seed initial metric entry so device immediately shows on Latest Device Status with collected_at = NOW() (uptime=0 prevents false RECENTLY REBOOT entry)
			metricQuery := `INSERT INTO device_metrics (device_id, cpu_usage, memory_usage, tx_rate, rx_rate, status, reachability_status, snmp_status, latency, packet_loss, jitter, uptime, collected_at) VALUES (?, 0, 0, 0, 0, 'up', 'up', 'up', 1.5, 0, 0, 0, ?)`
			_, _ = db.ExecContext(ctx, metricQuery, insertedID, now)
		}
		// Asynchronous background vendor detection & offline alerts sync
		go func(devID interface{}, devIP, origType string) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			vendor := ProbeSNMPVendor(bgCtx, devIP)
			if vendor != "" && vendor != "Generic" {
				var autoType string
				dtLower := strings.ToLower(strings.TrimSpace(origType))
				if dtLower == "access_point" || dtLower == "ap" || dtLower == "" {
					switch vendor {
					case "Ruijie":
						autoType = "access_point_ruijie"
					case "TP-Link":
						autoType = "access_point_tplink"
					default:
						autoType = "access_point_" + strings.ToLower(vendor)
					}
				}
				if autoType != "" {
					_, _ = db.ExecContext(bgCtx, `UPDATE devices SET vendor = ?, device_type = ?, type = ? WHERE id = ? OR ip_address = ?`, vendor, autoType, autoType, devID, devIP)
				} else {
					_, _ = db.ExecContext(bgCtx, `UPDATE devices SET vendor = ? WHERE id = ? OR ip_address = ?`, vendor, devID, devIP)
				}
			}
		}(insertedID, ip, devType)
	}

	return vStr, err
}

// DeleteDevice deletes a device by IP along with its associated metrics and logs
func (db *Database) DeleteDevice(ctx context.Context, ip string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var idList []string
	rows, err := tx.QueryContext(ctx, `SELECT id FROM devices WHERE ip_address = ?`, ip)
	if err == nil {
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				idList = append(idList, id)
			}
		}
		rows.Close()
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM devices WHERE ip_address = ?`, ip); err != nil {
		return err
	}

	for _, devID := range idList {
		_, _ = tx.ExecContext(ctx, `DELETE FROM device_metrics WHERE device_id = ? OR device_id = ?`, devID, ip)
		_, _ = tx.ExecContext(ctx, `DELETE FROM interface_metrics WHERE device_id = ? OR device_id = ?`, devID, ip)
		_, _ = tx.ExecContext(ctx, `DELETE FROM polling_logs WHERE device_id = ?`, devID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM incidents WHERE device_id = ?`, devID)
	}

	if ip != "" {
		_, _ = tx.ExecContext(ctx, `DELETE FROM alerts WHERE message LIKE CONCAT('%', ?, '%') OR incident_id IN (SELECT id FROM incidents WHERE description LIKE CONCAT('%', ?, '%'))`, ip)
		_, _ = tx.ExecContext(ctx, `DELETE FROM incidents WHERE description LIKE CONCAT('%', ?, '%') OR device_id = ?`, ip)
	}
	_, _ = tx.ExecContext(ctx, `DELETE FROM device_metrics WHERE device_id = ?`, ip)
	_, _ = tx.ExecContext(ctx, `DELETE FROM interface_metrics WHERE device_id = ?`, ip)

	return tx.Commit()
}

// UpdateDevice updates a device
func (db *Database) UpdateDevice(ctx context.Context, oldIP, newIP, name, devType, parentIP string) error {
	query := `UPDATE devices SET ip_address = ?, name = ?, device_type = ?, parent_ip = ? WHERE ip_address = ?`
	_, err := db.ExecContext(ctx, query, newIP, name, devType, parentIP, oldIP)
	return err
}

// GetPaginatedDevices returns paginated devices
func (db *Database) GetPaginatedDevices(ctx context.Context, page, limit int, search, devType string) ([]models.Device, int, error) {
	fromClause := ` FROM devices d LEFT JOIN (
			SELECT m1.device_id, m1.status
			FROM device_metrics m1
			INNER JOIN (
				SELECT device_id, MAX(id) AS max_id
				FROM device_metrics
				WHERE collected_at >= NOW() - INTERVAL 1 HOUR
				GROUP BY device_id
			) m2 ON m1.id = m2.max_id
		) m ON m.device_id = d.id`
	whereClause := " WHERE COALESCE(d.enabled, 1) = 1"
	args := []interface{}{}

	if search != "" {
		sLower := strings.ToLower(strings.TrimSpace(search))
		switch sLower {
		case "online", "up":
			whereClause += " AND LOWER(COALESCE(m.status, d.status, 'up')) IN ('up', 'online', 'success', 'degraded')"
		case "offline", "down":
			whereClause += " AND LOWER(COALESCE(m.status, d.status, 'up')) NOT IN ('up', 'online', 'success', 'degraded')"
		default:
			whereClause += " AND (LOWER(d.name) LIKE ? OR d.ip_address LIKE ? OR LOWER(d.device_type) LIKE ? OR LOWER(COALESCE(m.status, d.status, 'up')) LIKE ?)"
			pattern := "%" + sLower + "%"
			args = append(args, pattern, pattern, pattern, pattern)
		}
	}
	if devType != "" && strings.ToLower(devType) != "semua" && strings.ToLower(devType) != "all" {
		dt := strings.ToLower(strings.TrimSpace(devType))
		switch dt {
		case "access_point", "access point", "ap":
			whereClause += " AND (LOWER(d.device_type) LIKE '%access%' OR LOWER(d.device_type) LIKE '%ap%' OR LOWER(d.device_type) LIKE '%ruijie%' OR LOWER(d.device_type) LIKE '%tplink%' OR LOWER(d.device_type) LIKE '%tp-link%')"
		case "router":
			whereClause += " AND (LOWER(d.device_type) LIKE '%router%' OR LOWER(d.device_type) LIKE '%mikrotik%')"
		case "radio":
			whereClause += " AND (LOWER(d.device_type) LIKE '%radio%' OR LOWER(d.device_type) LIKE '%ubiquiti%' OR LOWER(d.device_type) LIKE '%airmax%')"
		case "switch":
			whereClause += " AND LOWER(d.device_type) LIKE '%switch%'"
		case "server":
			whereClause += " AND LOWER(d.device_type) LIKE '%server%'"
		case "firewall":
			whereClause += " AND LOWER(d.device_type) LIKE '%firewall%'"
		case "ruijie":
			whereClause += " AND LOWER(d.device_type) LIKE '%ruijie%'"
		case "tplink", "tp-link":
			whereClause += " AND (LOWER(d.device_type) LIKE '%tplink%' OR LOWER(d.device_type) LIKE '%tp-link%')"
		default:
			whereClause += " AND LOWER(d.device_type) LIKE ?"
			args = append(args, "%"+dt+"%")
		}
	}

	var total int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) "+fromClause+whereClause, args...).Scan(&total)

	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}
	query := "SELECT d.id, COALESCE(d.name, '') AS name, COALESCE(d.ip_address, '') AS ip_address, COALESCE(d.device_type, 'router') AS device_type, COALESCE(d.enabled, 1) AS enabled, d.parent_ip, d.last_polled_at, COALESCE(d.created_at, NOW()) AS created_at, COALESCE(m.status, d.status, 'up') AS status " + fromClause + whereClause + " ORDER BY d.created_at DESC, d.id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return []models.Device{}, 0, err
	}
	defer rows.Close()

	var result []models.Device
	for rows.Next() {
		var d models.Device
		var lastPolled sql.NullTime
		var createdAt sql.NullTime
		var parentIP sql.NullString
		var rawID string
		if err := rows.Scan(&rawID, &d.Name, &d.IPAddress, &d.DeviceType, &d.Enabled, &parentIP, &lastPolled, &createdAt, &d.Status); err == nil {
			if pId, pErr := strconv.Atoi(rawID); pErr == nil {
				d.ID = pId
			} else {
				d.ID = int(crc32.ChecksumIEEE([]byte(rawID)) & 0x7fffffff)
			}
			if parentIP.Valid {
				d.ParentIP = parentIP.String
			}
			if lastPolled.Valid {
				d.LastPolledAt = &lastPolled.Time
			}
			if createdAt.Valid {
				d.CreatedAt = createdAt.Time
			}
			result = append(result, d)
		} else {
			log.Printf("Error scanning device: %v", err)
		}
	}
	return result, total, nil
}

// GetInterfaceMetricsByIP returns interface metrics
func (db *Database) GetInterfaceMetricsByIP(ctx context.Context, ip string) ([]map[string]interface{}, error) {
	var devID int
	if err := db.QueryRowContext(ctx, "SELECT id FROM devices WHERE ip_address = ? LIMIT 1", ip).Scan(&devID); err != nil {
		// If not found by IP, try parsing the string as an ID directly
		if parsedID, parseErr := strconv.Atoi(ip); parseErr == nil {
			devID = parsedID
		} else {
			// Not found and not an ID, return empty
			return []map[string]interface{}{}, nil
		}
	}

	inClause := "?"
	args := []interface{}{devID}

	query := fmt.Sprintf(`
		SELECT 
			im.interface_name,
			COALESCE(im.interface_status, 'down') AS status,
			COALESCE(im.interface_speed, 0) AS speed,
			COALESCE(im.interface_alias, '') AS alias,
			COALESCE(im.rx_mbps, 0) AS rx_mbps,
			COALESCE(im.tx_mbps, 0) AS tx_mbps,
			COALESCE(im.in_octets, 0) AS in_octets,
			COALESCE(im.out_octets, 0) AS out_octets,
			COALESCE(im.in_errors, 0) AS in_errors,
			COALESCE(im.out_errors, 0) AS out_errors,
			im.collected_at
		FROM interface_metrics im
		INNER JOIN (
			SELECT interface_name, MAX(collected_at) as max_time
			FROM interface_metrics
			WHERE device_id IN (%s)
			GROUP BY interface_name
		) latest ON im.interface_name = latest.interface_name AND im.collected_at = latest.max_time
		WHERE im.device_id IN (%s)
		ORDER BY im.interface_name ASC
	`, inClause, inClause)

	queryArgs := append(args, args...)
	rows, err := db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		log.Printf("GetInterfaceMetricsByIP query error: %v", err)
		return []map[string]interface{}{}, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var name, status, alias string
		var speed, inOctets, outOctets, inErrors, outErrors int64
		var rxMbps, txMbps float64
		var collectedAt time.Time

		if err := rows.Scan(&name, &status, &speed, &alias, &rxMbps, &txMbps, &inOctets, &outOctets, &inErrors, &outErrors, &collectedAt); err != nil {
			log.Printf("GetInterfaceMetricsByIP scan error: %v", err)
			continue
		}

		results = append(results, map[string]interface{}{
			"name":         name,
			"status":       status,
			"speed":        speed,
			"alias":        alias,
			"rx_mbps":      rxMbps,
			"tx_mbps":      txMbps,
			"in_octets":    inOctets,
			"out_octets":   outOctets,
			"in_errors":    inErrors,
			"out_errors":   outErrors,
			"collected_at": collectedAt.Format(time.RFC3339),
		})
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	return results, nil
}

// GetInventoryStats returns inventory summary stats
func (db *Database) GetInventoryStats(ctx context.Context) (map[string]interface{}, error) {
	metrics, err := db.GetLatestMetrics(ctx)
	if err != nil || len(metrics) == 0 {
		var totalDevices, totalOnline, totalOffline int
		_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM devices WHERE enabled = 1").Scan(&totalDevices)
		_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM devices WHERE enabled = 1 AND LOWER(status) IN ('up', 'online', 'success', 'degraded')").Scan(&totalOnline)
		totalOffline = totalDevices - totalOnline
		return map[string]interface{}{
			"total_devices": totalDevices,
			"online":        totalOnline,
			"offline":       totalOffline,
			"Total":         totalDevices,
			"Online":        totalOnline,
			"Offline":       totalOffline,
		}, nil
	}

	totalDevices := len(metrics)
	totalOnline := 0
	totalOffline := 0
	routerCount := 0
	radioCount := 0
	apCount := 0
	switchCount := 0
	firewallCount := 0

	for _, m := range metrics {
		rawStatus, _ := m["status"].(string)
		st := strings.ToLower(rawStatus)
		if st == "up" || st == "online" || st == "success" || st == "degraded" {
			totalOnline++
		} else {
			totalOffline++
		}

		devType, _ := m["device_type"].(string)
		if devType == "" {
			devType, _ = m["type"].(string)
		}
		devType = strings.ToLower(devType)

		if strings.Contains(devType, "router") || strings.Contains(devType, "mikrotik") {
			routerCount++
		} else if strings.Contains(devType, "radio") {
			radioCount++
		} else if strings.Contains(devType, "access") || devType == "ap" || strings.Contains(devType, "access_point") {
			apCount++
		} else if strings.Contains(devType, "switch") {
			switchCount++
		} else if strings.Contains(devType, "firewall") {
			firewallCount++
		}
	}

	return map[string]interface{}{
		"total_devices": totalDevices,
		"online":        totalOnline,
		"offline":       totalOffline,
		"Total":         totalDevices,
		"Online":        totalOnline,
		"Offline":       totalOffline,
		"Router":        routerCount,
		"Radio":         radioCount,
		"Access Point":  apCount,
		"Switch":        switchCount,
		"Firewall":      firewallCount,
	}, nil
}

// Worker DatabaseClient Methods

func (db *Database) GetAllEnabledDevices(ctx context.Context) ([]models.Device, error) {
	query := `SELECT id, name, ip_address, device_type, polling_interval, enabled, parent_ip, COALESCE(snmp_community, 'public') FROM devices WHERE enabled = 1`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.Device
	for rows.Next() {
		var d models.Device
		var parentIP, comm sql.NullString
		var rawID string
		if err := rows.Scan(&rawID, &d.Name, &d.IPAddress, &d.DeviceType, &d.PollingInterval, &d.Enabled, &parentIP, &comm); err == nil {
			if pId, pErr := strconv.Atoi(rawID); pErr == nil {
				d.ID = pId
			} else {
				d.ID = int(crc32.ChecksumIEEE([]byte(rawID)) & 0x7fffffff)
			}
			if parentIP.Valid {
				d.ParentIP = parentIP.String
			}
			if comm.Valid && comm.String != "" {
				// Phase 11: Decrypt SNMP Community (with fallback to plaintext if decryption fails/not encrypted)
				if db.encryptionKey != "" {
					decrypted, err := utils.Decrypt(comm.String, db.encryptionKey)
					if err == nil && decrypted != "" {
						d.SNMPCommunity = decrypted
					} else {
						d.SNMPCommunity = comm.String
					}
				} else {
					d.SNMPCommunity = comm.String
				}
			} else {
				d.SNMPCommunity = "public"
			}
			result = append(result, d)
		} else {
			log.Printf("Error scanning enabled device: %v", err)
		}
	}
	return result, nil
}

func (db *Database) UpdateDeviceStatus(ctx context.Context, deviceID int, reachability string, snmpStatus string, overallStatus string) error {
	query := `UPDATE devices SET reachability_status = ?, snmp_status = ?, status = ? WHERE id = ?`
	_, err := db.ExecContext(ctx, query, reachability, snmpStatus, overallStatus, deviceID)
	return err
}

func (db *Database) UpdateDeviceLastPolledAt(ctx context.Context, deviceID int) error {
	query := `UPDATE devices SET last_polled_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := db.ExecContext(ctx, query, deviceID)
	return err
}

func (db *Database) InsertPollingLog(ctx context.Context, deviceID int, status string, message string, durationMs int) error {
	query := `INSERT INTO polling_logs (device_id, status, message, duration_ms) VALUES (?, ?, ?, ?)`
	_, err := db.ExecContext(ctx, query, deviceID, status, message, durationMs)
	return err
}

func (db *Database) InsertDeviceMetric(ctx context.Context, metric *models.DeviceMetric) error {
	st := metric.Status
	switch st {
	case "", "online":
		st = "up"
	case "offline":
		st = "down"
	}

	rSt := metric.ReachabilityStatus
	if rSt == "" {
		rSt = "unknown"
	}
	sSt := metric.SNMPStatus
	if sSt == "" {
		sSt = "unknown"
	}

	lat := metric.LatencyMs
	if lat == 0 {
		lat = metric.Latency
	}
	query := `INSERT INTO device_metrics (device_id, cpu_usage, memory_usage, tx_rate, rx_rate, status, reachability_status, snmp_status, latency, packet_loss, jitter, uptime, temperature, voltage, signal_strength, ccq, model, memory_total, memory_used, collected_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := db.ExecContext(ctx, query, metric.DeviceID, metric.CPUUsage, metric.MemoryUsage, metric.TxRate, metric.RxRate, st, rSt, sSt, lat, metric.PacketLoss, metric.Jitter, metric.Uptime, metric.Temperature, metric.Voltage, metric.SignalStrength, metric.CCQ, metric.Model, metric.MemoryTotal, metric.MemoryUsed, metric.CollectedAt)
	return err
}

func (db *Database) BatchInsertDeviceMetrics(ctx context.Context, metrics []*models.DeviceMetric) error {
	if len(metrics) == 0 {
		return nil
	}

	query := "INSERT INTO device_metrics (device_id, cpu_usage, memory_usage, tx_rate, rx_rate, status, reachability_status, snmp_status, latency, packet_loss, jitter, uptime, temperature, voltage, signal_strength, ccq, model, memory_total, memory_used, collected_at) VALUES "
	vals := []interface{}{}
	placeholders := []string{}

	for _, metric := range metrics {
		st := metric.Status
		switch st {
		case "", "online":
			st = "up"
		case "offline":
			st = "down"
		}

		rSt := metric.ReachabilityStatus
		if rSt == "" {
			rSt = "unknown"
		}
		sSt := metric.SNMPStatus
		if sSt == "" {
			sSt = "unknown"
		}

		lat := metric.LatencyMs
		if lat == 0 {
			lat = metric.Latency
		}

		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		vals = append(vals, metric.DeviceID, metric.CPUUsage, metric.MemoryUsage, metric.TxRate, metric.RxRate, st, rSt, sSt, lat, metric.PacketLoss, metric.Jitter, metric.Uptime, metric.Temperature, metric.Voltage, metric.SignalStrength, metric.CCQ, metric.Model, metric.MemoryTotal, metric.MemoryUsed, metric.CollectedAt)
	}

	query += strings.Join(placeholders, ",")
	_, err := db.ExecContext(ctx, query, vals...)
	return err
}

func (db *Database) BatchInsertInterfaceMetrics(ctx context.Context, metrics []*models.InterfaceMetric) error {
	if len(metrics) == 0 {
		return nil
	}

	// Create multi-row INSERT IGNORE statement
	query := "INSERT IGNORE INTO interface_metrics (device_id, interface_index, interface_name, interface_alias, interface_status, in_octets, out_octets, in_errors, out_errors, rx_mbps, tx_mbps, collected_at) VALUES "

	vals := []interface{}{}
	placeholders := []string{}

	for _, m := range metrics {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		vals = append(vals, m.DeviceID, m.InterfaceIndex, m.InterfaceName, m.InterfaceAlias, m.InterfaceStatus, m.InOctets, m.OutOctets, m.InErrors, m.OutErrors, m.RxMbps, m.TxMbps, m.CollectedAt)
	}

	query += strings.Join(placeholders, ",")

	_, err := db.ExecContext(ctx, query, vals...)
	return err
}

func (db *Database) CreateIncident(ctx context.Context, deviceID int, incidentType string, description string) (int64, error) {
	query := `INSERT INTO incidents (device_id, type, description, status) VALUES (?, ?, ?, 'active')`
	res, err := db.ExecContext(ctx, query, deviceID, incidentType, description)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *Database) ResolveIncident(ctx context.Context, deviceID int, incidentType string) error {
	query := `UPDATE incidents SET status = 'resolved', resolved_at = CURRENT_TIMESTAMP WHERE device_id = ? AND type = ? AND status = 'active'`
	_, err := db.ExecContext(ctx, query, deviceID, incidentType)
	if err == nil {
		_, _ = db.ExecContext(ctx, `UPDATE alerts SET is_read = 1 WHERE incident_id IN (SELECT id FROM incidents WHERE device_id = ? AND type = ?)`, deviceID, incidentType)
	}
	return err
}

func (db *Database) ResolveOfflineIncidentByIP(ctx context.Context, ip string) error {
	cleanIP := strings.TrimSpace(ip)
	if cleanIP == "" {
		return nil
	}
	// 1. Mark active offline incidents as resolved
	queryIncidents := `
		UPDATE incidents 
		SET status = 'resolved', resolved_at = CURRENT_TIMESTAMP 
		WHERE status = 'active' 
		  AND (
			description LIKE CONCAT('%', ?, '%') 
			OR device_id IN (SELECT id FROM devices WHERE TRIM(ip_address) = ? OR ip_address LIKE CONCAT('%', ?, '%'))
			
		  )`
	_, _ = db.ExecContext(ctx, queryIncidents, cleanIP, cleanIP, cleanIP, cleanIP)

	// 2. Mark alerts for this IP as read
	queryAlerts := `
		UPDATE alerts 
		SET is_read = 1 
		WHERE message LIKE CONCAT('%', ?, '%')
		   OR incident_id IN (SELECT id FROM incidents WHERE device_id IN (SELECT id FROM devices WHERE TRIM(ip_address) = ? OR ip_address LIKE CONCAT('%', ?, '%')) )
	`
	_, _ = db.ExecContext(ctx, queryAlerts, cleanIP, cleanIP, cleanIP, cleanIP)

	// 3. Update device status in devices table to online/up
	_, _ = db.ExecContext(ctx, `UPDATE devices SET status = 'up', updated_at = CURRENT_TIMESTAMP WHERE TRIM(ip_address) = ? OR ip_address LIKE CONCAT('%', ?, '%')`, cleanIP, cleanIP)

	// 4. Insert an online metric so SyncOfflineAlerts won't revert device to offline immediately
	var devID int
	err := db.QueryRowContext(ctx, `SELECT id FROM devices WHERE TRIM(ip_address) = ? OR ip_address LIKE CONCAT('%', ?, '%') LIMIT 1`, cleanIP, cleanIP).Scan(&devID)
	if err == nil && devID > 0 {
		_ = db.InsertDeviceMetric(ctx, &models.DeviceMetric{
			DeviceID:    devID,
			Status:      "up",
			LatencyMs:   10,
			PacketLoss:  0,
			CPUUsage:    0,
			MemoryUsage: 0,
		})
	}

	return nil
}

func (db *Database) GetActiveIncident(ctx context.Context, deviceID int, incidentType string) (int64, error) {
	query := `SELECT id FROM incidents WHERE device_id = ? AND type = ? AND status = 'active' LIMIT 1`
	var id int64
	err := db.QueryRowContext(ctx, query, deviceID, incidentType).Scan(&id)
	return id, err
}

func (db *Database) GetActiveIncidentsByDevice(ctx context.Context, deviceID int) (map[string]int64, error) {
	query := `SELECT id, type FROM incidents WHERE device_id = ? AND status = 'active'`
	rows, err := db.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	active := make(map[string]int64)
	for rows.Next() {
		var id int64
		var incType string
		if err := rows.Scan(&id, &incType); err == nil {
			active[incType] = id
		}
	}
	return active, nil
}

func (db *Database) CreateAlert(ctx context.Context, incidentID interface{}, alertType string, message string) error {
	// Prevent duplicate unread messages from flooding the UI
	var exists int
	_ = db.QueryRowContext(ctx, `SELECT 1 FROM alerts WHERE message = ? AND is_read = 0 AND created_at >= NOW() - INTERVAL 12 HOUR LIMIT 1`, message).Scan(&exists)
	if exists == 1 {
		return nil // Skip creating duplicate alert
	}

	query := `INSERT INTO alerts (incident_id, type, message) VALUES (?, ?, ?)`
	_, err := db.ExecContext(ctx, query, incidentID, alertType, message)
	return err
}

func (db *Database) CleanupOldData(ctx context.Context, metricsDays int, logsDays int) (map[string]int64, error) {
	// P1-14 / P0-15: Retention Optimization - Batched Deletes
	// Prevent database locking and transaction log explosion by deleting in chunks

	stats := make(map[string]int64)
	var errs []string

	deleteInBatches := func(tableName string, timeCol string, days int) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE %s < DATE_SUB(NOW(), INTERVAL ? DAY) LIMIT 5000`, tableName, timeCol)
		var totalDeleted int64 = 0
		for {
			select {
			case <-ctx.Done():
				stats[tableName] = totalDeleted
				return ctx.Err()
			default:
			}

			res, err := db.ExecContext(ctx, query, days)
			if err != nil {
				stats[tableName] = totalDeleted
				return fmt.Errorf("failed to batch delete %s: %w", tableName, err)
			}

			affected, err := res.RowsAffected()
			if err != nil {
				stats[tableName] = totalDeleted
				return err
			}

			totalDeleted += affected

			if affected == 0 {
				break // Done
			}
			// Small sleep to yield DB resources to other queries (prevent locking)
			time.Sleep(50 * time.Millisecond)
		}
		stats[tableName] = totalDeleted
		return nil
	}

	if err := deleteInBatches("polling_logs", "created_at", logsDays); err != nil {
		log.Printf("[WARNING] Cleanup polling_logs failed: %v", err)
		errs = append(errs, err.Error())
	}

	if err := deleteInBatches("activity_logs", "created_at", logsDays); err != nil {
		log.Printf("[WARNING] Cleanup activity_logs failed: %v", err)
		errs = append(errs, err.Error())
	}

	if err := deleteInBatches("device_status_history", "created_at", logsDays); err != nil {
		log.Printf("[WARNING] Cleanup device_status_history failed: %v", err)
		errs = append(errs, err.Error())
	}

	if err := deleteInBatches("device_metrics", "collected_at", metricsDays); err != nil {
		log.Printf("[WARNING] Cleanup device_metrics failed: %v", err)
		errs = append(errs, err.Error())
	}

	if err := deleteInBatches("interface_metrics", "collected_at", metricsDays); err != nil {
		log.Printf("[WARNING] Cleanup interface_metrics failed: %v", err)
		errs = append(errs, err.Error())
	}

	// Note: The previous downsampling query (DELETE JOIN) was highly destructive and caused massive table locks.
	// It has been disabled in favor of standard retention deletion.
	// To implement proper downsampling, an aggregation table (e.g. device_metrics_hourly) should be used.

	if len(errs) > 0 {
		return stats, fmt.Errorf("cleanup completed with errors: %s", strings.Join(errs, "; "))
	}

	return stats, nil
}

// GetGlobalBandwidthHistory returns aggregated historical bandwidth for gateway devices (routers/firewalls)

func (db *Database) GetGlobalBandwidthHistory(ctx context.Context, durationStr string) ([]map[string]interface{}, error) {
	dur, err := time.ParseDuration(durationStr)
	if err != nil || dur <= 0 {
		dur = 30 * time.Minute
	}

	intervalHours := int(dur.Hours())
	if intervalHours == 0 {
		intervalHours = 1 // At least 1 hour for DB interval if < 1h
	}
	if intervalHours < 1 {
		intervalHours = 1
	} // Safe check

	// Determine grouping divisor to avoid too many points
	var divisor int = 10
	if dur > 12*time.Hour {
		divisor = 300 // 5m
	} else if dur > 2*time.Hour {
		divisor = 60 // 1m
	} else if dur > 1*time.Hour {
		divisor = 30 // 30s
	}

	query := fmt.Sprintf(`
		SELECT 
			FROM_UNIXTIME(FLOOR(UNIX_TIMESTAMP(m.collected_at) / %d) * %d, '%%H:%%i:%%s') AS time_label,
			COALESCE(SUM(m.rx_rate), 0) AS total_rx,
			COALESCE(SUM(m.tx_rate), 0) AS total_tx
		FROM device_metrics m
		JOIN devices d ON m.device_id = d.id
		WHERE (LOWER(d.device_type) LIKE '%%router%%' OR LOWER(d.device_type) LIKE '%%firewall%%' OR LOWER(d.device_type) LIKE '%%gateway%%')
		  AND m.collected_at >= NOW() - INTERVAL %d HOUR
		GROUP BY FLOOR(UNIX_TIMESTAMP(m.collected_at) / %d)
		ORDER BY MIN(m.collected_at) DESC
		LIMIT 60`, divisor, divisor, intervalHours, divisor)
	rows, err := db.QueryContext(ctx, query)
	var items []map[string]interface{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var timeLabel string
			var totalRx, totalTx float64
			if err := rows.Scan(&timeLabel, &totalRx, &totalTx); err == nil {
				items = append(items, map[string]interface{}{
					"timestamp": timeLabel,
					"download":  totalRx,
					"upload":    totalTx,
				})
			}
		}

		// Reverse so oldest timestamp is first for Chart.js
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	}

	// If DB yields no history yet (e.g. fresh startup), generate smooth baseline timeline
	if len(items) == 0 {
		now := time.Now()
		for i := 19; i >= 0; i-- {
			t := now.Add(time.Duration(-i*5) * time.Second)
			items = append(items, map[string]interface{}{
				"timestamp": t.Format("15:04:05"),
				"download":  0.0,
				"upload":    0.0,
			})
		}
	}

	return items, nil
}
