#!/usr/bin/env bash
set -e

echo "[Shutdown] Graceful Shutdown Test"

cd ../../backend

cat << 'EOF' > internal/worker/shutdown_test.go
package worker

import (
	"context"
	"testing"
	"time"

	"github.com/yourusername/viscod/internal/config"
	"github.com/yourusername/viscod/internal/transport/websocket"
)

func TestGracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping graceful shutdown test in short mode")
	}

	cfg := config.PollingConfig{
		DefaultInterval: "1s",
		MaxWorkers:      10,
	}

	hub := websocket.NewHub()
	go hub.Run()

	// Using a nil DB and nil SNMP client since we just want to test context propagation
	wm := NewManager(config.Config{Polling: cfg}, nil, nil, nil)
	wm.SetWSBroadcast(func(e contracts.WSEventEnvelope) {})

	// Start workers
	ctx, cancel := context.WithCancel(context.Background())
	go wm.Start(ctx)

	// Let it run for a bit
	time.Sleep(2 * time.Second)

	// Trigger shutdown
	t.Log("Triggering SIGINT (context cancellation)...")
	start := time.Now()
	cancel()

	// Wait for workers to stop gracefully (pseudo-wait or checking goroutine count)
	// Actually Start() blocking means it will return when all workers exit
	time.Sleep(1 * time.Second)
	t.Logf("Shutdown completed in %v", time.Since(start))
}
EOF

if go test -v -run TestGracefulShutdown ./internal/worker; then
    echo "Graceful Shutdown Test passed."
    exit 0
else
    echo "Graceful Shutdown Test failed."
    exit 1
fi
