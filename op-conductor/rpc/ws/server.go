package ws

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/ethereum/go-ethereum/log"
)

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// Registered clients
	clients map[*Client]bool
	// Mutex to protect access to clients map
	clientsMu sync.RWMutex

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Inbound messages to broadcast to the clients
	broadcast chan []byte

	// Signal to stop the hub
	done chan struct{}

	// Logger
	log log.Logger
}

// newHub creates a new hub
func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		done:       make(chan struct{}),
		log:        log.New("component", "websocket-hub"),
	}
}

// run starts the hub's main loop
func (h *Hub) run() {
	for {
		select {
		case <-h.done:
			// Close all client connections
			h.clientsMu.Lock()
			for client := range h.clients {
				client.Close()
				delete(h.clients, client)
			}
			h.clientsMu.Unlock()
			return
		case client := <-h.register:
			h.clientsMu.Lock()
			h.clients[client] = true
			clientCount := len(h.clients)
			h.log.Info("Client registered with hub", "totalClients", clientCount)
			h.clientsMu.Unlock()
		case client := <-h.unregister:
			h.clientsMu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.clientsMu.Unlock()
		case message := <-h.broadcast:
			maxClientFailures := 5
			h.clientsMu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
					client.failureCount = 0
				default:
					client.failureCount++
					if client.failureCount >= maxClientFailures {
						delete(h.clients, client)
						client.Close()
					}
				}
			}
			h.clientsMu.Unlock()
		}
	}
}

// Client represents a connected WebSocket client
type Client struct {
	conn         *websocket.Conn
	send         chan []byte
	failureCount int
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.Mutex // protects closed flag
	closed       bool
	hub          *Hub
	log          log.Logger
}

// Close closes the client's WebSocket connection and send channel
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.closed {
		c.closed = true
		c.cancel()
		c.conn.Close(websocket.StatusNormalClosure, "client closed")
		close(c.send)
	}
}

// newClient creates a new client with default settings
func newClient(conn *websocket.Conn, ctx context.Context, hub *Hub, logger log.Logger) *Client {
	ctx, cancel := context.WithCancel(ctx)
	return &Client{
		conn:   conn,
		send:   make(chan []byte, 256),
		ctx:    ctx,
		cancel: cancel,
		hub:    hub,
		log:    logger,
	}
}

// serveWs handles WebSocket requests from clients
func (h *Handler) serveWs(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection using coder/websocket
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
	})

	if err != nil {
		h.log.Error("failed to upgrade connection", "err", err)
		return
	}

	client := newClient(conn, r.Context(), h.hub, h.log)

	// Register the client with the hub
	h.hub.register <- client
	h.log.Info("WebSocket client connected")

	// Start client handling
	go h.writePump(client)
	h.readPump(client)
}

// readPump pumps messages from the WebSocket connection to the hub
func (h *Handler) readPump(client *Client) {
	defer func() {
		// Unregister the client when the read pump exits
		h.hub.unregister <- client
		h.log.Info("WebSocket read pump exited, client unregistered")
	}()

	for {
		// Check if context is done
		select {
		case <-client.ctx.Done():
			// If the context is done, exit the read pump
			return
		default:
			// If the context is not done, continue
		}

		// Check if we're the leader
		isLeader := h.isLeaderFn(client.ctx)

		// Read with timeout (same for both leader and non-leader)
		readCtx, cancel := context.WithTimeout(client.ctx, 60*time.Second)
		messageType, _, err := client.conn.Read(readCtx)
		cancel()

		if err != nil {
			if isLeader {
				// Leader: exit on any error for strict availability
				h.log.Warn("Error reading from WebSocket client as leader", "err", err)
				return
			} else if errors.Is(err, context.DeadlineExceeded) {
				// Non-leader: continue on timeout, but exit on real errors
				h.log.Debug("Read timeout as non-leader, continuing")
				continue
			} else if websocket.CloseStatus(err) != -1 {
				// Normal close for both
				h.log.Info("Client closed connection", "code", websocket.CloseStatus(err))
				return
			} else {
				// Non-leader: exit on actual connection errors
				h.log.Warn("Error reading from WebSocket client", "err", err)
				return
			}
		}

		// Process message if needed (we usually don't expect application messages)
		if messageType == websocket.MessageText || messageType == websocket.MessageBinary {
			h.log.Debug("Received message from client", "type", messageType)
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (h *Handler) writePump(client *Client) {
	defer func() {
		client.Close()
	}()

	// Configure ping for connection keepalive
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case <-client.ctx.Done():
			return

		case message, ok := <-client.send:
			if !ok {
				// The hub closed the channel, exit the write pump
				h.log.Debug("Client send channel closed")
				return
			}

			// Write with timeout
			writeCtx, cancel := context.WithTimeout(client.ctx, writeTimeout)
			err := client.conn.Write(writeCtx, websocket.MessageText, message)
			cancel()

			if err != nil {
				h.log.Warn("Error writing to client", "err", err)
				return
			}

			// Reset failure count on successful write
			client.failureCount = 0

		case <-pingTicker.C:
			// Only send ping if we're not the leader
			if !h.isLeaderFn(client.ctx) {
				pingCtx, cancel := context.WithTimeout(client.ctx, pongTimeout)
				err := client.conn.Ping(pingCtx)
				cancel()

				if err != nil {
					h.log.Warn("Ping error", "err", err)
					return
				}
			}
		}
	}
}
