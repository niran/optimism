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

const (
	// MaxClientFailures is the number of consecutive failures before dropping a client
	MaxClientFailures = 5
	// MaxReconnectAttempts is the maximum number of reconnection attempts
	MaxReconnectAttempts = 10
	// ReconnectDelay is the delay between reconnection attempts
	ReconnectDelay = 5 * time.Second
	// ClientSendBufferSize is the buffer size for client send channels
	ClientSendBufferSize = 1
	// BroadcastBufferSize is the buffer size for the broadcast channel
	BroadcastBufferSize = 256
	// UnregisterBufferSize is the buffer size for the unregister channel
	UnregisterBufferSize = 64
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
		send: make(chan []byte, ClientSendBufferSize),
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
		broadcast:  make(chan []byte, BroadcastBufferSize),
		register:   make(chan *Client),
		unregister: make(chan *Client, UnregisterBufferSize),
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
					if client.failureCount >= MaxClientFailures {
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
	if cfg.RollupBoostWsURL == "" {
		log.Info("rollup boost WebSocket disabled, no URL configured")
		return handler, nil
	}

	log.Info("connecting to rollup boost WebSocket", "url", cfg.RollupBoostWsURL)
	conn, _, err := websocket.DefaultDialer.Dial(cfg.RollupBoostWsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rollup boost WebSocket: %w", err)
	}
	handler.rollupBoostConn = conn
	log.Info("connected to rollup boost WebSocket", "url", cfg.RollupBoostWsURL)

	return handler, nil
}

// Start initializes the flashblocks handler and starts the WebSocket server and rollup boost listener
func (h *Handler) Start(ctx context.Context) error {
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
	// Signal the hub to stop if it exists
	if h.hub != nil {
		close(h.hub.done)
	}

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
	h.log.Trace("Broadcasting message to WebSocket clients")
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

	retryCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.hub.done:
			return
		default:
			// Check if connection is nil and try to reconnect
			if h.rollupBoostConn == nil {
				if retryCount >= MaxReconnectAttempts {
					h.log.Error("exceeded maximum reconnection attempts to rollup boost WebSocket", "maxRetries", MaxReconnectAttempts)
					return
				}

				h.log.Info("attempting to connect to rollup boost WebSocket", "url", h.cfg.RollupBoostWsURL, "attempt", retryCount+1)
				conn, _, err := websocket.DefaultDialer.Dial(h.cfg.RollupBoostWsURL, nil)
				if err != nil {
					retryCount++
					h.log.Warn("failed to connect to rollup boost WebSocket, will retry", "err", err, "retryIn", ReconnectDelay)
					time.Sleep(ReconnectDelay)
					continue
				}

				h.rollupBoostConn = conn
				h.log.Info("successfully reconnected to rollup boost WebSocket")
				retryCount = 0 // Reset retry counter on successful connection
			}

			// Read messages from the connection
			_, message, err := h.rollupBoostConn.ReadMessage()
			if err != nil {
				h.log.Warn("error reading from rollup boost WebSocket", "err", err)
				// Close the connection and try to reconnect
				h.rollupBoostConn.Close()
				h.rollupBoostConn = nil
				continue
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

	// Create the hub for this WebSocket server
	h.hub = newHub()
	// Start the hub
	go h.hub.run()

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
