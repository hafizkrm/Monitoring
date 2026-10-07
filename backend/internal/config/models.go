package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure
type Config struct {
	App      AppConfig      `yaml:"app"`
	Database DatabaseConfig `yaml:"database"`
	RouterOS RouterOSConfig `yaml:"routeros"`
	SNMP     SNMPConfig     `yaml:"snmp"`
	Polling  PollingConfig  `yaml:"polling"`
	PromQL   PromQLConfig   `yaml:"promql"`
	Logger   LoggerConfig   `yaml:"logger"`
}

// PromQLConfig holds PromQL hardening parameters
type PromQLConfig struct {
	MaxRange          time.Duration `yaml:"max_range"`
	MaxResultSize     int           `yaml:"max_result_size"`
	RateLimitRequests int           `yaml:"rate_limit_requests"`
	RateLimitWindow   time.Duration `yaml:"rate_limit_window"`
}

// AppConfig holds application-level configuration
type AppConfig struct {
	Name          string `yaml:"name"`
	Version       string `yaml:"version"`
	LogLevel      string `yaml:"log_level"` // DEBUG, INFO, WARN, ERROR
	Port          int    `yaml:"port"`      // For health check endpoint
	JWTSecret     string `yaml:"jwt_secret"`
	EncryptionKey string `yaml:"encryption_key"`
	TSDBUrl       string `yaml:"tsdb_url"`
}

// GetJWTSecret returns JWT secret, falling back to environment variable
func (a *AppConfig) GetJWTSecret() string {
	if a.JWTSecret != "" {
		return a.JWTSecret
	}
	return os.Getenv("JWT_SECRET")
}

// GetEncryptionKey returns encryption key, falling back to environment variable
func (a *AppConfig) GetEncryptionKey() string {
	if a.EncryptionKey != "" {
		return a.EncryptionKey
	}
	return os.Getenv("ENCRYPTION_KEY")
}

// GetTSDBUrl returns TSDB URL, falling back to environment variable, defaults to http://localhost:9090
func (a *AppConfig) GetTSDBUrl() string {
	if a.TSDBUrl != "" {
		return a.TSDBUrl
	}
	envUrl := os.Getenv("TSDB_URL")
	if envUrl != "" {
		return envUrl
	}
	return "http://localhost:9090"
}

// RouterOSConfig holds RouterOS API configuration
type RouterOSConfig struct {
	Address  string `yaml:"address"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// DatabaseConfig holds MySQL connection configuration
type DatabaseConfig struct {
	Host               string `yaml:"host"`
	Port               int    `yaml:"port"`
	User               string `yaml:"user"`
	Password           string `yaml:"password"`
	Database           string `yaml:"database"`
	MaxConnections     int    `yaml:"max_connections"`
	ConnectionTimeout  string `yaml:"connection_timeout"` // e.g., "10s"
	MaxIdleConnections int    `yaml:"max_idle_connections"`
	MaxOpenConnections int    `yaml:"max_open_connections"`
	Params             string `yaml:"params"`            // e.g., "parseTime=true&loc=Asia/Jakarta"
	ConnMaxLifetime    string `yaml:"conn_max_lifetime"` // e.g., "1h"
}

// GetConnectionTimeout returns database connection timeout as time.Duration
func (c *DatabaseConfig) GetConnectionTimeout() time.Duration {
	if c.ConnectionTimeout == "" {
		return 10 * time.Second // Default 10 seconds if not set
	}
	d, _ := time.ParseDuration(c.ConnectionTimeout)
	return d
}

// GetConnMaxLifetime returns database connection max lifetime as time.Duration
func (c *DatabaseConfig) GetConnMaxLifetime() time.Duration {
	if c.ConnMaxLifetime == "" {
		return time.Hour // Default 1 hour if not set
	}
	d, _ := time.ParseDuration(c.ConnMaxLifetime)
	return d
}

// SNMPConfig holds SNMP configuration
type SNMPConfig struct {
	Timeout    string `yaml:"timeout"`     // e.g., "5s"
	Retries    int    `yaml:"retries"`     // Default 3
	Community  string `yaml:"community"`   // Default "public"
	Version    string `yaml:"version"`     // "1", "2c", "3"
	Port       int    `yaml:"port"`        // Default 161
	UseContext bool   `yaml:"use_context"` // For timeout
}

// PollingConfig holds polling strategy configuration
type PollingConfig struct {
	DefaultInterval         string `yaml:"default_interval"`          // e.g., "60s"
	MaxWorkers              int    `yaml:"max_workers"`               // Max concurrent workers
	BatchInsertSize         int    `yaml:"batch_insert_size"`         // e.g., 100
	BatchInsertTimeout      string `yaml:"batch_insert_timeout"`      // e.g., "10s"
	HealthCheckInterval     string `yaml:"health_check_interval"`     // e.g., "30s"
	RateCalculationInterval string `yaml:"rate_calculation_interval"` // e.g., "30s"
	CleanupInterval         string `yaml:"cleanup_interval"`          // e.g., "24h"
	MetricsRetentionDays    int    `yaml:"metrics_retention_days"`    // Days to keep metrics data
	LogsRetentionDays       int    `yaml:"logs_retention_days"`       // Days to keep log data
}

// LoggerConfig holds logging configuration
type LoggerConfig struct {
	Level      string `yaml:"level"`       // DEBUG, INFO, WARN, ERROR
	Format     string `yaml:"format"`      // json, text
	OutputPath string `yaml:"output_path"` // e.g., "./var/log/nms-agent.log"
	MaxSize    int    `yaml:"max_size"`    // MB
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"` // Days
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := &Config{}
	expandedData := os.ExpandEnv(string(data))
	if err := yaml.Unmarshal([]byte(expandedData), config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	config.setDefaults()

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// setDefaults sets default values for configuration
func (c *Config) setDefaults() {
	if c.App.LogLevel == "" {
		c.App.LogLevel = "INFO"
	}
	if c.App.Port == 0 {
		c.App.Port = 8080
	}

	// P2-21: Default to structured INFO logging
	if c.Logger.Level == "" {
		c.Logger.Level = "INFO"
	}
	if c.Logger.Format == "" {
		c.Logger.Format = "json"
	}
	if c.Logger.OutputPath == "" {
		c.Logger.OutputPath = "stdout"
	}

	if c.Database.Port == 0 {
		c.Database.Port = 3306
	}
	if c.Database.MaxConnections == 0 {
		c.Database.MaxConnections = 25
	}
	if c.Database.MaxIdleConnections == 0 {
		c.Database.MaxIdleConnections = 5
	}
	if c.Database.MaxOpenConnections == 0 {
		c.Database.MaxOpenConnections = 25
	}
	if c.Database.ConnectionTimeout == "" {
		c.Database.ConnectionTimeout = "10s"
	}

	if c.SNMP.Timeout == "" {
		c.SNMP.Timeout = "5s"
	}
	if c.SNMP.Retries == 0 {
		c.SNMP.Retries = 3
	}
	if c.SNMP.Community == "" {
		c.SNMP.Community = "public"
	}
	if c.SNMP.Version == "" {
		c.SNMP.Version = "2c"
	}
	if c.SNMP.Port == 0 {
		c.SNMP.Port = 161
	}

	if c.Polling.DefaultInterval == "" {
		c.Polling.DefaultInterval = "60s"
	}
	if c.Polling.MaxWorkers == 0 {
		c.Polling.MaxWorkers = 50
	}
	if c.Polling.BatchInsertSize == 0 {
		c.Polling.BatchInsertSize = 100
	}
	if c.Polling.BatchInsertTimeout == "" {
		c.Polling.BatchInsertTimeout = "10s"
	}
	if c.Polling.HealthCheckInterval == "" {
		c.Polling.HealthCheckInterval = "30s"
	}
	if c.Polling.MetricsRetentionDays == 0 {
		c.Polling.MetricsRetentionDays = 3 // default 3 days for metrics
	}
	if c.Polling.LogsRetentionDays == 0 {
		c.Polling.LogsRetentionDays = 30 // default 30 days for logs
	}

	if c.PromQL.MaxRange == 0 {
		c.PromQL.MaxRange = 24 * time.Hour
	}
	if c.PromQL.MaxResultSize == 0 {
		c.PromQL.MaxResultSize = 1000
	}
	if c.PromQL.RateLimitRequests == 0 {
		c.PromQL.RateLimitRequests = 10
	}
	if c.PromQL.RateLimitWindow == 0 {
		c.PromQL.RateLimitWindow = time.Minute
	}

	if c.Logger.MaxSize == 0 {
		c.Logger.MaxSize = 100 // MB
	}
	if c.Logger.MaxBackups == 0 {
		c.Logger.MaxBackups = 5
	}
	if c.Logger.MaxAge == 0 {
		c.Logger.MaxAge = 30 // Days
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("database user is required")
	}
	if c.Database.Database == "" {
		return fmt.Errorf("database name is required")
	}

	// Validate timeout values
	if _, err := time.ParseDuration(c.SNMP.Timeout); err != nil {
		return fmt.Errorf("invalid SNMP timeout: %w", err)
	}
	if _, err := time.ParseDuration(c.Polling.DefaultInterval); err != nil {
		return fmt.Errorf("invalid default polling interval: %w", err)
	}
	if _, err := time.ParseDuration(c.Polling.HealthCheckInterval); err != nil {
		return fmt.Errorf("invalid health check interval: %w", err)
	}

	return nil
}

// GetSNMPTimeout returns SNMP timeout as time.Duration
func (c *Config) GetSNMPTimeout() time.Duration {
	d, _ := time.ParseDuration(c.SNMP.Timeout)
	return d
}

// GetPollingInterval returns default polling interval as time.Duration
func (c *Config) GetPollingInterval() time.Duration {
	d, _ := time.ParseDuration(c.Polling.DefaultInterval)
	return d
}

// GetHealthCheckInterval returns the health check interval as time.Duration
func (c *Config) GetHealthCheckInterval() time.Duration {
	d, _ := time.ParseDuration(c.Polling.HealthCheckInterval)
	return d
}

// GetBatchInsertTimeout returns batch insert timeout as time.Duration
func (c *Config) GetBatchInsertTimeout() time.Duration {
	d, _ := time.ParseDuration(c.Polling.BatchInsertTimeout)
	return d
}

// GetConnectionTimeout returns database connection timeout as time.Duration
func (c *Config) GetConnectionTimeout() time.Duration {
	d, _ := time.ParseDuration(c.Database.ConnectionTimeout)
	return d
}

// GetTimeout returns SNMP timeout as time.Duration
func (c *SNMPConfig) GetTimeout() time.Duration {
	d, _ := time.ParseDuration(c.Timeout)
	return d
}

// GetRateCalculationInterval returns the rate calculation interval as time.Duration
func (c *Config) GetRateCalculationInterval() time.Duration {
	if c.Polling.RateCalculationInterval == "" {
		return 30 * time.Second // Default 30 seconds if not set
	}
	d, _ := time.ParseDuration(c.Polling.RateCalculationInterval)
	return d
}

// GetCleanupInterval returns the cleanup interval as time.Duration
func (c *Config) GetCleanupInterval() time.Duration {
	if c.Polling.CleanupInterval == "" {
		return 24 * time.Hour // Default 24 hours if not set
	}
	d, _ := time.ParseDuration(c.Polling.CleanupInterval)
	return d
}
