package worker

import (
	"context"
	"log"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/hafizkrm/Monitoring/backend/internal/database"
	"github.com/hafizkrm/Monitoring/backend/internal/logger"
	"gopkg.in/routeros.v2"
)

// StartVPNLogger starts a background goroutine to poll and log VPN connections every 5 minutes
func StartVPNLogger(ctx context.Context, cfg *config.Config, db *database.Database, appLog logger.Logger) {
	if cfg.RouterOS.Address == "" {
		appLog.Warn("RouterOS API config not set, VPN Logger will not start", nil)
		return
	}

	appLog.Info("Starting VPN Logger (polling every 5 minutes)", nil)

	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		// Run once immediately
		logVPNUsers(cfg, db, appLog)

		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				appLog.Info("Stopping VPN Logger", nil)
				return
			case <-ticker.C:
				logVPNUsers(cfg, db, appLog)
			}
		}
	}()
}

func logVPNUsers(cfg *config.Config, db *database.Database, appLog logger.Logger) {
	client, err := routeros.DialTimeout(cfg.RouterOS.Address, cfg.RouterOS.Username, cfg.RouterOS.Password, 5*time.Second)
	if err != nil {
		appLog.Error("VPN Logger: Error connecting to Mikrotik API", map[string]interface{}{"error": err})
		return
	}
	defer client.Close()

	reply, err := client.Run("/ppp/active/print")
	if err != nil {
		appLog.Error("VPN Logger: Error running /ppp/active/print", map[string]interface{}{"error": err})
		return
	}

	for _, re := range reply.Re {
		name := re.Map["name"]
		service := re.Map["service"]
		callerID := re.Map["caller-id"]
		address := re.Map["address"]
		uptime := re.Map["uptime"]

		if name == "" {
			continue
		}

		// Insert into db
		query := `
			INSERT INTO vpn_connections (name, service, caller_id, address, uptime) 
			VALUES (?, ?, ?, ?, ?)
		`
		_, err := db.Exec(query, name, service, callerID, address, uptime)
		if err != nil {
			log.Printf("VPN Logger: Failed to insert user %s: %v", name, err)
		}
	}
}
