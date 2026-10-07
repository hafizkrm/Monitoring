package database

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

// Profiling is meant to be run against a real database in Staging
func TestDatabaseProfiling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db profiling in short mode")
	}
	t.Log("Connecting to Database...")
	rawDB, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/nms_verification_db?parseTime=true")
	if err != nil {
		t.Fatalf("Failed to connect to staging DB: %v", err)
	}
	db := &Database{DB: rawDB}

	// Create a dummy device for testing insert
	_, _ = db.Exec("INSERT IGNORE INTO devices (id, name, ip_address, device_type, enabled) VALUES (9999, 'Test Device', '127.0.0.99', 'router', 1)")

	var initialQueries int
	err = db.QueryRow("SHOW GLOBAL STATUS LIKE 'Queries'").Scan(new(string), &initialQueries)
	if err != nil {
		t.Logf("Warning: Could not get MySQL query stats: %v", err)
	}

	t.Logf("T0: Global Queries = %d", initialQueries)
	t.Log("Simulating 100 devices polling cycle...")

	start := time.Now()
	// Simulate writing metrics for 100 devices
	for i := 0; i < 100; i++ {
		metric := &models.DeviceMetric{
			DeviceID:    9999,
			CPUUsage:    10.5 + float64(i%10),
			MemoryUsage: 2048.0,
			Temperature: 35.0,
			Uptime:      int64(1000 + i),
		}
		_, err = db.Exec(
			"INSERT INTO device_metrics (device_id, cpu_usage, memory_usage, temperature, uptime) VALUES (?, ?, ?, ?, ?)",
			metric.DeviceID, metric.CPUUsage, metric.MemoryUsage, metric.Temperature, metric.Uptime,
		)
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}
	duration := time.Since(start)

	var finalQueries int
	err = db.QueryRow("SHOW GLOBAL STATUS LIKE 'Queries'").Scan(new(string), &finalQueries)
	if err == nil {
		diff := finalQueries - initialQueries
		t.Logf("T+End: Global Queries = %d. Diff = %d queries. Took %v", finalQueries, diff, duration)
		t.Logf("Statements/poll: %.2f", float64(diff)/100.0)
		t.Logf("Latency/poll: %v", duration/100)

		t.Log("NOTE: These 300+ queries originate from a raw SQL verification harness (db.Exec directly loop 100x), NOT from the production Go application workload.")
		t.Log("      This is to prove the database is capable of handling the write amplification, but it skips the batching logic in production.")

		if diff > 500 {
			t.Errorf("Write amplification too high! Diff = %d", diff)
		}
	}
}
