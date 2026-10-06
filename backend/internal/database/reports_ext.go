package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func formatBandwidth(bps float64) string {
	if bps > 1000000000 {
		return fmt.Sprintf("%.1f Gbps", bps/1000000000)
	}
	if bps > 1000000 {
		return fmt.Sprintf("%.1f Mbps", bps/1000000)
	}
	if bps > 1000 {
		return fmt.Sprintf("%.1f Kbps", bps/1000)
	}
	return fmt.Sprintf("%.0f bps", bps)
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	var res string
	if days > 0 {
		res += fmt.Sprintf("%dh ", days)
	}
	if hours > 0 {
		res += fmt.Sprintf("%djam ", hours)
	}
	if minutes > 0 || res == "" {
		res += fmt.Sprintf("%dmenit", minutes)
	}
	return res
}

// GetTrafficMonthlyReport generates traffic report for the given range
func (db *Database) GetTrafficMonthlyReport(ctx context.Context, start time.Time, end time.Time) (map[string]interface{}, error) {
	query := `
		SELECT d.name, d.ip_address,
		       IFNULL(AVG(m.tx_rate + m.rx_rate), 0) * 1000000 as avg_traffic,
			   IFNULL(MAX(m.tx_rate + m.rx_rate), 0) * 1000000 as max_traffic
		FROM devices d
		LEFT JOIN device_metrics m ON d.id = m.device_id AND m.collected_at >= ? AND m.collected_at <= ?
		WHERE d.enabled = 1
		GROUP BY d.id, d.name, d.ip_address
		ORDER BY avg_traffic DESC`

	ctx, cancel := context.WithTimeout(ctx, db.config.ConnectionTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query traffic report: %w", err)
	}
	defer rows.Close()

	var items []map[string]interface{}
	var totalAvgTraffic float64
	var count int

	for rows.Next() {
		var name, ip sql.NullString
		var avgTraffic, maxTraffic sql.NullFloat64
		if err := rows.Scan(&name, &ip, &avgTraffic, &maxTraffic); err != nil {
			continue
		}

		items = append(items, map[string]interface{}{
			"name":        name.String,
			"ip":          ip.String,
			"avg_traffic": formatBandwidth(avgTraffic.Float64),
			"max_traffic": formatBandwidth(maxTraffic.Float64),
		})

		totalAvgTraffic += avgTraffic.Float64
		count++
	}

	summary := map[string]interface{}{
		"Total Perangkat":       count,
		"Rata-rata Keseluruhan": "0 bps",
	}

	if count > 0 {
		summary["Rata-rata Keseluruhan"] = formatBandwidth(totalAvgTraffic / float64(count))
	}

	return map[string]interface{}{
		"summary": summary,
		"items":   items,
	}, nil
}

// GetDowntimeMonthlyReport generates precise downtime report
func (db *Database) GetDowntimeMonthlyReport(ctx context.Context, start time.Time, end time.Time) (map[string]interface{}, error) {
	// First get all enabled devices
	devQuery := `SELECT id, name, ip_address FROM devices WHERE enabled = 1`
	devRows, err := db.QueryContext(ctx, devQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query devices: %w", err)
	}

	type deviceInfo struct {
		Name string
		IP   string
	}
	devices := make(map[int]deviceInfo)
	for devRows.Next() {
		var id int
		var name, ip string
		if err := devRows.Scan(&id, &name, &ip); err == nil {
			devices[id] = deviceInfo{Name: name, IP: ip}
		}
	}
	devRows.Close()

	// Then get all downtime logs
	logQuery := `
		SELECT device_id, created_at
		FROM polling_logs
		WHERE (status IN ('timeout', 'error') 
		   OR LOWER(message) LIKE '%down%' 
		   OR LOWER(message) LIKE '%failed%' 
		   OR LOWER(message) LIKE '%unreachable%' 
		   OR LOWER(message) LIKE '%reboot%')
		  AND created_at >= ? AND created_at <= ?
		ORDER BY device_id, created_at ASC`

	ctx, cancel := context.WithTimeout(ctx, db.config.ConnectionTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, logQuery, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query polling logs: %w", err)
	}
	defer rows.Close()

	type devStats struct {
		Incidents     int
		TotalSeconds  int64
		LastLogTime   time.Time
		IncidentStart time.Time
	}
	stats := make(map[int]*devStats)

	for rows.Next() {
		var devID int
		var createdAt time.Time
		if err := rows.Scan(&devID, &createdAt); err != nil {
			continue
		}

		s, exists := stats[devID]
		if !exists {
			s = &devStats{
				Incidents:     1,
				TotalSeconds:  60, // default 1 minute for single log
				LastLogTime:   createdAt,
				IncidentStart: createdAt,
			}
			stats[devID] = s
			continue
		}

		// If time gap is > 5 minutes, consider it a new incident
		gap := createdAt.Sub(s.LastLogTime)
		if gap > 5*time.Minute {
			s.Incidents++
			s.TotalSeconds += 60 // 1 min for the new incident start
			s.IncidentStart = createdAt
		} else {
			// Extend current incident
			s.TotalSeconds += int64(gap.Seconds())
		}
		s.LastLogTime = createdAt
	}

	var items []map[string]interface{}
	var sumDowntime int64
	var totalDown int

	for id, info := range devices {
		var kaliDown int
		var durasiSec int64

		if s, ok := stats[id]; ok {
			kaliDown = s.Incidents
			durasiSec = s.TotalSeconds
		}

		totalRangeSeconds := end.Sub(start).Seconds()
		if totalRangeSeconds <= 0 {
			totalRangeSeconds = 1 // Prevent div by zero
		}
		uptimePercent := ((totalRangeSeconds - float64(durasiSec)) / totalRangeSeconds) * 100
		if uptimePercent < 0 {
			uptimePercent = 0
		}

		items = append(items, map[string]interface{}{
			"name":           info.Name,
			"ip":             info.IP,
			"kali_down":      kaliDown,
			"durasi_down":    formatDuration(time.Duration(durasiSec) * time.Second),
			"uptime_percent": fmt.Sprintf("%.2f%%", uptimePercent),
		})

		sumDowntime += durasiSec
		totalDown += kaliDown
	}

	// Sort by kali_down DESC
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i]["kali_down"].(int) < items[j]["kali_down"].(int) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	summary := map[string]interface{}{
		"Total Insiden Down": totalDown,
		"Total Durasi Down":  formatDuration(time.Duration(sumDowntime) * time.Second),
	}

	return map[string]interface{}{
		"summary": summary,
		"items":   items,
	}, nil
}

// GetVPNMonthlyReport generates VPN users connection history report
func (db *Database) GetVPNMonthlyReport(ctx context.Context, start time.Time, end time.Time) (map[string]interface{}, error) {
	query := `
		SELECT name, service, caller_id, address, uptime, connected_at
		FROM vpn_connections
		WHERE connected_at >= ? AND connected_at <= ?
		ORDER BY connected_at DESC
		LIMIT 1000`

	ctx, cancel := context.WithTimeout(ctx, db.config.ConnectionTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, query, start, end)
	if err != nil {
		return map[string]interface{}{
			"summary": map[string]interface{}{"Total Koneksi": 0},
			"items":   []interface{}{},
		}, nil
	}
	defer rows.Close()

	var items []map[string]interface{}
	var count int
	uniqueUsers := make(map[string]bool)

	for rows.Next() {
		var name, service, callerID, address, uptime sql.NullString
		var connectedAt sql.NullTime
		if err := rows.Scan(&name, &service, &callerID, &address, &uptime, &connectedAt); err != nil {
			continue
		}

		items = append(items, map[string]interface{}{
			"name":         name.String,
			"service":      service.String,
			"caller_id":    callerID.String,
			"address":      address.String,
			"uptime":       uptime.String,
			"connected_at": connectedAt.Time,
		})

		count++
		uniqueUsers[name.String] = true
	}

	summary := map[string]interface{}{
		"Total Koneksi Terdaftar": count,
		"User Unik Terhubung":     len(uniqueUsers),
	}

	return map[string]interface{}{
		"summary": summary,
		"items":   items,
	}, nil
}

// GetBandwidthTopTalkersReport generates top talkers report
func (db *Database) GetBandwidthTopTalkersReport(ctx context.Context, start time.Time, end time.Time) (map[string]interface{}, error) {
	res, err := db.GetTrafficMonthlyReport(ctx, start, end)
	if err != nil {
		return nil, err
	}

	items := res["items"].([]map[string]interface{})
	if len(items) > 10 {
		items = items[:10]
		res["items"] = items
	}
	return res, nil
}

// GetNetworkQualityReport generates network quality (latency, packet loss)
func (db *Database) GetNetworkQualityReport(ctx context.Context, start time.Time, end time.Time) (map[string]interface{}, error) {
	query := `
		SELECT d.name, d.ip_address,
		       COALESCE(AVG(CASE WHEN LOWER(pl.status) IN ('up', 'online', 'ok') THEN pl.duration_ms ELSE NULL END), 0) as avg_latency,
		       COUNT(pl.id) as total_polls,
		       SUM(CASE WHEN LOWER(pl.status) IN ('down', 'timeout', 'failed', 'error') THEN 1 ELSE 0 END) as failed_polls
		FROM devices d
		LEFT JOIN polling_logs pl ON d.id = pl.device_id AND pl.created_at >= ? AND pl.created_at <= ?
		WHERE d.enabled = 1
		GROUP BY d.id, d.name, d.ip_address
		ORDER BY avg_latency DESC
	`

	ctx, cancel := context.WithTimeout(ctx, db.config.ConnectionTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query network quality report: %w", err)
	}
	defer rows.Close()

	var items []map[string]interface{}
	var sumLatency float64
	var sumLoss float64
	var count int

	for rows.Next() {
		var name, ip sql.NullString
		var avgLatency sql.NullFloat64
		var totalPolls, failedPolls sql.NullInt64

		if err := rows.Scan(&name, &ip, &avgLatency, &totalPolls, &failedPolls); err != nil {
			continue
		}

		packetLoss := 0.0
		if totalPolls.Int64 > 0 {
			packetLoss = (float64(failedPolls.Int64) / float64(totalPolls.Int64)) * 100
		}

		latencyMs := avgLatency.Float64

		items = append(items, map[string]interface{}{
			"name":        name.String,
			"ip":          ip.String,
			"latency":     fmt.Sprintf("%.1f ms", latencyMs),
			"packet_loss": fmt.Sprintf("%.2f%%", packetLoss),
		})

		sumLatency += latencyMs
		sumLoss += packetLoss
		count++
	}

	summary := map[string]interface{}{
		"Total Perangkat":       count,
		"Rata-rata Latency":     "0 ms",
		"Rata-rata Packet Loss": "0%",
	}

	if count > 0 {
		summary["Rata-rata Latency"] = fmt.Sprintf("%.1f ms", sumLatency/float64(count))
		summary["Rata-rata Packet Loss"] = fmt.Sprintf("%.2f%%", sumLoss/float64(count))
	}

	return map[string]interface{}{
		"summary": summary,
		"items":   items,
	}, nil
}

// GetSLAReport generates SLA compliance report
func (db *Database) GetSLAReport(ctx context.Context, start time.Time, end time.Time) (map[string]interface{}, error) {
	return db.GetAvailabilityReport(ctx, start, end)
}
