package websocket

import (
	"log"

	"github.com/yourusername/viscod/internal/contracts"
)

// Hub menangani register, unregister client, dan mem-broadcast pesan.
type Hub struct {
	clients map[*Client]bool
	broadcast chan contracts.WSEventEnvelope
	register chan *Client
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan contracts.WSEventEnvelope, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) Run() {
	log.Println("WebSocket Hub is running...")
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			// Determine the required topic for this message
			topic := "global"
			if message.DeviceID != "" {
				topic = "device:" + message.DeviceID
			}

			for client := range h.clients {
				// Only send if the client subscribed to this topic
				if !client.HasTopic(topic) && !client.HasTopic("all") {
					continue
				}

				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// BroadcastMessage adalah interface/fungsi yang dipanggil oleh DeviceManager
func (h *Hub) BroadcastMessage(msg contracts.WSEventEnvelope) {
	select {
	case h.broadcast <- msg:
	default:
		log.Println("[WebSocket Hub] Broadcast channel full, non-blocking drop to prevent caller stalling")
	}
}
