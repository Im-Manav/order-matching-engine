package ws

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/Im-Manav/ome/pkg/logger"
	"github.com/Im-Manav/ome/pkg/models"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins in development.
	// In production restrict this to your frontend domain.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client represents one connected WebSocket browser tab.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte // buffered outbound message channel
	symbol string      // symbol this client is subscribed to ("" = all)
}

// Hub manages all connected WebSocket clients.
// It implements ports.Broadcaster.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			logger.Info("websocket client connected",
				zap.String("symbol", client.symbol),
				zap.Int("total_clients", len(h.clients)),
			)

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				logger.Info("websocket client disconnected",
					zap.Int("total_clients", len(h.clients)),
				)
			}

		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// BroadcastTrade implements ports.Broadcaster.
// Serialises the trade event and sends to all connected clients.
func (h *Hub) BroadcastTrade(event models.TradeEvent) {
	msg := wsMessage{Type: "orderbook", Payload: event}
	h.broadcastJSON(msg)
}

// BroadcastOrderBookUpdate implements ports.Broadcaster.
func (h *Hub) BroadcastOrderBookUpdate(snap models.OrderBookSnapshot) {
	msg := wsMessage{Type: "orderbook", Payload: snap}
	h.broadcastJSON(msg)
}

// wsMessage is the envelope sent to every WebSocket client.
// Type tells the frontend which component to update.
type wsMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

func (h *Hub) broadcastJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		logger.Error("ws broadcast marshal failed", logger.Err(err))
		return
	}
	select {
	case h.broadcast <- data:
	default:
		logger.Warn("ws broadcast channel full - dropping message")
	}
}

// ClientCount returns the number of connected clients.
// Used by Prometheus metrics.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeWS upgrades the HTTP connection to WebSocket and registers the client.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, symbol string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("ws upgrade failed", logger.Err(err))
		return
	}

	client := &Client{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 256),
		symbol: symbol,
	}

	h.register <- client

	// Each client gets two goroutines:
	// writePump — sends messages from hub to browser
	// readPump  — reads pings from browser (keeps connection alive)
	go client.writePump()
	go client.readPump()
}

// readPump handles incoming messages from the browser.
// We only expect pong responses — any other message is ignored.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				logger.Error("ws unexpected close", logger.Err(err))
			}
			break
		}
	}
}

// writePump sends messages from the hub to the browser.
// Also sends periodic pings to detect dead connections.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
