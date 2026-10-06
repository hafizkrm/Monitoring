package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 15 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		
		return u.Host == r.Host
	},
}

type SubscribeMessage struct {
	Action string   `json:"action"` // "subscribe" or "unsubscribe"
	Topics []string `json:"topics"`
}

// Client adalah representasi koneksi dari sebuah browser/frontend.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan contracts.WSEventEnvelope
	topics map[string]bool
	mu     sync.RWMutex
}

func (c *Client) HasTopic(topic string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.topics[topic]
}

// readPump membaca pesan/pong dari koneksi WebSocket dan menangani unregister saat terputus.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WebSocket Client] Read error: %v", err)
			}
			break
		}

		var sub SubscribeMessage
		if err := json.Unmarshal(message, &sub); err == nil {
			c.mu.Lock()
			switch sub.Action {
			case "subscribe":
				for _, topic := range sub.Topics {
					c.topics[topic] = true
				}
			case "unsubscribe":
				for _, topic := range sub.Topics {
					delete(c.topics, topic)
				}
			}
			c.mu.Unlock()
		}
	}
}

// writePump mengirim (pump) pesan dari Hub ke koneksi WebSocket.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			json.NewEncoder(w).Encode(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWS menangani request koneksi WebSocket dari klien.
func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	client := &Client{
		hub:    hub, 
		conn:   conn, 
		send:   make(chan contracts.WSEventEnvelope, 256),
		topics: make(map[string]bool),
	}
	
	// Automatically subscribe to global topics if desired, e.g., "global"
	client.topics["global"] = true

	client.hub.register <- client

	// Jalankan rutin baca & tulis di background
	go client.writePump()
	go client.readPump()
}
