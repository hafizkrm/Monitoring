package snmp

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yourusername/viscod/internal/config"
	"github.com/yourusername/viscod/internal/models"
)

// TestGoroutineStability30m is the 30-minute SNMP load test harness.
func TestGoroutineStability30m(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 30m stability test in short mode")
	}

	startGoroutines := runtime.NumGoroutine()
	t.Logf("T0: Active Goroutines = %d", startGoroutines)

	cfg := config.SNMPConfig{
		Timeout:   "1s", // intentionally short to force timeouts quickly
		Retries:   1,
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

	var totalRequests int64
	var totalErrors int64

	// Start continuous polling workers
	for i := 0; i < deviceCount; i++ {
		go func(deviceID int) {
			ticker := time.NewTicker(pollingInterval)
			defer ticker.Stop()
			// Use an IP address that doesn't route or drops packets (e.g. 192.0.2.x TEST-NET-1)
			device := models.Device{
				ID:        deviceID + 1,
				IPAddress: fmt.Sprintf("192.0.2.%d", (deviceID%250)+1), 
			}
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					atomic.AddInt64(&totalRequests, 1)
					// This will block and timeout
					_, err := client.CollectDeviceMetrics(ctx, device)
					if err != nil {
						atomic.AddInt64(&totalErrors, 1)
					}
				}
			}
		}(i)
	}

	// Wait and observe
	checkpoints := []time.Duration{
		0,
		5 * time.Minute,
		10 * time.Minute,
		15 * time.Minute,
		20 * time.Minute,
		25 * time.Minute,
		30 * time.Minute,
	}

	start := time.Now()
	for _, cp := range checkpoints {
		if cp > 0 {
			time.Sleep(time.Until(start.Add(cp)))
		}
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		g := runtime.NumGoroutine()
		reqs := atomic.LoadInt64(&totalRequests)
		errs := atomic.LoadInt64(&totalErrors)
		t.Logf("T%vm: Goroutines=%d, HeapAlloc=%vMB, Requests=%d, Timeouts/Errors=%d", 
			cp.Minutes(), g, m.Alloc/1024/1024, reqs, errs)
		
		if float64(g) > float64(startGoroutines)*3.0 + float64(deviceCount) + 50 {
			t.Errorf("Goroutine leak detected at T%vm! Count: %d", cp.Minutes(), g)
			return
		}
	}
}
