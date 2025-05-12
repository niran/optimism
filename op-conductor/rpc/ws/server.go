package ws

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/gorilla/websocket"
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
}

// newHub creates a new hub
func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte, broadcastBufferSize),
		register:   make(chan *Client),
		unregister: make(chan *Client, unregisterBufferSize),
		clients:    make(map[*Client]bool),
		done:       make(chan struct{}),
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
			h.clientsMu.Unlock()
			log.Info("Client registered with hub", "remote", client.conn.RemoteAddr(), "totalClients", clientCount)
		case client := <-h.unregister:
			h.clientsMu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.clientsMu.Unlock()
		case message := <-h.broadcast:
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
}

// Close closes the client's WebSocket connection and send channel
func (c *Client) Close() {
	close(c.send)
	c.conn.Close()
}

// newClient creates a new client with default settings
func newClient(conn *websocket.Conn) *Client {
	return &Client{
		conn: conn,
		send: make(chan []byte, clientSendBufferSize),
	}
}

// serveWs handles WebSocket requests from clients
func (h *Handler) serveWs(w http.ResponseWriter, r *http.Request, upgrader websocket.Upgrader) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("failed to upgrade connection", "err", err)
		return
	}

	client := newClient(conn)

	// Register the client with the hub
	h.hub.register <- client
	h.log.Info("WebSocket proxy connected", "remote", conn.RemoteAddr())

	// Start the write pump in a separate goroutine
	go h.writePump(client)

	// Start the read pump in this goroutine
	h.readPump(client)
}

// readPump pumps messages from the WebSocket connection to the hub
func (h *Handler) readPump(client *Client) {
	defer func() {
		// Unregister the client when the read pump exits
		h.hub.unregister <- client
		client.Close()
		h.log.Info("WebSocket read pump exited, client unregistered", "remote", client.conn.RemoteAddr())
	}()

	// Set up pong handler to respond to pings
	client.conn.SetPongHandler(func(string) error {
		// Reset read deadline when we get a pong
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		// Check if we're the leader
		ctx := context.Background()
		isLeader := h.isLeaderFn(ctx)

		if isLeader {
			// As leader, we expect to be sending blocks regularly
			// Set a timeout to detect if something is wrong
			client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		} else {
			// As non-leader, we don't expect to send anything
			// Set a longer timeout to allow for ping/pong
			client.conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		}

		// Read from the connection
		_, _, err := client.conn.ReadMessage()

		if err != nil {
			if isLeader {
				// If we're the leader and there's an error, log and break
				h.log.Warn("Error reading from WebSocket client as leader", "err", err, "remote", client.conn.RemoteAddr())
				break
			} else if websocket.IsUnexpectedCloseError(err) {
				// If it's an unexpected close, log and break
				h.log.Warn("WebSocket connection closed unexpectedly", "err", err, "remote", client.conn.RemoteAddr())
				break
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// If it's a timeout and we're not the leader, just log and continue
				h.log.Debug("Read timeout as non-leader, continuing", "remote", client.conn.RemoteAddr())
				continue
			} else {
				// For other errors, log and break
				h.log.Warn("Error reading from WebSocket client", "err", err, "remote", client.conn.RemoteAddr())
				break
			}
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (h *Handler) writePump(client *Client) {
	defer func() {
		// Unregister client if not already done by readPump
		select {
		case h.hub.unregister <- client:
			h.log.Debug("Unregistered client from writePump", "remote", client.conn.RemoteAddr())
		default:
			// If channel is full or client already unregistered, just log
			h.log.Debug("Client may already be unregistered", "remote", client.conn.RemoteAddr())
		}

		// Close the client connection
		client.Close()
	}()

	// Configure ping for connection keepalive
	pingPeriod := 30 * time.Second
	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()

	// Track consecutive failures
	consecutiveFailures := 0
	maxConsecutiveFailures := 5 // Allow up to 5 consecutive failures before disconnecting

	for {
		select {
		case message, ok := <-client.send:
			if !ok {
				// The hub closed the channel, exit the write pump
				h.log.Debug("Client send channel closed", "remote", client.conn.RemoteAddr())
				return
			}

			// Set a short write deadline to prevent slow clients from blocking
			client.conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
			err := client.conn.WriteMessage(websocket.TextMessage, message)

			if err != nil {
				consecutiveFailures++
				h.log.Warn("error writing to WebSocket client",
					"err", err,
					"remote", client.conn.RemoteAddr(),
					"consecutiveFailures", consecutiveFailures,
					"maxAllowed", maxConsecutiveFailures)

				// Only break the loop if we've had too many consecutive failures
				if consecutiveFailures >= maxConsecutiveFailures {
					h.log.Error("too many consecutive write failures, disconnecting client",
						"remote", client.conn.RemoteAddr(),
						"failures", consecutiveFailures)
					return
				}

				// Continue processing other messages
				continue
			}

			// Reset failure counter on successful write
			if consecutiveFailures > 0 {
				h.log.Debug("successful write after previous failures",
					"remote", client.conn.RemoteAddr(),
					"resetFailureCount", consecutiveFailures)
				consecutiveFailures = 0
			}

		case <-pingTicker.C:
			// Only send ping if we're not the leader
			ctx := context.Background()
			if !h.isLeaderFn(ctx) {
				// Send a ping to keep the connection alive
				client.conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
				if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					h.log.Warn("error sending ping to client", "err", err, "remote", client.conn.RemoteAddr())
					consecutiveFailures++

					// Only break the loop if we've had too many consecutive failures
					if consecutiveFailures >= maxConsecutiveFailures {
						h.log.Error("too many consecutive ping failures, disconnecting client",
							"remote", client.conn.RemoteAddr(),
							"failures", consecutiveFailures)
						return
					}
				} else {
					// Reset failure counter on successful ping
					if consecutiveFailures > 0 {
						consecutiveFailures = 0
					}
				}
			}
		}
	}
}
