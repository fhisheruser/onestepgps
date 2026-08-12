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


type client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID string

	mu    sync.RWMutex
	query domain.DeviceQuery
}


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


func (c *client) enqueue(frame []byte) {
	defer func() {
		
		_ = recover()
	}()

	select {
	case c.send <- frame:
	default:
		c.hub.log.Warn("dropping slow websocket client", "user", c.userID)
		_ = c.conn.Close()
	}
}


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
			
			c.setQuery(domain.DeviceQuery{
				Search:        msg.Data.Search,
				Status:        domain.StatusFilter(strings.ToLower(msg.Data.Status)),
				SortKey:       domain.SortKey(strings.ToLower(msg.Data.SortKey)),
				SortDirection: domain.SortDirection(strings.ToLower(msg.Data.SortDirection)),
				IncludeHidden: msg.Data.IncludeHidden,
				OnlyPinned:    msg.Data.OnlyPinned,
			})
		case "refresh":
			
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
