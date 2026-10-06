package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println(" NMS - BASELINE METRICS COLLECTOR (PHASE 0)  ")
	fmt.Println("==================================================")
	fmt.Println("Collecting data from local MySQL (nms_db)...")
	fmt.Println()

	// Sesuaikan dengan konfigurasi DB Anda di config.yaml
	dsn := "root:@tcp(127.0.0.1:3306)/nms_db?parseTime=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("❌ Failed to initialize database: %v\n", err)
	}
	defer db.Close()

	// Test connection
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	err = db.Ping()
	if err != nil {
		log.Fatalf("❌ Failed to connect to database. Make sure MySQL is running!\nError: %v\n", err)
	}
	fmt.Println("✅ Database connection successful.")
	fmt.Println()

	// 1. Get DB Size
	var dbSizeMB float64
	querySize := `
		SELECT ROUND(SUM(data_length + index_length) / 1024 / 1024, 2) 
		FROM information_schema.tables 
		WHERE table_schema = 'nms_db'`

	err = db.QueryRow(querySize).Scan(&dbSizeMB)
	if err != nil {
		dbSizeMB = 0.0
	}

	// 2. Get table row counts
	tables := []string{"devices", "device_metrics", "interface_metrics", "polling_logs", "alerts"}
	counts := make(map[string]int)

	for _, table := range tables {
		var count int
		// Warning: Ini query count dinamis, aman di sini karena nama tabel statis
		err = db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			count = -1 // Table not found or error
		}
		counts[table] = count
	}

	// 3. Get Active Connections
	var activeConns int
	err = db.QueryRow("SHOW STATUS WHERE `variable_name` = 'Threads_connected'").Scan(new(string), &activeConns)
	if err != nil {
		activeConns = -1
	}

	fmt.Println("=== 📊 DATABASE METRICS ===")
	fmt.Printf("- DB Size (nms_db)      : %.2f MB\n", dbSizeMB)
	fmt.Printf("- Active Connections    : %d\n", activeConns)
	fmt.Println("- Row Counts:")
	for _, table := range tables {
		if counts[table] == -1 {
			fmt.Printf("  * %-20s: [Tabel tidak ditemukan / Error]\n", table)
		} else {
			fmt.Printf("  * %-20s: %d rows\n", table, counts[table])
		}
	}

	fmt.Println()
	fmt.Println("✅ Silakan salin angka-angka di atas ke dalam file docs/PHASE_0_BASELINE.md")
	fmt.Println("   Untuk metrik CPU dan RAM, Anda bisa memantaunya via Task Manager (Windows) atau 'top' (Linux) saat agent berjalan.")
	fmt.Println("==================================================")
}

