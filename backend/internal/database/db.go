package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/hafizkrm/Monitoring/backend/internal/logger"
)

// Database wraps sql.DB with additional functionality
type Database struct {
	*sql.DB
	config        DatabaseConfig
	logger        logger.Logger
	encryptionKey string
}

type DatabaseConfig struct {
	Host               string
	Port               int
	User               string
	Password           string
	Database           string
	MaxConnections     int
	MaxIdleConnections int
	MaxOpenConnections int
	ConnectionTimeout  time.Duration
	ConnMaxLifetime    time.Duration
	EncryptionKey      string
}

// NewDatabase creates a new database connection
func NewDatabase(cfg config.DatabaseConfig, log logger.Logger) (*Database, error) {
	params := cfg.Params
	if !strings.Contains(params, "multiStatements=true") {
		if params == "" {
			params = "multiStatements=true"
		} else {
			params += "&multiStatements=true"
		}
	}

	// Build DSN (Data Source Name)
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		params,
	)

	dbCfg := DatabaseConfig{
		Host:               cfg.Host,
		Port:               cfg.Port,
		User:               cfg.User,
		Password:           cfg.Password,
		Database:           cfg.Database,
		MaxConnections:     cfg.MaxConnections,
		MaxIdleConnections: cfg.MaxIdleConnections,
		MaxOpenConnections: cfg.MaxOpenConnections,
		ConnectionTimeout:  cfg.GetConnectionTimeout(),
		ConnMaxLifetime:    time.Minute * 3, // Force refresh every 3 minutes to avoid MySQL wait_timeout drops
		EncryptionKey:      os.Getenv("ENCRYPTION_KEY"),
	}

	// Open database connection
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(dbCfg.MaxIdleConnections)
	sqlDB.SetMaxOpenConns(dbCfg.MaxOpenConnections)
	sqlDB.SetConnMaxLifetime(dbCfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(time.Minute * 1) // Phase 3: Enforce Idle Time limit

	dbWrapper := &Database{
		DB:            sqlDB,
		config:        dbCfg,
		logger:        log,
		encryptionKey: dbCfg.EncryptionKey,
	}

	// Test connection synchronously
	ctx, cancel := context.WithTimeout(context.Background(), dbCfg.ConnectionTimeout)
	err = sqlDB.PingContext(ctx)
	cancel()

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Info("Successfully connected to database", map[string]interface{}{
		"database": dbCfg.Database,
		"host":     dbCfg.Host,
		"port":     dbCfg.Port,
	})

	// Run migrations (Synchronously, fails startup if it fails)
	// P1: Hilangkan runtime schema repair, gunakan migrasi terpusat
	// NOTE: Migrations are now handled by database.Migrate() in main.go
	// to avoid conflicting with the old migration logic.

	return dbWrapper, nil
}

// Health checks the database connection
func (db *Database) Health(ctx context.Context) error {
	return db.PingContext(ctx)
}

// Close closes the database connection
func (db *Database) Close() error {
	return db.DB.Close()
}

// ExecuteWithTimeout executes a query with timeout
func (db *Database) ExecuteWithTimeout(ctx context.Context, timeout time.Duration, query string, args ...interface{}) (sql.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return db.ExecContext(ctx, query, args...)
}

// QueryWithTimeout queries with timeout and executes a callback with the resulting rows.
func (db *Database) QueryWithTimeout(ctx context.Context, timeout time.Duration, fn func(*sql.Rows) error, query string, args ...interface{}) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rows, err := db.QueryContext(timeoutCtx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	return fn(rows)
}

// QueryRowWithTimeout queries a single row with timeout and executes a callback.
func (db *Database) QueryRowWithTimeout(ctx context.Context, timeout time.Duration, fn func(*sql.Row) error, query string, args ...interface{}) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	row := db.QueryRowContext(timeoutCtx, query, args...)
	return fn(row)
}
