#!/usr/bin/env bash
set -e

echo "[WebSocket] Load Test Setup"

cd ../../backend

cat << 'EOF' > internal/transport/websocket/load_test.go
package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWSLoad200Clients(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ws load test in short mode")
	}

	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWS(hub, w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	clientCount := 200
	var conns []*websocket.Conn

	t.Logf("Connecting %d clients...", clientCount)
	for i := 0; i < clientCount; i++ {
		c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Dial failed: %v", err)
		}
		conns = append(conns, c)
		// Drain background
		go func(conn *websocket.Conn) {
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}(c)
	}

	t.Log("All clients connected. Idle for 3 seconds...")
	time.Sleep(3 * time.Second)

	// Simulate delta updates
	t.Log("Broadcasting events...")
	start := time.Now()
	for i := 0; i < 100; i++ {
		hub.Broadcast <- []byte(`{"type":"delta","device":"192.168.1.1","status":"down"}`)
	}
	
	time.Sleep(2 * time.Second)
	t.Logf("Broadcasting finished. Time: %v", time.Since(start))

	for _, c := range conns {
		c.Close()
	}
}
EOF

if go test -v -run TestWSLoad200Clients ./internal/transport/websocket; then
    echo "WebSocket Load Test passed."
    exit 0
else
    echo "WebSocket Load Test failed or blocked."
    exit 1
fi
