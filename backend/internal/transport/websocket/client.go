package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	nms_middleware "github.com/hafizkrm/Monitoring/backend/internal/api/middleware"
	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
	"github.com/hafizkrm/Monitoring/backend/internal/repository"
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

type sessionLookup interface {
	GetByID(context.Context, string, time.Time) (repository.AuthSession, error)
}

// Client adalah representasi koneksi dari sebuah browser/frontend.
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan contracts.WSEventEnvelope
	topics    map[string]bool
	role      string
	sessionID string
	sessions  sessionLookup
	mu        sync.RWMutex
}

func (c *Client) HasTopic(topic string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.topics[topic]
}

func (c *Client) CanSubscribe(topic string) bool {
	if c.role == "admin" {
		return true
	}
	return topic == "global" || strings.HasPrefix(topic, "device:")
}

func (c *Client) CanReceive(event string) bool {
	if c.role == "admin" {
		return true
	}
	switch event {
	case "device.connected", "device.disconnected", "metrics.updated", "alert.created":
		return true
	default:
		return false
	}
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
					if c.CanSubscribe(topic) {
						c.topics[topic] = true
					}
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
			if c.sessions != nil && c.sessionID != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, err := c.sessions.GetByID(ctx, c.sessionID, time.Now().UTC())
				cancel()
				if err != nil {
					_ = c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "session revoked"), time.Now().Add(writeWait))
					return
				}
			}
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWS menangani request koneksi WebSocket dari klien.
func ServeWS(hub *Hub, sessions sessionLookup, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	client := &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan contracts.WSEventEnvelope, 256),
		topics:   make(map[string]bool),
		role:     "viewer",
		sessions: sessions,
	}
	if role, ok := r.Context().Value(nms_middleware.UserRoleContextKey).(string); ok {
		client.role = role
	}
	client.sessionID, _ = r.Context().Value(nms_middleware.SessionIDContextKey).(string)

	// Automatically subscribe to global topics if desired, e.g., "global"
	client.topics["global"] = true

	client.hub.register <- client

	// Jalankan rutin baca & tulis di background
	go client.writePump()
	go client.readPump()
}
