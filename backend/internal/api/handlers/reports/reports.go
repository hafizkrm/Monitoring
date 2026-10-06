package reports

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/database"
)

func ReportsHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		reportType := r.URL.Query().Get("type")
		reportRange := r.URL.Query().Get("range")

		if reportType == "" {
			reportType = "availability"
		}
		if reportRange == "" {
			reportRange = "7days"
		}

		// Calculate start and end times
		endTime := time.Now()
		var startTime time.Time

		switch reportRange {
		case "today":
			// Start of today
			now := time.Now()
			startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		case "30days":
			startTime = endTime.AddDate(0, 0, -30)
		case "custom":
			startDateStr := r.URL.Query().Get("start_date")
			endDateStr := r.URL.Query().Get("end_date")

			if startDateStr != "" {
				if parsedStart, err := time.Parse("2006-01-02", startDateStr); err == nil {
					startTime = parsedStart
				} else {
					startTime = endTime.AddDate(0, 0, -7)
				}
			} else {
				startTime = endTime.AddDate(0, 0, -7)
			}

			if endDateStr != "" {
				if parsedEnd, err := time.Parse("2006-01-02", endDateStr); err == nil {
					endTime = time.Date(parsedEnd.Year(), parsedEnd.Month(), parsedEnd.Day(), 23, 59, 59, 999999999, parsedEnd.Location())
				}
			}
		case "7days":
			fallthrough
		default:
			startTime = endTime.AddDate(0, 0, -7)
		}

		var data interface{}
		var err error

		switch reportType {
		case "availability":
			data, err = db.GetAvailabilityReport(r.Context(), startTime, endTime)
		case "performance":
			data, err = db.GetPerformanceReport(r.Context(), startTime, endTime)
		case "incidents":
			data, err = db.GetIncidentsReport(r.Context(), startTime, endTime)
		case "down_frequency":
			data, err = db.GetDownFrequencyReport(r.Context(), startTime, endTime)
		case "traffic_monthly":
			data, err = db.GetTrafficMonthlyReport(r.Context(), startTime, endTime)
		case "downtime_monthly":
			data, err = db.GetDowntimeMonthlyReport(r.Context(), startTime, endTime)
		case "vpn_monthly":
			data, err = db.GetVPNMonthlyReport(r.Context(), startTime, endTime)
		case "bandwidth_top":
			data, err = db.GetBandwidthTopTalkersReport(r.Context(), startTime, endTime)
		case "network_quality":
			data, err = db.GetNetworkQualityReport(r.Context(), startTime, endTime)
		case "sla":
			data, err = db.GetSLAReport(r.Context(), startTime, endTime)
		default:
			http.Error(w, "Invalid report type", http.StatusBadRequest)
			return
		}

		if err != nil {
			// If error, return empty format
			emptyRes := map[string]interface{}{
				"summary": map[string]interface{}{},
				"items":   []interface{}{},
			}
			json.NewEncoder(w).Encode(emptyRes)
			return
		}

		if err := json.NewEncoder(w).Encode(data); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}
