#!/usr/bin/env bash
set -e

echo "[SNMP] Checking environment..."
cd ../../backend

# We will run a specific Go test designed for 30 minutes.
# Test harness is in backend/internal/snmp/client_leak_test.go
# But we need a dedicated 30m test. We will create it dynamically or run an existing one.

# Let's create a dedicated 30-minute harness test file
cat << 'EOF' > internal/snmp/client_30m_test.go
package snmp

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/yourusername/viscod/internal/config"
)

// TestGoroutineStability30m is the 30-minute SNMP load test harness.
func TestGoroutineStability30m(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 30m stability test in short mode")
	}

	startGoroutines := runtime.NumGoroutine()
	t.Logf("T0: Active Goroutines = %d", startGoroutines)

	cfg := config.SNMPConfig{
		Timeout:   "2s", // intentionally short to force timeouts
		Retries:   3,
		Community: "public",
		Version:   "2c",
		Port:      161,
	}

	client := NewClient(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deviceCount := 50
	pollingInterval := 5 * time.Second
	duration := 30 * time.Minute

	// Start continuous polling workers
	for i := 0; i < deviceCount; i++ {
		go func(deviceID int) {
			ticker := time.NewTicker(pollingInterval)
			defer ticker.Stop()
			ip := fmt.Sprintf("192.168.200.%d", deviceID+1) // fake IPs
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// This will block and timeout
					_, _ = client.GetMetrics(ctx, ip)
				}
			}
		}(i)
	}

	// Wait and observe
	checkpoints := []time.Duration{
		5 * time.Minute,
		10 * time.Minute,
		15 * time.Minute,
		20 * time.Minute,
		25 * time.Minute,
		30 * time.Minute,
	}

	start := time.Now()
	for _, cp := range checkpoints {
		time.Sleep(time.Until(start.Add(cp)))
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		g := runtime.NumGoroutine()
		t.Logf("T%vm: Goroutines=%d, HeapAlloc=%vMB", cp.Minutes(), g, m.Alloc/1024/1024)
		if float64(g) > float64(startGoroutines)*3.0 + float64(deviceCount) {
			t.Errorf("Goroutine leak detected at T%vm! Count: %d", cp.Minutes(), g)
			return
		}
	}
}
EOF

echo "[SNMP] Running 30-minute stability test..."
# Go test default timeout is 10m, we need to extend it
if go test -v -timeout 40m -run TestGoroutineStability30m ./internal/snmp; then
    echo "SNMP 30m test passed."
    exit 0
else
    echo "SNMP 30m test failed."
    exit 1
fi
