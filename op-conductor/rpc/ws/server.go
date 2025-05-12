package ws

import (
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
			h.clientsMu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
					// Message sent successfully
					client.failureCount = 0 // Reset failure count on success
				default:
					// Client send buffer is full, increment failure count
					client.failureCount++
					if client.failureCount >= maxClientFailures {
						// Try to unregister, but don't block if channel is full
						select {
						case h.unregister <- client:
							// Successfully queued for unregistration
						default:
							// Unregister channel is full, we'll try again next loop
							log.Warn("Unregister channel full, client will be retried next loop",
								"remote", client.conn.RemoteAddr())
						}
					}
				}
			}
			h.clientsMu.RUnlock()
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

// writePump pumps messages from the hub to the WebSocket connection
func (h *Handler) writePump(client *Client) {
	defer func() {
		// The client.Close() will be called here
		// The readPump will handle unregistering the client
	}()

	for message := range client.send {
		// Set a short write deadline to prevent slow clients from blocking
		client.conn.SetWriteDeadline(time.Now().Add(1 * time.Second))

		err := client.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			h.log.Error("error writing to WebSocket client", "err", err, "remote", client.conn.RemoteAddr())
			// Break the loop, which will trigger the defer and close the client
			break
		}
	}
}

// readPump pumps messages from the WebSocket connection to the hub
func (h *Handler) readPump(client *Client) {
	defer func() {
		// Unregister the client when the read pump exits
		h.hub.unregister <- client
		client.Close()
		h.log.Info("WebSocket proxy disconnected", "remote", client.conn.RemoteAddr())
	}()

	// Set read deadline to detect closed connections
	for {
		client.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		_, _, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err) || !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				h.log.Warn("WebSocket connection closed by client", "err", err, "remote", client.conn.RemoteAddr())
			}
			break
		}
	}
}
