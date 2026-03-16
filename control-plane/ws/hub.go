package ws

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type EventType string

const (
	EventContainerDying        EventType = "container.dying"
	EventContainerRestarting   EventType = "container.restarting"
	EventContainerHealthy      EventType = "container.healthy"
	EventContainerProvisioning EventType = "container.provisioning"
	EventControlPlaneError     EventType = "control_plane.error"
	EventGameStarted           EventType = "game.started"
	EventGameEnded             EventType = "game.ended"
	EventReconcilerTick        EventType = "reconciler.tick"
	EventHealFailed            EventType = "heal.failed"
	EventSimConfig             EventType = "simulation.config"
)

type Event struct {
	Type    EventType   `json:"type"`
	Payload interface{} `json:"payload"`
}

type Hub struct {
	clients   map[*websocket.Conn]bool
	mu        sync.Mutex
	broadcast chan Event
	upgrader  websocket.Upgrader
}

func NewHub() *Hub {
	h := &Hub{
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan Event, 100),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	for event := range h.broadcast {
		h.mu.Lock()
		for conn := range h.clients {
			if err := conn.WriteJSON(event); err != nil {
				log.Printf("ws write error: %v", err)
				conn.Close()
				delete(h.clients, conn)
			}
		}
		h.mu.Unlock()
	}
}

func (h *Hub) Broadcast(e Event) {
	select {
	case h.broadcast <- e:
	default:
		log.Println("ws broadcast channel full, dropping event")
	}
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, conn)
			h.mu.Unlock()
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}
