package ws

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/google/uuid"
)

type Hub struct {
	// Active connections mapped by user ID
	clients    map[uuid.UUID]map[*Client]bool
	clientsMu  sync.RWMutex
	Register   chan *Client
	Unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.clientsMu.Lock()
			if _, exists := h.clients[client.UserID]; !exists {
				h.clients[client.UserID] = make(map[*Client]bool)
			}
			h.clients[client.UserID][client] = true
			h.clientsMu.Unlock()
			log.Printf("WebSocket Client registered: %s", client.UserID)

		case client := <-h.Unregister:
			h.clientsMu.Lock()
			if userClients, exists := h.clients[client.UserID]; exists {
				if _, ok := userClients[client]; ok {
					delete(userClients, client)
					close(client.Send)
					if len(userClients) == 0 {
						delete(h.clients, client.UserID)
					}
				}
			}
			h.clientsMu.Unlock()
			log.Printf("WebSocket Client unregistered: %s", client.UserID)
		}
	}
}

// BroadcastToUsers sends an event to a specific list of user IDs
func (h *Hub) BroadcastToUsers(eventType string, payload interface{}, userIDs []uuid.UUID) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling WS payload: %v", err)
		return
	}

	event := Event{
		Type:    eventType,
		Payload: jsonPayload,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		log.Printf("Error marshaling WS event: %v", err)
		return
	}

	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	for _, userID := range userIDs {
		if userClients, exists := h.clients[userID]; exists {
			for client := range userClients {
				select {
				case client.Send <- eventBytes:
				default:
					// If client channel is blocked, close it asynchronously
					log.Printf("WS client buffer full for user %s, discarding message", userID)
				}
			}
		}
	}
}

// IsUserOnline checks if a user has any active connection
func (h *Hub) IsUserOnline(userID uuid.UUID) bool {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	conns, exists := h.clients[userID]
	return exists && len(conns) > 0
}
