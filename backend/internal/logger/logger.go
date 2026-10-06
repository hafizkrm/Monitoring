package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yourusername/viscod/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger interface for structured logging
type Logger interface {
	Debug(message string, fields map[string]interface{})
	Info(message string, fields map[string]interface{})
	Warn(message string, fields map[string]interface{})
	Error(message string, fields map[string]interface{})
	Sync() error
}

// SimpleLogger is a basic logger implementation with automatic rotation support
type SimpleLogger struct {
	level      string
	format     string
	outputPath string
	output     io.Writer
	mu         sync.Mutex
}

// InitLogger initializes logger based on config
func InitLogger(cfg config.LoggerConfig) Logger {
	// Ensure log directory exists
	logDir := filepath.Dir(cfg.OutputPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create log directory: %v\n", err)
	}

	maxMB := cfg.MaxSize
	if maxMB <= 0 {
		maxMB = 50 // Default 50MB
	}

	maxBackups := cfg.MaxBackups
	if maxBackups <= 0 {
		maxBackups = 5
	}

	maxAge := cfg.MaxAge
	if maxAge <= 0 {
		maxAge = 30 // Default 30 days
	}

	l := &SimpleLogger{
		level:      cfg.Level,
		format:     cfg.Format,
		outputPath: cfg.OutputPath,
	}

	if l.outputPath == "" || l.outputPath == "stdout" {
		l.output = os.Stdout
	} else {
		// Use lumberjack for log rotation with compression
		l.output = &lumberjack.Logger{
			Filename:   l.outputPath,
			MaxSize:    maxMB, // megabytes
			MaxBackups: maxBackups,
			MaxAge:     maxAge, // days
			Compress:   true,   // disabled by default
		}
	}

	return l
}

// Debug logs a debug message
func (l *SimpleLogger) Debug(message string, fields map[string]interface{}) {
	if l.shouldLog("DEBUG") {
		l.log("DEBUG", message, fields)

	}
}

// Info logs an info message
func (l *SimpleLogger) Info(message string, fields map[string]interface{}) {
	if l.shouldLog("INFO") {
		l.log("INFO", message, fields)
	}
}

// Warn logs a warning message
func (l *SimpleLogger) Warn(message string, fields map[string]interface{}) {
	if l.shouldLog("WARN") {
		l.log("WARN", message, fields)
	}
}

// Error logs an error message
func (l *SimpleLogger) Error(message string, fields map[string]interface{}) {
	if l.shouldLog("ERROR") {
		l.log("ERROR", message, fields)
	}
}

// Sync flushes any buffered log entries
func (l *SimpleLogger) Sync() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return nil
}

// shouldLog checks if message at level should be logged
func (l *SimpleLogger) shouldLog(level string) bool {
	levels := map[string]int{
		"DEBUG": 0,
		"INFO":  1,
		"WARN":  2,
		"ERROR": 3,
	}

	return levels[level] >= levels[l.level]
}

// log outputs a log message thread-safely with rotation check
func (l *SimpleLogger) log(level string, message string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	var output string

	if l.format == "json" {
		logMap := make(map[string]interface{})
		for k, v := range fields {
			logMap[k] = v
		}
		logMap["time"] = timestamp
		logMap["level"] = level
		logMap["message"] = message

		b, err := json.Marshal(logMap)
		if err == nil {
			output = string(b) + "\n"
		} else {
			output = fmt.Sprintf("{\"time\": \"%s\", \"level\": \"%s\", \"message\": \"%s\", \"error\": \"failed to marshal fields\"}\n", timestamp, level, message)
		}
	} else {
		fieldStr := ""
		for k, v := range fields {
			fieldStr += fmt.Sprintf(" %s=%v", k, v)
		}
		output = fmt.Sprintf("[%s] [%s] %s%s\n", timestamp, level, message, fieldStr)
	}

	_, _ = fmt.Fprint(l.output, output)
}
