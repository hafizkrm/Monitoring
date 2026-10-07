package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func setupTestDB(t *testing.T) *Database {
	rawDB, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/nms_verification_db?parseTime=true")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Make sure table exists
	_, _ = rawDB.Exec("CREATE TABLE IF NOT EXISTS device_metrics (id INT AUTO_INCREMENT PRIMARY KEY, device_id INT, collected_at DATETIME, cpu_usage FLOAT)")
	_, _ = rawDB.Exec("TRUNCATE TABLE device_metrics")

	return &Database{DB: rawDB}
}

func TestCleanupOldData_BoundaryAndBatching(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	db := setupTestDB(t)
	defer db.Close()

	// Insert 12,000 old records (older than 30 days) and 1,000 new records
	oldDate := time.Now().Add(-35 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	newDate := time.Now().Format("2006-01-02 15:04:05")

	for i := 0; i < 12000; i += 1000 {
		var vals string
		for j := 0; j < 1000; j++ {
			if j > 0 {
				vals += ","
			}
			vals += fmt.Sprintf("(1, '%s', 50)", oldDate)
		}
		_, err := db.Exec(fmt.Sprintf("INSERT INTO device_metrics (device_id, collected_at, cpu_usage) VALUES %s", vals))
		if err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	// Insert new records
	var vals string
	for j := 0; j < 1000; j++ {
		if j > 0 {
			vals += ","
		}
		vals += fmt.Sprintf("(1, '%s', 50)", newDate)
	}
	_, err := db.Exec(fmt.Sprintf("INSERT INTO device_metrics (device_id, collected_at, cpu_usage) VALUES %s", vals))
	if err != nil {
		t.Fatalf("insert new failed: %v", err)
	}

	// Run cleanup
	ctx := context.Background()
	stats, err := db.CleanupOldData(ctx, 30, 30)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	if stats["device_metrics"] != 12000 {
		t.Errorf("Expected 12000 rows deleted, got %d", stats["device_metrics"])
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM device_metrics").Scan(&count)
	if count != 1000 {
		t.Errorf("Expected 1000 new records to remain, got %d", count)
	}
}

func TestCleanupOldData_Cancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	db := setupTestDB(t)
	defer db.Close()

	// Insert 20,000 old records
	oldDate := time.Now().Add(-35 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	for i := 0; i < 20000; i += 1000 {
		var vals string
		for j := 0; j < 1000; j++ {
			if j > 0 {
				vals += ","
			}
			vals += fmt.Sprintf("(1, '%s', 50)", oldDate)
		}
		db.Exec(fmt.Sprintf("INSERT INTO device_metrics (device_id, collected_at, cpu_usage) VALUES %s", vals))
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 20ms (should interrupt the batching)
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	stats, err := db.CleanupOldData(ctx, 30, 30)
	if err == nil {
		t.Errorf("Expected context cancellation error, got nil")
	}

	// Should not have deleted all 20,000 rows
	var count int
	db.QueryRow("SELECT COUNT(*) FROM device_metrics").Scan(&count)
	if count == 0 {
		t.Errorf("Expected some records to remain due to cancellation, but got %d", count)
	}
	t.Logf("Cancellation successful, %d rows remain, stats: %v", count, stats)
}

func TestCleanupOldData_ConcurrentInsert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	db := setupTestDB(t)
	defer db.Close()

	// Insert 15,000 old records
	oldDate := time.Now().Add(-35 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	for i := 0; i < 15000; i += 1000 {
		var vals string
		for j := 0; j < 1000; j++ {
			if j > 0 {
				vals += ","
			}
			vals += fmt.Sprintf("(1, '%s', 50)", oldDate)
		}
		db.Exec(fmt.Sprintf("INSERT INTO device_metrics (device_id, collected_at, cpu_usage) VALUES %s", vals))
	}

	var wg sync.WaitGroup
	ctx := context.Background()

	// Goroutine 1: Cleanup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := db.CleanupOldData(ctx, 30, 30)
		if err != nil {
			t.Errorf("Concurrent cleanup failed: %v", err)
		}
	}()

	// Goroutine 2: Concurrent Inserts
	wg.Add(1)
	newDate := time.Now().Format("2006-01-02 15:04:05")
	insertSuccess := 0
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, err := db.Exec("INSERT INTO device_metrics (device_id, collected_at, cpu_usage) VALUES (1, ?, 50)", newDate)
			if err != nil {
				t.Errorf("Concurrent insert failed (lock?): %v", err)
			} else {
				insertSuccess++
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()

	wg.Wait()

	if insertSuccess != 50 {
		t.Errorf("Expected 50 successful concurrent inserts, got %d", insertSuccess)
	}
}
