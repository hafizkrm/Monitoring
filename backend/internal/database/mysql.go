package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/yourusername/viscod/internal/config"
)

// Connect membuat koneksi pool ke MySQL
func Connect(cfg config.DatabaseConfig) (*sql.DB, error) {
	// 1. Connect tanpa nama database untuk membuat database jika belum ada
	baseDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/?%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Params)
	tempDB, err := sql.Open("mysql", baseDSN)
	if err == nil {
		_, err = tempDB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", cfg.Database))
		if err != nil {
			log.Printf("Warning: failed to create database %s: %v", cfg.Database, err)
		}
		tempDB.Close()
	}

	// 2. Connect ke database yang sebenarnya
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.Params,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConnections)
	db.SetConnMaxLifetime(3 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error pinging database: %w", err)
	}

	log.Printf("Successfully connected to MySQL database: %s", cfg.Database)
	return db, nil
}
