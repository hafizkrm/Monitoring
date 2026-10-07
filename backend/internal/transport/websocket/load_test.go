package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
)

func TestWSLoad200Devices(t *testing.T) {
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

	// 10 multiple concurrent clients
	clientCount := 10
	var conns []*websocket.Conn
	var wg sync.WaitGroup

	t.Logf("Connecting %d websocket clients...", clientCount)
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

	t.Log("All clients connected. Idle for 1 seconds...")
	time.Sleep(1 * time.Second)

	// Simulate 200 devices sending deltas concurrently
	t.Log("Simulating 200 concurrent devices...")
	start := time.Now()

	deviceCount := 200
	wg.Add(deviceCount)

	for i := 0; i < deviceCount; i++ {
		go func(deviceID int) {
			defer wg.Done()
			for j := 0; j < 5; j++ { // 5 updates per device
				hub.broadcast <- contracts.WSEventEnvelope{
					Event: "device_metrics_delta",
					Payload: map[string]interface{}{
						"device_id": deviceID,
						"cpu":       10 + j,
					},
				}
				time.Sleep(10 * time.Millisecond) // Poll interval simulation
			}
		}(i)
	}

	wg.Wait()
	t.Logf("Broadcasting finished for 200 devices. Time: %v", time.Since(start))

	for _, c := range conns {
		c.Close()
	}
}
