package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	configContent := `
app:
  name: test-nms
  version: 1.0.0
database:
  host: localhost
  user: root
  database: monitoring
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Test loading
	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.App.Name != "test-nms" {
		t.Errorf("expected app name 'test-nms', got '%s'", cfg.App.Name)
	}

	if cfg.Database.Host != "localhost" {
		t.Errorf("expected db host 'localhost', got '%s'", cfg.Database.Host)
	}

	// Check defaults
	if cfg.App.LogLevel != "INFO" {
		t.Errorf("expected default log level 'INFO', got '%s'", cfg.App.LogLevel)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "Valid config",
			config: Config{
				Database: DatabaseConfig{
					Host:     "localhost",
					User:     "root",
					Database: "test",
				},
			},
			wantErr: false,
		},
		{
			name: "Missing host",
			config: Config{
				Database: DatabaseConfig{
					User:     "root",
					Database: "test",
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid timeout",
			config: Config{
				Database: DatabaseConfig{
					Host:     "localhost",
					User:     "root",
					Database: "test",
				},
				SNMP: SNMPConfig{
					Timeout: "invalid",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.setDefaults()
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

