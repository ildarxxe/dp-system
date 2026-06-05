package websocket

import "sync"

type Hub struct {
	clients     map[*Client]bool
	taskClients map[uint64]*Client
	Register    chan *Client
	unregister  chan *Client
	mu          sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		taskClients: make(map[uint64]*Client),
		Register:    make(chan *Client),
		unregister:  make(chan *Client),
		clients:     make(map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.clients[client] = true
			h.taskClients[client.TaskID] = client
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(h.taskClients, client.TaskID)
				close(client.Send)
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) SendMessage(taskID uint64, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if client, ok := h.taskClients[taskID]; ok {
		select {
		case client.Send <- message:
		default:
			go func() {
				h.unregister <- client
			}()
		}
	}
}
