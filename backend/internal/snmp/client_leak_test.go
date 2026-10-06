package snmp

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/yourusername/viscod/internal/config"
	"github.com/yourusername/viscod/internal/logger"
)

// TestGoroutineStability simulates 50 offline/slow devices to ensure no goroutines leak.
func TestGoroutineStability(t *testing.T) {
	// Start a blackhole UDP server
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("Failed to start blackhole server: %v", err)
	}
	defer conn.Close()

	port := conn.LocalAddr().(*net.UDPAddr).Port

	cfg := &config.Config{
		SNMP: config.SNMPConfig{
			Port:      port,
			Timeout:   "1s",
			Community: "public",
			Version:   "2c",
		},
	}
	
	// Create a dummy logger
	dummyLog := logger.InitLogger(config.LoggerConfig{})
	client := NewClient(cfg, dummyLog)
	defer client.Close()

	// Initial Goroutines
	time.Sleep(1 * time.Second)
	startGoroutines := runtime.NumGoroutine()
	t.Logf("Initial Goroutines: %d", startGoroutines)

	// Simulate 50 concurrent requests that will time out
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var done chan bool = make(chan bool, 50)

	for i := 0; i < 50; i++ {
		go func(id int) {
			ip := fmt.Sprintf("127.0.0.%d", id)
			// Query Multiple OIDs which will eventually call queryGet
			_, _ = client.QueryMultipleOIDs(ctx, ip, []string{".1.3.6.1.2.1.1.1.0"}, "public")
			done <- true
		}(i)
	}

	// Wait for all to finish or timeout
	timeout := time.After(10 * time.Second) // generous bound
	completed := 0
	for completed < 50 {
		select {
		case <-done:
			completed++
		case <-timeout:
			t.Fatalf("Test timed out! Goroutines leaked or blocked indefinitely.")
		}
	}

	// Check final goroutines
	time.Sleep(1 * time.Second) // allow cleanup
	endGoroutines := runtime.NumGoroutine()
	t.Logf("Final Goroutines: %d", endGoroutines)

	if float64(endGoroutines) > float64(startGoroutines)*1.5 {
		t.Errorf("Goroutine leak detected: start %d, end %d", startGoroutines, endGoroutines)
	}
}
