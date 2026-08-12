package ws

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"fleetview/internal/domain"
)

// client is one live browser connection.
type client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID string

	mu    sync.RWMutex
	query domain.DeviceQuery
}

// inboundMessage is the (small) control protocol a client may speak.
type inboundMessage struct {
	Type string `json:"type"`
	Data struct {
		Search        string `json:"search"`
		Status        string `json:"status"`
		SortKey       string `json:"sortKey"`
		SortDirection string `json:"sortDirection"`
		IncludeHidden bool   `json:"includeHidden"`
		OnlyPinned    bool   `json:"onlyPinned"`
	} `json:"data"`
}

// snapshotQuery returns a copy of the client's current filter.
func (c *client) snapshotQuery() domain.DeviceQuery {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.query
}

func (c *client) setQuery(q domain.DeviceQuery) {
	c.mu.Lock()
	c.query = q
	c.mu.Unlock()
}

// enqueue queues a frame, dropping the client if it cannot keep up. A slow
// consumer must never apply backpressure to the poller.
func (c *client) enqueue(frame []byte) {
	defer func() {
		// enqueue can race with hub.remove closing the channel; recovering is
		// cheaper than holding the hub lock for the whole fan-out.
		_ = recover()
	}()

	select {
	case c.send <- frame:
	default:
		c.hub.log.Warn("dropping slow websocket client", "user", c.userID)
		_ = c.conn.Close()
	}
}

// readPump consumes control frames and keeps the read deadline fresh. It also
// owns deregistration: when it returns, the connection is finished.
func (c *client) readPump() {
	defer func() {
		c.hub.remove(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.hub.log.Debug("websocket read ended", "user", c.userID, "error", err)
			}
			return
		}

		var msg inboundMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "query":
			// The client mirrors its list filters onto the socket so pushes
			// arrive pre-filtered and the UI never has to re-sort.
			c.setQuery(domain.DeviceQuery{
				Search:        msg.Data.Search,
				Status:        domain.StatusFilter(strings.ToLower(msg.Data.Status)),
				SortKey:       domain.SortKey(strings.ToLower(msg.Data.SortKey)),
				SortDirection: domain.SortDirection(strings.ToLower(msg.Data.SortDirection)),
				IncludeHidden: msg.Data.IncludeHidden,
				OnlyPinned:    msg.Data.OnlyPinned,
			})
		case "refresh":
			// Client asked for an out-of-band render (e.g. after saving a
			// preference) — answer from cache, no upstream call involved.
			go func() {
				ctx, cancel := contextWithTimeout()
				defer cancel()
				c.hub.sendFeed(ctx, c, "fleet.updated")
			}()
		case "ping":
			c.enqueue(mustFrame(Envelope{Type: "pong", At: time.Now().UTC()}))
		}
	}
}

// writePump serialises all writes onto the connection, which gorilla requires,
// and emits pings so dead peers are detected promptly.
func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case frame, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, frame); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// contextWithTimeout bounds an out-of-band feed render triggered by a client.
func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func mustFrame(env Envelope) []byte {
	frame, err := json.Marshal(env)
	if err != nil {
		return []byte(`{"type":"error"}`)
	}
	return frame
}
