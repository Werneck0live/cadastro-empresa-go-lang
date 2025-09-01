package ws

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
)

type Client struct {
	ID   string
	Send chan []byte
}

type unicastMsg struct {
	id  string
	msg []byte
}

type Hub struct {
	mu       sync.RWMutex
	clients  map[string]*Client // id -> client
	register chan *Client
	unreg    chan *Client

	sendAll chan []byte     // envio para todos
	unicast chan unicastMsg // envio para 1 cliente

	log     *slog.Logger
	stop    chan struct{}
	stopped chan struct{}

	nextID atomic.Uint64
}

func NewHub(log *slog.Logger) *Hub {
	if log == nil {
		log = slog.Default()
	}
	return &Hub{
		clients:  make(map[string]*Client),
		register: make(chan *Client),
		unreg:    make(chan *Client),
		sendAll:  make(chan []byte, 1024),
		unicast:  make(chan unicastMsg, 1024),
		log:      log.With("cmp", "ws.hub"),
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

func (h *Hub) newID() string {
	id := h.nextID.Add(1)
	return fmt.Sprintf("c%d", id)
}

func (h *Hub) Run() {
	h.log.Info("hub_run_start")
	defer close(h.stopped)

	for {
		select {
		case c := <-h.register:
			if c.ID == "" {
				c.ID = h.newID()
			}
			h.mu.Lock()
			h.clients[c.ID] = c
			h.mu.Unlock()
			h.log.Info("client_registered", "id", c.ID, "total", len(h.clients))

		case c := <-h.unreg:
			h.mu.Lock()
			if c != nil && c.ID != "" {
				if _, ok := h.clients[c.ID]; ok {
					delete(h.clients, c.ID)
					close(c.Send)
				}
			}
			h.mu.Unlock()
			h.log.Info("client_unregistered", "id", c.ID, "total", len(h.clients))

		case msg := <-h.sendAll:
			h.mu.RLock()
			for id, c := range h.clients {
				select {
				case c.Send <- msg:
				default:
					// cliente lento -> dropa para não travar o hub
					h.mu.RUnlock()
					h.mu.Lock()
					delete(h.clients, id)
					close(c.Send)
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()

		case u := <-h.unicast:
			h.mu.RLock()
			c := h.clients[u.id]
			h.mu.RUnlock()
			if c == nil {
				h.log.Warn("send_one_miss", "id", u.id)
				continue
			}
			select {
			case c.Send <- u.msg:
			default:
				// cliente lento -> remove
				h.mu.Lock()
				if cc := h.clients[u.id]; cc != nil {
					delete(h.clients, u.id)
					close(cc.Send)
				}
				h.mu.Unlock()
				h.log.Warn("send_one_drop_slow", "id", u.id)
			}

		case <-h.stop:
			h.mu.Lock()
			for id, c := range h.clients {
				close(c.Send)
				delete(h.clients, id)
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
func (h *Hub) Unregister(c *Client) { h.unreg <- c }

func (h *Hub) Broadcast(b []byte)               { h.sendAll <- b }
func (h *Hub) SendToClient(id string, b []byte) { h.unicast <- unicastMsg{id: id, msg: b} }
