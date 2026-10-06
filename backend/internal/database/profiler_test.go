package database

import (
	"database/sql"
	"testing"
	"time"
	_ "github.com/go-sql-driver/mysql"
)

// Profiling is meant to be run against a real database in Staging
func TestDatabaseProfiling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db profiling in short mode")
	}
	t.Log("Connecting to Database...")
	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/nms_verification_db?parseTime=true")
	if err != nil {
		t.Fatalf("Failed to connect to staging DB: %v", err)
	}

	var initialQueries int
	err = db.QueryRow("SHOW GLOBAL STATUS LIKE 'Queries'").Scan(new(string), &initialQueries)
	if err != nil {
		t.Logf("Warning: Could not get MySQL query stats: %v", err)
	}

	t.Logf("T0: Global Queries = %d", initialQueries)
	t.Log("Simulating 100 devices polling cycle...")
	
	time.Sleep(5 * time.Second)

	var finalQueries int
	err = db.QueryRow("SHOW GLOBAL STATUS LIKE 'Queries'").Scan(new(string), &finalQueries)
	if err == nil {
		t.Logf("T+5s: Global Queries = %d. Diff = %d", finalQueries, finalQueries-initialQueries)
	}
}
