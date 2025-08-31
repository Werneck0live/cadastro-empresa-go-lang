package ws

import (
	"log/slog"
	"sync"
)

type Client struct {
	Send chan []byte
	// você pode guardar metadados se quiser (id, etc.)
}

type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	log        *slog.Logger
	stop       chan struct{}
	stopped    chan struct{}
}

func NewHub(log *slog.Logger) *Hub {
	if log == nil {
		log = slog.Default()
	}
	return &Hub{
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 1024),
		log:        log.With("cmp", "ws.hub"),
		stop:       make(chan struct{}),
		stopped:    make(chan struct{}),
	}
}

func (h *Hub) Run() {
	h.log.Info("hub_run_start")
	defer close(h.stopped)

	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.mu.Unlock()
			h.log.Info("client_registered", "total", len(h.clients))
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.Send)
			}
			h.mu.Unlock()
			h.log.Info("client_unregistered", "total", len(h.clients))
		case msg := <-h.broadcast:
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.Send <- msg:
				default:
					// cliente lento: derruba para não travar o hub
					h.mu.RUnlock()
					h.mu.Lock()
					delete(h.clients, c)
					close(c.Send)
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		case <-h.stop:
			h.mu.Lock()
			for c := range h.clients {
				close(c.Send)
				delete(h.clients, c)
			}
			h.mu.Unlock()
			h.log.Info("hub_run_stop")
			return
		}
	}
}

func (h *Hub) Stop() {
	close(h.stop)
	<-h.stopped
}

func (h *Hub) Register(c *Client)   { h.register <- c }
func (h *Hub) Unregister(c *Client) { h.unregister <- c }
func (h *Hub) Broadcast(b []byte)   { h.broadcast <- b }
