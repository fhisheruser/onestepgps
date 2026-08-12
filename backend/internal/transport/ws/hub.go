// Package ws implements the realtime push channel. Clients that can hold a
// WebSocket open receive fleet updates the moment the poller refreshes them;
// clients that cannot fall back to REST polling with no loss of functionality.
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
	// writeWait is how long a single write may block before the peer is
	// considered dead.
	writeWait = 10 * time.Second
	// pongWait must exceed pingPeriod, or healthy clients get disconnected.
	pongWait   = 60 * time.Second
	pingPeriod = 25 * time.Second
	// maxMessageSize bounds inbound control frames.
	maxMessageSize = 4 << 10
	// sendBuffer is how many pending messages a slow client may accumulate
	// before it is dropped, so one stalled browser cannot block the poller.
	sendBuffer = 16
)

// FeedBuilder is the read use case the hub needs. Depending on this narrow
// interface (rather than the concrete service) keeps the hub unit-testable.
type FeedBuilder interface {
	Feed(ctx context.Context, userID string, q domain.DeviceQuery) (service.Feed, error)
}

// Envelope is the frame format every server push uses.
type Envelope struct {
	Type string    `json:"type"`
	Data any       `json:"data,omitempty"`
	At   time.Time `json:"at"`
}

// Hub owns the set of live connections and fans updates out to them.
type Hub struct {
	builder  FeedBuilder
	log      *slog.Logger
	clock    domain.Clock
	upgrader websocket.Upgrader

	mu      sync.RWMutex
	clients map[*client]struct{}
}

// Options configures the hub.
type Options struct {
	Builder        FeedBuilder
	Logger         *slog.Logger
	Clock          domain.Clock
	AllowedOrigins []string
}

// NewHub builds a Hub.
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
		// A WebSocket is not subject to the same-origin policy, so the origin
		// must be checked explicitly or any site could open a socket to us.
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				// Non-browser clients (curl, tests, native apps) send no Origin.
				return true
			}
			if wildcard {
				return true
			}
			normalised := strings.TrimRight(strings.ToLower(origin), "/")
			if allowed[normalised] {
				return true
			}
			// Same-origin requests are always fine.
			if u, err := url.Parse(origin); err == nil && strings.EqualFold(u.Host, r.Host) {
				return true
			}
			return false
		},
	}
	return hub
}

// ClientCount reports how many sockets are currently connected.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Publish implements domain.Publisher. The poller calls it after every
// refresh; each client receives the feed rendered for *its* user and filters.
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

// sendFeed renders and queues the current feed for one client.
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

// Serve upgrades an HTTP request to a WebSocket and runs the connection until
// it closes. It is transport-agnostic so it can be mounted on any router.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, userID string, query domain.DeviceQuery) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade already wrote an error response.
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

	// Push the current state immediately so the UI paints without waiting for
	// the next poll tick.
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
