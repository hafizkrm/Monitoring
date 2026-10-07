package incidents

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"time"

	nms_middleware "github.com/hafizkrm/Monitoring/backend/internal/api/middleware"
	"github.com/hafizkrm/Monitoring/backend/internal/database"
)

func getUserInfoFromContext(r *http.Request) (*int64, string) {
	var userID *int64
	if idRaw := r.Context().Value(nms_middleware.UserContextKey); idRaw != nil {
		id := int64(idRaw.(int))
		userID = &id
	}
	username := "Admin"
	if nameRaw := r.Context().Value(nms_middleware.UserNameContextKey); nameRaw != nil {
		username = nameRaw.(string)
	}
	return userID, username
}

type Incident struct {
	ID          int64      `json:"id"`
	DeviceID    int        `json:"device_id"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Description string     `json:"description"`
	StartedAt   time.Time  `json:"started_at"`
	ResolvedAt  *time.Time `json:"resolved_at"`
}

type Alert struct {
	ID             int64      `json:"id"`
	IncidentID     *int64     `json:"incident_id"`
	Type           string     `json:"type"`
	Message        string     `json:"message"`
	IsRead         bool       `json:"is_read"`
	CreatedAt      time.Time  `json:"created_at"`
	DeviceName     string     `json:"device_name"`
	DeviceIP       string     `json:"device_ip"`
	IncidentStatus string     `json:"incident_status"`
	ResolvedAt     *time.Time `json:"resolved_at"`
}

// GetActiveIncidents handles fetching active incidents
func GetActiveIncidents(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		query := `SELECT id, device_id, type, status, COALESCE(description, ''), started_at, resolved_at FROM incidents WHERE status = 'active' ORDER BY started_at DESC`
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			log.Printf("[ERROR] GetActiveIncidents query error: %v", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]Incident{})
			return
		}
		defer rows.Close()

		var incidents []Incident
		for rows.Next() {
			var id int64
			var devID sql.NullInt64
			var incType, status, desc sql.NullString
			var startedAt, resolvedAt sql.NullTime

			if err := rows.Scan(&id, &devID, &incType, &status, &desc, &startedAt, &resolvedAt); err != nil {
				log.Printf("[ERROR] GetActiveIncidents scan error: %v", err)
				continue
			}

			var resTime *time.Time
			if resolvedAt.Valid {
				resTime = &resolvedAt.Time
			}

			incidents = append(incidents, Incident{
				ID:          id,
				DeviceID:    int(devID.Int64),
				Type:        incType.String,
				Status:      status.String,
				Description: desc.String,
				StartedAt:   startedAt.Time,
				ResolvedAt:  resTime,
			})
		}

		if incidents == nil {
			incidents = make([]Incident, 0)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(incidents)
	}
}

// GetAlerts handles fetching both active and resolved alerts for UI
func GetAlerts(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		query := `
			SELECT a.id, a.incident_id, COALESCE(a.type, 'warning'), COALESCE(a.message, ''), COALESCE(a.is_read, 0), COALESCE(a.created_at, CURRENT_TIMESTAMP), 
			       COALESCE(NULLIF(d.name, ''), 'Device') as device_name, 
			       COALESCE(NULLIF(d.ip_address, ''), '') as device_ip,
			       COALESCE(i.status, 'active') as incident_status,
			       i.resolved_at
			FROM alerts a 
			LEFT JOIN incidents i ON a.incident_id = i.id 
			LEFT JOIN devices d ON d.id = i.device_id
			ORDER BY a.created_at DESC LIMIT 200
		`
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			log.Printf("[ERROR] GetAlerts query error: %v", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]Alert{})
			return
		}
		defer rows.Close()

		var alerts []Alert
		for rows.Next() {
			var id int64
			var incidentID sql.NullInt64
			var alertType, message, devName, devIP, incStatus sql.NullString
			var isRead sql.NullBool
			var createdAt sql.NullTime
			var resolvedAt sql.NullTime

			if err := rows.Scan(&id, &incidentID, &alertType, &message, &isRead, &createdAt, &devName, &devIP, &incStatus, &resolvedAt); err != nil {
				log.Printf("[ERROR] GetAlerts scan error: %v", err)
				continue
			}

			var incID *int64
			if incidentID.Valid {
				incID = &incidentID.Int64
			}

			var resAt *time.Time
			if resolvedAt.Valid {
				resAt = &resolvedAt.Time
			}

			finalIP := devIP.String
			if finalIP == "" {
				if m := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`).FindString(message.String); m != "" {
					finalIP = m
				}
			}

			alerts = append(alerts, Alert{
				ID:             id,
				IncidentID:     incID,
				Type:           alertType.String,
				Message:        message.String,
				IsRead:         isRead.Bool,
				CreatedAt:      createdAt.Time,
				DeviceName:     devName.String,
				DeviceIP:       finalIP,
				IncidentStatus: incStatus.String,
				ResolvedAt:     resAt,
			})
		}

		if alerts == nil {
			alerts = make([]Alert, 0)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alerts)
	}
}

// MarkAlertRead marks an alert as read
func MarkAlertRead(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing alert ID", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		query := `UPDATE alerts SET is_read = 1 WHERE id = ?`
		_, err := db.ExecContext(ctx, query, id)
		if err != nil {
			http.Error(w, "Failed to update alert", http.StatusInternalServerError)
			return
		}

		userID, username := getUserInfoFromContext(r)
		_ = db.InsertActivityLog(r.Context(), userID, username, "UPDATE", "Alerts", "Acknowledged alert ID: "+id, r.RemoteAddr)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}
}

// MarkAllAlertsRead marks all unread alerts as read
func MarkAllAlertsRead(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		query := `UPDATE alerts SET is_read = 1 WHERE is_read = 0 OR is_read IS NULL`
		_, err := db.ExecContext(ctx, query)
		if err != nil {
			http.Error(w, "Failed to update alerts", http.StatusInternalServerError)
			return
		}

		queryInc := `UPDATE incidents SET status = 'resolved', resolved_at = CURRENT_TIMESTAMP WHERE status = 'active'`
		_, _ = db.ExecContext(ctx, queryInc)

		userID, username := getUserInfoFromContext(r)
		_ = db.InsertActivityLog(r.Context(), userID, username, "UPDATE", "Alerts", "Acknowledged all alerts", r.RemoteAddr)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}
}

// ResolveAlertsByIP resolves incidents & marks alerts read for a specific target IP
func ResolveAlertsByIP(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.URL.Query().Get("ip")
		if ip == "" {
			var body struct {
				IP string `json:"ip"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.IP != "" {
				ip = body.IP
			}
		}
		if ip == "" {
			http.Error(w, "Missing ip parameter", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := db.ResolveOfflineIncidentByIP(ctx, ip); err != nil {
			http.Error(w, "Failed to resolve alert", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}
}
