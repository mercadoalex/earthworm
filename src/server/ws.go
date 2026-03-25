package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// WSMessage is the envelope for all WebSocket messages.
type WSMessage struct {
	Type    string      `json:"type"` // "heartbeat", "alert", "status"
	Payload interface{} `json:"payload"`
}

// Client represents a single WebSocket connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Hub manages WebSocket client connections and broadcasts.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's event loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastHeartbeat sends a heartbeat event to all connected clients.
func (h *Hub) BroadcastHeartbeat(event Heartbeat) {
	msg := WSMessage{Type: "heartbeat", Payload: event}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal heartbeat WS message: %v", err)
		return
	}
	h.broadcast <- data
}

// BroadcastAlert sends an alert to all connected clients.
func (h *Hub) BroadcastAlert(alert Alert) {
	msg := WSMessage{Type: "alert", Payload: alert}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal alert WS message: %v", err)
		return
	}
	h.broadcast <- data
}

// BroadcastEbpfEvent sends an enriched kernel event to all connected clients.
func (h *Hub) BroadcastEbpfEvent(event EnrichedEvent) {
	msg := WSMessage{Type: "ebpf_event", Payload: event}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal ebpf_event WS message: %v", err)
		return
	}
	h.broadcast <- data
}

// BroadcastCausalChain sends a causal chain to all connected clients.
func (h *Hub) BroadcastCausalChain(chain CausalChain) {
	msg := WSMessage{Type: "causal_chain", Payload: chain}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal causal_chain WS message: %v", err)
		return
	}
	h.broadcast <- data
}

// BroadcastPrediction sends a prediction alert to all connected clients.
func (h *Hub) BroadcastPrediction(prediction Prediction) {
	msg := WSMessage{Type: "prediction", Payload: prediction}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal prediction WS message: %v", err)
		return
	}
	h.broadcast <- data
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ServeWS handles WebSocket upgrade requests.
func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	hub.register <- client

	// Writer goroutine
	go func() {
		defer func() {
			conn.Close()
		}()
		for message := range client.send {
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				break
			}
		}
	}()

	// Reader goroutine (just drains messages to detect disconnection)
	go func() {
		defer func() {
			hub.unregister <- client
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}
