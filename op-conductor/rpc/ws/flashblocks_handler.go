package ws

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/gorilla/websocket"
)

// FlashblockHandler manages WebSocket connections for flashblocks
type FlashblockHandler interface {
	// Start initializes and starts the flashblocks handler
	Start(ctx context.Context) error
	// Stop closes any open WebSocket connections
	Stop()
	// BroadcastMessage sends a message to all connected WebSocket clients
	BroadcastMessage(message []byte)
}

// Config contains configuration for the flashblocks handler
type Config struct {
	// WebsocketServerPort is the port to listen for WebSocket connections
	WebsocketServerPort int
	// RollupBoostWsURL is the URL of the rollup boost WebSocket
	RollupBoostWsURL string
}

// Client represents a connected WebSocket client
type Client struct {
	conn         *websocket.Conn
	send         chan []byte
	failureCount int
	maxFailures  int
}

// Close closes the client's WebSocket connection and send channel
func (c *Client) Close() {
	close(c.send)
	c.conn.Close()
}

// newClient creates a new client with default settings
func newClient(conn *websocket.Conn) *Client {
	return &Client{
		conn:        conn,
		send:        make(chan []byte, 1), // Buffer size of 1
		maxFailures: 5,                    // Drop client after 5 consecutive failures
	}
}

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
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client, 64), // Add buffer to handle multiple unregister events
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
			h.clientsMu.Unlock()
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
					if client.failureCount >= client.maxFailures {
						// Send to unregister channel - now non-blocking with buffered channel
						h.unregister <- client
					}
				}
			}
			h.clientsMu.RUnlock()
		}
	}
}

// Handler implements the FlashblockHandler interface
type Handler struct {
	cfg             Config
	log             log.Logger
	isLeaderFn      func(context.Context) bool
	rollupBoostConn *websocket.Conn
	server          *http.Server
	hub             *Hub
}

// NewHandler creates a new flashblocks handler
func NewHandler(cfg Config, log log.Logger, isLeaderFn func(context.Context) bool) (FlashblockHandler, error) {
	// Create the hub
	hub := newHub()

	// Initialize the handler
	handler := &Handler{
		cfg:        cfg,
		log:        log,
		isLeaderFn: isLeaderFn,
		hub:        hub,
	}

	// Establish connection to rollup boost if URL is configured
	if cfg.RollupBoostWsURL != "" {
		log.Info("connecting to rollup boost WebSocket", "url", cfg.RollupBoostWsURL)
		conn, _, err := websocket.DefaultDialer.Dial(cfg.RollupBoostWsURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to rollup boost WebSocket: %w", err)
		}
		handler.rollupBoostConn = conn
		log.Info("connected to rollup boost WebSocket", "url", cfg.RollupBoostWsURL)
	} else {
		log.Info("rollup boost WebSocket disabled, no URL configured")
	}

	return handler, nil
}

// Start initializes the flashblocks handler and starts the WebSocket server and rollup boost listener
func (h *Handler) Start(ctx context.Context) error {
	go h.hub.run()

	// Start the WebSocket server
	if err := h.startWebSocketServer(ctx); err != nil {
		return err
	}

	// Start the rollup boost listener if connection exists
	if h.rollupBoostConn != nil {
		go h.listenToRollupBoost(ctx)
	}

	return nil
}

// Stop closes any open WebSocket connections and shuts down the server
func (h *Handler) Stop() {
	// Signal the hub to stop
	close(h.hub.done)

	// Close the rollup boost connection if it exists
	if h.rollupBoostConn != nil {
		h.log.Info("closing rollup boost WebSocket connection due to context cancellation")
		h.rollupBoostConn.Close()
		h.rollupBoostConn = nil
	}

	// Force close the HTTP server if it's running
	if h.server != nil {
		h.log.Info("closing WebSocket server")
		err := h.server.Close()
		if err != nil {
			h.log.Error("Error closing WebSocket server", "err", err)
		}
		h.log.Info("WebSocket server closed")
	}
}

// BroadcastMessage sends a message to all connected WebSocket clients
func (h *Handler) BroadcastMessage(message []byte) {
	h.hub.clientsMu.RLock()
	clientCount := len(h.hub.clients)
	h.hub.clientsMu.RUnlock()

	if clientCount == 0 {
		h.log.Debug("no WebSocket clients connected, not broadcasting message")
		return
	}

	h.log.Trace("Broadcasting message to WebSocket clients", "clientCount", clientCount)
	h.hub.broadcast <- message
}

// listenToRollupBoost listens for messages from the rollup boost WebSocket
func (h *Handler) listenToRollupBoost(ctx context.Context) {
	defer func() {
		if h.rollupBoostConn != nil {
			h.log.Info("closing rollup boost WebSocket connection")
			h.rollupBoostConn.Close()
			h.rollupBoostConn = nil
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.hub.done:
			return
		default:
			// Read messages from the connection
			_, message, err := h.rollupBoostConn.ReadMessage()
			if err != nil {
				h.log.Warn("error reading from rollup boost WebSocket", "err", err)
				// Connection error, exit the loop
				return
			}

			h.handleRollupBoostMessage(ctx, message)
		}
	}
}

// handleRollupBoostMessage processes a message received from rollup boost
func (h *Handler) handleRollupBoostMessage(ctx context.Context, message []byte) {
	// Only forward messages if we're the leader - check dynamically each time
	if !h.isLeaderFn(ctx) {
		h.log.Debug("not forwarding rollup boost message, not the leader")
		return
	}

	// Forward the message to connected clients
	h.BroadcastMessage(message)
}

// startWebSocketServer initializes and starts a WebSocket server
func (h *Handler) startWebSocketServer(_ context.Context) error {
	if h.cfg.WebsocketServerPort <= 0 {
		h.log.Info("WebSocket server disabled, no port configured")
		return nil
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all connections
		},
	}

	// Create HTTP server with WebSocket endpoint
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		h.serveWs(w, r, upgrader)
	})

	// Start HTTP server
	h.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", h.cfg.WebsocketServerPort),
		Handler: mux,
	}

	h.log.Info("starting WebSocket server", "port", h.cfg.WebsocketServerPort)
	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.log.Error("WebSocket server error", "err", err)
		}
	}()

	return nil
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
		client.Close()
	}()

	for message := range client.send {
		err := client.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			h.log.Error("error writing to WebSocket client", "err", err, "remote", client.conn.RemoteAddr())
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
