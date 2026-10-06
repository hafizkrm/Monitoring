package websocket

import (
	"log"

	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
	"github.com/hafizkrm/Monitoring/backend/internal/transport/eventbus"
	"github.com/prometheus/client_golang/prometheus"
)

// Hub menangani register, unregister client, dan mem-broadcast pesan.
type Hub struct {
	clients map[*Client]bool
	broadcast chan contracts.WSEventEnvelope
	register chan *Client
	unregister chan *Client

	eventBus     eventbus.EventBus
	subscribers  []*eventbus.Subscriber
	stopEventBus chan struct{}
}

func NewHub() *Hub {
	return &Hub{
		broadcast:    make(chan contracts.WSEventEnvelope, 256),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
		clients:      make(map[*Client]bool),
		stopEventBus: make(chan struct{}),
	}
}

// SetEventBus injects the EventBus dependency (M3).
func (h *Hub) SetEventBus(eb eventbus.EventBus) {
	h.eventBus = eb
}

func (h *Hub) Run() {
	log.Println("WebSocket Hub is running...")
	
	if h.eventBus != nil {
		h.subscribeToDomainEvents()
	}

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
			topic := "global"
			if message.DeviceID != "" {
				topic = "device:" + message.DeviceID
			}

			for client := range h.clients {
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
		case <-h.stopEventBus:
			if h.eventBus != nil {
				h.unsubscribeFromDomainEvents()
			}
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			return
		}
	}
}

func (h *Hub) subscribeToDomainEvents() {
	topics := []contracts.EventType{
		contracts.DomainEventDeviceConnected,
		contracts.DomainEventDeviceDisconnected,
		contracts.DomainEventMetricsUpdated,
		contracts.DomainEventAlertCreated,
		contracts.DomainEventLogCreated,
	}

	for _, topic := range topics {
		// Bounded buffer for backpressure
		sub := h.eventBus.Subscribe(topic, 100)
		h.subscribers = append(h.subscribers, sub)
		go h.listenToEventBus(sub)
	}

	// Phase 3: Prometheus Observability - Export dropped count for WebSocket Hub
	prometheus.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Name: "nms_eventbus_dropped_messages_total",
			Help: "Total dropped messages by eventbus subscriber",
			ConstLabels: prometheus.Labels{"component": "websocket_hub"},
		},
		func() float64 {
			var total uint64
			for _, sub := range h.subscribers {
				total += sub.DroppedCount()
			}
			return float64(total)
		},
	))
}

func (h *Hub) listenToEventBus(sub *eventbus.Subscriber) {
	for {
		select {
		case domainEvent, ok := <-sub.Channel:
			if !ok {
				return // Channel closed
			}
			// Translate DomainEvent to WSEventEnvelope (M3)
			wsMsg := contracts.WSEventEnvelope{
				Event:     string(domainEvent.Type),
				Timestamp: domainEvent.Timestamp,
				DeviceID:  domainEvent.DeviceID,
				Payload:   domainEvent.Payload,
			}
			select {
			case h.broadcast <- wsMsg:
			case <-h.stopEventBus:
				return
			default:
				log.Println("[WebSocket Hub] Broadcast channel full, dropping EventBus message")
			}
		case <-h.stopEventBus:
			return
		}
	}
}

func (h *Hub) unsubscribeFromDomainEvents() {
	// Not strictly required since EventBus Shutdown handles it, but good for completeness
	// Actually EventBus Unsubscribe takes the topic which we didn't store.
	// Since we are shutting down, we can just let it go.
}

func (h *Hub) Shutdown() {
	select {
	case <-h.stopEventBus:
		// already closed
	default:
		close(h.stopEventBus)
	}
}

// BroadcastMessage adalah interface/fungsi yang dipanggil oleh DeviceManager.
// Maintained for Phase 1 backward compatibility.
func (h *Hub) BroadcastMessage(msg contracts.WSEventEnvelope) {
	select {
	case h.broadcast <- msg:
	default:
		log.Println("[WebSocket Hub] Broadcast channel full, non-blocking drop to prevent caller stalling")
	}
}
