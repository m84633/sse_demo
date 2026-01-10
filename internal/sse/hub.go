package sse

import (
	"context"
	"sync"

	"sse_demo/internal/model"
)

type Client struct {
	Room string
	Ch   chan model.Notification
}

type Hub struct {
	register   chan *Client
	unregister chan *Client
	broadcast  chan model.Notification
	rooms      map[string]map[*Client]struct{}
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan model.Notification, 64),
		rooms:      make(map[string]map[*Client]struct{}),
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) Broadcast(notification model.Notification) {
	h.broadcast <- notification
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.addClient(client)
		case client := <-h.unregister:
			h.removeClient(client)
		case notification := <-h.broadcast:
			h.broadcastToRoom(notification)
		}
	}
}

func (h *Hub) addClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[client.Room] == nil {
		h.rooms[client.Room] = make(map[*Client]struct{})
	}
	h.rooms[client.Room][client] = struct{}{}
}

func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	room := h.rooms[client.Room]
	if room == nil {
		return
	}
	delete(room, client)
	if len(room) == 0 {
		delete(h.rooms, client.Room)
	}
}

func (h *Hub) broadcastToRoom(notification model.Notification) {
	h.mu.RLock()
	room := h.rooms[notification.Room]
	h.mu.RUnlock()
	for client := range room {
		select {
		case client.Ch <- notification:
		default:
			// Drop if the client is too slow.
		}
	}
}
