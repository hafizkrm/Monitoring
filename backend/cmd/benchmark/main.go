//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := "root:@tcp(127.0.0.1:3306)/nms_db?parseTime=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Gagal connect ke DB: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("DB belum siap: %v", err)
	}

	fmt.Println("Menghapus devices lama...")
	db.Exec("DELETE FROM devices")

	fmt.Println("Menyuntikkan 5000 perangkat benchmark...")
	
	total := 5000
	batchSize := 500

	for i := 0; i < total; i += batchSize {
		query := "INSERT INTO devices (name, ip_address, device_type, vendor, enabled, created_at, updated_at) VALUES "
		var vals []interface{}
		
		for j := 0; j < batchSize; j++ {
			id := i + j + 1
			query += "(?, ?, ?, ?, ?, ?, ?),"
			vals = append(vals, 
				fmt.Sprintf("Bench-Device-%d", id),
				fmt.Sprintf("10.100.%d.%d", id/250, id%250),
				"Switch",
				"Generic",
				true,
				time.Now(),
				time.Now(),
			)
		}
		
		query = query[:len(query)-1] // Hapus koma terakhir
		
		_, err := db.Exec(query, vals...)
		if err != nil {
			log.Fatalf("Gagal insert batch: %v", err)
		}
		fmt.Printf("Inserted %d/%d...\n", i+batchSize, total)
	}

	fmt.Println("5000 perangkat berhasil ditambahkan! Silakan jalankan backend agent.")
}
