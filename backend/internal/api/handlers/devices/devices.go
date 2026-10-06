package devices

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	nms_middleware "github.com/hafizkrm/Monitoring/backend/internal/api/middleware"
	"github.com/hafizkrm/Monitoring/backend/internal/database"
)

var validHostRegex = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)

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

type AddDeviceReq struct {
	IP   string `json:"ip"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func AddDeviceHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req AddDeviceReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.IP == "" || req.Name == "" {
			http.Error(w, "IP and Name are required", http.StatusBadRequest)
			return
		}
		vendor, err := db.AddDeviceWithBrand(r.Context(), req.IP, req.Name, req.Type)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		userID, username := getUserInfoFromContext(r)
		_ = db.InsertActivityLog(r.Context(), userID, username, "CREATE", "Device", fmt.Sprintf("Menambahkan perangkat baru %s dengan IP %s", req.Name, req.IP), r.RemoteAddr)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "vendor": vendor})
	}
}

type DeleteDeviceReq struct {
	IP string `json:"ip"`
}

func DeleteDeviceHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req DeleteDeviceReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := db.DeleteDevice(r.Context(), req.IP); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		userID, username := getUserInfoFromContext(r)
		_ = db.InsertActivityLog(r.Context(), userID, username, "DELETE", "Device", fmt.Sprintf("Menghapus perangkat dengan IP %s", req.IP), r.RemoteAddr)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

type UpdateDeviceReq struct {
	OldIP    string `json:"old_ip"`
	NewIP    string `json:"new_ip"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	ParentIP string `json:"parent_ip"`
}

func UpdateDeviceHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req UpdateDeviceReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := db.UpdateDevice(r.Context(), req.OldIP, req.NewIP, req.Name, req.Type, req.ParentIP); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		userID, username := getUserInfoFromContext(r)
		_ = db.InsertActivityLog(r.Context(), userID, username, "UPDATE", "Device", fmt.Sprintf("Mengubah data perangkat %s (%s)", req.Name, req.NewIP), r.RemoteAddr)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func InventoryHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		// Parse pagination parameters
		pageStr := r.URL.Query().Get("page")
		limitStr := r.URL.Query().Get("limit")
		search := r.URL.Query().Get("search")
		deviceType := r.URL.Query().Get("type")

		page := 1
		limit := 10
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}

		devices, total, err := db.GetPaginatedDevices(r.Context(), page, limit, search, deviceType)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Failed to fetch devices"})
			return
		}

		out := make([]map[string]interface{}, 0, len(devices))
		for _, d := range devices {
			out = append(out, map[string]interface{}{
				"id":             d.ID,
				"name":           d.Name,
				"ip_address":     d.IPAddress,
				"device_type":    d.DeviceType,
				"enabled":        d.Enabled,
				"status":         d.Status,
				"parent_ip":      d.ParentIP,
				"last_polled_at": d.LastPolledAt,
				"created_at":     d.CreatedAt,
			})
		}

		response := map[string]interface{}{
			"data":  out,
			"total": total,
			"page":  page,
			"limit": limit,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func InterfacesHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		ip := r.URL.Query().Get("ip")
		if ip == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "ip parameter required"})
			return
		}

		interfaces, err := db.GetInterfaceMetricsByIP(r.Context(), ip)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
			return
		}

		if err := json.NewEncoder(w).Encode(interfaces); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func streamCommandOutput(w http.ResponseWriter, r *http.Request, cmdName string, args ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Stderr = cmd.Stdout

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(w, "data: Error: %v\n\n", err)
		flusher.Flush()
		return
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(w, "data: Error starting %s: %v\n\n", cmdName, err)
		flusher.Flush()
		return
	}

	// Flush immediately so client gets SSE headers right away
	flusher.Flush()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.TrimSpace(text) != "" {
			fmt.Fprintf(w, "data: %s\n\n", text)
			flusher.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(w, "data: Error reading output: %v\n\n", err)
		flusher.Flush()
	}

	cmd.Wait()
	fmt.Fprintf(w, "data: [Process Completed]\n\n")
	flusher.Flush()
}

func PingHandler(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	if ip == "" || !validHostRegex.MatchString(ip) {
		http.Error(w, "Valid ip parameter required", http.StatusBadRequest)
		return
	}

	if runtime.GOOS == "windows" {
		streamCommandOutput(w, r, "ping", "-n", "4", "-w", "1000", ip)
	} else {
		streamCommandOutput(w, r, "ping", "-c", "4", "-W", "1", ip)
	}
}

func TraceHandler(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	if ip == "" || !validHostRegex.MatchString(ip) {
		http.Error(w, "Valid ip parameter required", http.StatusBadRequest)
		return
	}

	if runtime.GOOS == "windows" {
		streamCommandOutput(w, r, "tracert", "-d", "-h", "15", "-w", "1000", ip)
	} else {
		streamCommandOutput(w, r, "traceroute", "-n", "-m", "15", "-w", "1", ip)
	}
}

func InventoryStatsHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		stats, err := db.GetInventoryStats(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Failed to fetch stats"})
			return
		}

		if err := json.NewEncoder(w).Encode(stats); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}
