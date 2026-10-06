package monitoring

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"gopkg.in/routeros.v2"
)

type VPNUser struct {
	Name     string `json:"name"`
	Service  string `json:"service"`
	CallerID string `json:"caller_id"`
	Address  string `json:"address"`
	Uptime   string `json:"uptime"`
}

var (
	vpnCache      []VPNUser
	vpnCacheTime  time.Time
	vpnCacheMutex sync.Mutex
)

func VPNUsersHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if cfg.RouterOS.Address == "" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Konfigurasi RouterOS API belum diatur di config.yaml",
				"users":   []VPNUser{},
			})
			return
		}

		vpnCacheMutex.Lock()
		if time.Since(vpnCacheTime) < 15*time.Second && vpnCache != nil {
			users := vpnCache
			vpnCacheMutex.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"users":   users,
			})
			return
		}
		vpnCacheMutex.Unlock()

		// Connect to Mikrotik API
		client, err := routeros.DialTimeout(cfg.RouterOS.Address, cfg.RouterOS.Username, cfg.RouterOS.Password, 5*time.Second)
		if err != nil {
			log.Printf("Error connecting to Mikrotik API (%s): %v", cfg.RouterOS.Address, err)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Tidak dapat terhubung ke Router Mikrotik (%s). Pastikan port API (8728) aktif dan IP dapat dijangkau dari jaringan ini.", cfg.RouterOS.Address),
				"users":   []VPNUser{},
			})
			return
		}
		defer client.Close()

		// Run /ppp/active/print
		reply, err := client.Run("/ppp/active/print")
		if err != nil {
			log.Printf("Error running /ppp/active/print: %v", err)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Gagal mengeksekusi perintah /ppp/active/print pada Mikrotik",
				"users":   []VPNUser{},
			})
			return
		}

		var users []VPNUser
		for _, re := range reply.Re {
			user := VPNUser{
				Name:     re.Map["name"],
				Service:  re.Map["service"],
				CallerID: re.Map["caller-id"],
				Address:  re.Map["address"],
				Uptime:   re.Map["uptime"],
			}
			users = append(users, user)
		}

		if users == nil {
			users = []VPNUser{}
		}

		vpnCacheMutex.Lock()
		vpnCache = users
		vpnCacheTime = time.Now()
		vpnCacheMutex.Unlock()

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"users":   users,
		})
	}
}
