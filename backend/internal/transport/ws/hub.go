
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"fleetview/internal/domain"
	"fleetview/internal/service"
	"fleetview/internal/transport/dto"
)

const (
	
	writeWait = 10 * time.Second
	
	pongWait   = 60 * time.Second
	pingPeriod = 25 * time.Second
	
	maxMessageSize = 4 << 10
	
	sendBuffer = 16
)


type FeedBuilder interface {
	Feed(ctx context.Context, userID string, q domain.DeviceQuery) (service.Feed, error)
}


type Envelope struct {
	Type string    `json:"type"`
	Data any       `json:"data,omitempty"`
	At   time.Time `json:"at"`
}


type Hub struct {
	builder  FeedBuilder
	log      *slog.Logger
	clock    domain.Clock
	upgrader websocket.Upgrader

	mu      sync.RWMutex
	clients map[*client]struct{}
}


type Options struct {
	Builder        FeedBuilder
	Logger         *slog.Logger
	Clock          domain.Clock
	AllowedOrigins []string
}


func NewHub(opts Options) *Hub {
	if opts.Clock == nil {
		opts.Clock = domain.SystemClock{}
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	allowed := make(map[string]bool, len(opts.AllowedOrigins))
	wildcard := false
	for _, origin := range opts.AllowedOrigins {
		origin = strings.TrimRight(strings.ToLower(strings.TrimSpace(origin)), "/")
		if origin == "*" {
			wildcard = true
		}
		if origin != "" {
			allowed[origin] = true
		}
	}

	hub := &Hub{
		builder: opts.Builder,
		log:     opts.Logger.With("component", "ws"),
		clock:   opts.Clock,
		clients: make(map[*client]struct{}),
	}
	hub.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 4096,
		
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				
				return true
			}
			if wildcard {
				return true
			}
			normalised := strings.TrimRight(strings.ToLower(origin), "/")
			if allowed[normalised] {
				return true
			}
			
			if u, err := url.Parse(origin); err == nil && strings.EqualFold(u.Host, r.Host) {
				return true
			}
			return false
		},
	}
	return hub
}


func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}


func (h *Hub) Publish(event string, payload any) {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	switch event {
	case service.EventFleetUpdated:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, c := range clients {
			h.sendFeed(ctx, c, event)
		}
	default:
		frame, err := json.Marshal(Envelope{Type: event, Data: payload, At: h.clock.Now().UTC()})
		if err != nil {
			h.log.Error("marshal event failed", "event", event, "error", err)
			return
		}
		for _, c := range clients {
			c.enqueue(frame)
		}
	}
}


func (h *Hub) sendFeed(ctx context.Context, c *client, event string) {
	feed, err := h.builder.Feed(ctx, c.userID, c.snapshotQuery())
	if err != nil {
		h.log.Error("build feed for websocket client failed", "user", c.userID, "error", err)
		return
	}
	frame, err := json.Marshal(Envelope{
		Type: event,
		Data: dto.FromFeed(feed, h.clock.Now()),
		At:   h.clock.Now().UTC(),
	})
	if err != nil {
		h.log.Error("marshal feed failed", "error", err)
		return
	}
	c.enqueue(frame)
}


func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, userID string, query domain.DeviceQuery) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
	
		h.log.Warn("websocket upgrade failed", "error", err, "remote", r.RemoteAddr)
		return
	}

	c := &client{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, sendBuffer),
		userID: userID,
		query:  query,
	}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	count := len(h.clients)
	h.mu.Unlock()
	h.log.Info("websocket connected", "user", userID, "clients", count)

	go c.writePump()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	h.sendFeed(ctx, c, service.EventFleetUpdated)
	cancel()

	c.readPump()
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	count := len(h.clients)
	h.mu.Unlock()
	h.log.Info("websocket disconnected", "user", c.userID, "clients", count)
}

var _ domain.Publisher = (*Hub)(nil)
