package websocket

import (
	"encoding/json"
	"log"
	"sync"
)

type StatusChangeMessage struct {
	EventType        string `json:"eventType"`
	ProbeID          string `json:"probeId"`
	APIPath          string `json:"apiPath"`
	APIMethod        string `json:"apiMethod"`
	OldStatus        string `json:"oldStatus"`
	NewStatus        string `json:"newStatus"`
	TriggeredAt      string `json:"triggeredAt"`
	LastResponseMs   int    `json:"lastResponseMs"`
}

type Hub struct {
	projectClients map[string]map[*Client]bool
	mu             sync.RWMutex
}

type Client struct {
	projectID string
	send      chan []byte
}

func NewHub() *Hub {
	return &Hub{
		projectClients: make(map[string]map[*Client]bool),
	}
}

func (h *Hub) RegisterClient(projectID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.projectClients[projectID]; !exists {
		h.projectClients[projectID] = make(map[*Client]bool)
	}
	h.projectClients[projectID][client] = true
	log.Printf("[websocket] Client registered for project %s, total: %d", projectID, len(h.projectClients[projectID]))
}

func (h *Hub) UnregisterClient(projectID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, exists := h.projectClients[projectID]; exists {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			close(client.send)
			if len(clients) == 0 {
				delete(h.projectClients, projectID)
			}
			log.Printf("[websocket] Client unregistered from project %s, remaining: %d", projectID, len(clients))
		}
	}
}

func (h *Hub) BroadcastToProject(projectID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, exists := h.projectClients[projectID]; exists {
		for client := range clients {
			select {
			case client.send <- message:
			default:
				close(client.send)
				delete(clients, client)
			}
		}
	}
}

func (h *Hub) BroadcastStatusChange(projectID string, msg StatusChangeMessage) {
	msg.EventType = "status_change"
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[websocket] Failed to marshal status change message: %v", err)
		return
	}
	h.BroadcastToProject(projectID, data)
}
