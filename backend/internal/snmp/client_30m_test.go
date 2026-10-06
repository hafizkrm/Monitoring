package snmp

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/hafizkrm/Monitoring/backend/internal/logger"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

// TestGoroutineStability30m is the 30-minute SNMP continuous polling stability test.
// It simulates 50 devices polling every 5 seconds against a blackhole UDP server
// for 30 minutes, capturing goroutine/memory/request metrics at 5-minute intervals.
//
// Run: go test -v -timeout 40m -run TestGoroutineStability30m ./internal/snmp
// Skip in short mode: go test -short ./internal/snmp
func TestGoroutineStability30m(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 30m stability test in short mode")
	}

	// Start a blackhole UDP server to simulate unresponsive SNMP agents
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("Failed to start blackhole server: %v", err)
	}
	defer conn.Close()
	port := conn.LocalAddr().(*net.UDPAddr).Port

	cfg := &config.Config{
		SNMP: config.SNMPConfig{
			Port:      port,
			Timeout:   "1s", // intentionally short to force rapid timeouts
			Retries:   1,
			Community: "public",
			Version:   "2c",
		},
	}

	dummyLog := logger.InitLogger(config.LoggerConfig{})
	client := NewClient(cfg, dummyLog)
	defer client.Close()

	// Let the client stabilize
	time.Sleep(1 * time.Second)

	startGoroutines := runtime.NumGoroutine()
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)

	t.Logf("T0: Goroutines=%d, HeapAlloc=%dMB, Requests=0, Timeouts/Errors=0",
		startGoroutines, startMem.Alloc/1024/1024)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deviceCount := 50
	pollingInterval := 5 * time.Second

	var totalRequests int64
	var totalErrors int64

	// Start continuous polling workers â€” 50 goroutines each polling every 5s
	for i := 0; i < deviceCount; i++ {
		go func(deviceID int) {
			ticker := time.NewTicker(pollingInterval)
			defer ticker.Stop()
			device := models.Device{
				ID:        deviceID + 1,
				IPAddress: fmt.Sprintf("127.0.0.%d", (deviceID%254)+1),
			}
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					atomic.AddInt64(&totalRequests, 1)
					_, err := client.CollectDeviceMetrics(ctx, device)
					if err != nil {
						atomic.AddInt64(&totalErrors, 1)
					}
				}
			}
		}(i)
	}

	// Checkpoint intervals
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
		sleepUntil := start.Add(cp)
		time.Sleep(time.Until(sleepUntil))

		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		g := runtime.NumGoroutine()
		reqs := atomic.LoadInt64(&totalRequests)
		errs := atomic.LoadInt64(&totalErrors)

		t.Logf("T+%vm: Goroutines=%d, HeapAlloc=%dMB, Requests=%d, Timeouts/Errors=%d",
			cp.Minutes(), g, m.Alloc/1024/1024, reqs, errs)

		// Goroutine bound: start + deviceCount + some slack (for cleanup goroutine, etc.)
		maxAllowed := float64(startGoroutines) + float64(deviceCount) + 50
		if float64(g) > maxAllowed {
			t.Errorf("GOROUTINE LEAK at T+%vm! Count=%d, Max allowed=%.0f", cp.Minutes(), g, maxAllowed)
			return
		}
	}

	// Final summary
	finalReqs := atomic.LoadInt64(&totalRequests)
	finalErrs := atomic.LoadInt64(&totalErrors)
	t.Logf("FINAL: Total Requests=%d, Total Errors=%d, Duration=%v",
		finalReqs, finalErrs, time.Since(start))
	t.Logf("RESULT: Goroutine count remained bounded throughout 30 minutes of continuous polling.")
}
