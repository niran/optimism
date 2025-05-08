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

// Handler implements the FlashblockHandler interface
type Handler struct {
	cfg             Config
	log             log.Logger
	isLeaderFn      func(context.Context) bool
	wsClientMu      sync.Mutex
	rollupBoostConn *websocket.Conn
	server          *http.Server
	wsClients       []*websocket.Conn
}

// NewHandler creates a new flashblocks handler
func NewHandler(cfg Config, log log.Logger, isLeaderFn func(context.Context) bool) FlashblockHandler {
	return &Handler{
		cfg:        cfg,
		log:        log,
		isLeaderFn: isLeaderFn,
	}
}

// Start initializes the flashblocks handler and starts the WebSocket server and rollup boost listener
func (h *Handler) Start(ctx context.Context) error {
	// Start the WebSocket server
	if err := h.startWebSocketServer(ctx); err != nil {
		return err
	}

	// Start the rollup boost listener
	if err := h.startRollupBoostListener(ctx); err != nil {
		return err
	}

	return nil
}

// Stop closes any open WebSocket connections and shuts down the server
// This should only be called if the conductor is shut down
func (h *Handler) Stop() {
	// Close the rollup boost connection if it exists
	if h.rollupBoostConn != nil {
		h.rollupBoostConn.Close()
		h.rollupBoostConn = nil
	}

	// Close all WebSocket client connections
	h.wsClientMu.Lock()
	for _, client := range h.wsClients {
		client.Close()
	}
	h.wsClients = nil
	h.wsClientMu.Unlock()

	// Force close the HTTP server if it's running
	if h.server != nil {
		h.log.Info("Forcibly closing WebSocket server")
		err := h.server.Close()
		if err != nil {
			h.log.Error("Error closing WebSocket server", "err", err)
		}
		h.log.Info("WebSocket server closed")
	}
}

// BroadcastMessage sends a message to all connected WebSocket clients
func (h *Handler) BroadcastMessage(message []byte) {
	h.wsClientMu.Lock()
	defer h.wsClientMu.Unlock()

	if len(h.wsClients) == 0 {
		h.log.Debug("no WebSocket clients connected, not broadcasting message")
		return
	}

	h.log.Info("Broadcasting message to WebSocket clients", "clientCount", len(h.wsClients))

	// Send message to all clients
	for _, client := range h.wsClients {
		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			h.log.Error("error writing to WebSocket client", "err", err, "remote", client.RemoteAddr())
			// Just log the error but keep the connection
		}
	}
}

// startRollupBoostListener initializes and starts a WebSocket client to listen for rollup boost messages
func (h *Handler) startRollupBoostListener(ctx context.Context) error {
	if h.cfg.RollupBoostWsURL == "" {
		h.log.Info("rollup boost WebSocket disabled, no URL configured")
		return nil
	}

	// Start a goroutine to maintain the rollup boost WebSocket connection
	go func() {
		backoff := time.Second
		maxBackoff := 30 * time.Second

		for {
			select {
			case <-ctx.Done():
				if h.rollupBoostConn != nil {
					h.log.Info("closing rollup boost WebSocket connection due to context cancellation")
					h.rollupBoostConn.Close()
					h.rollupBoostConn = nil
				}
				return
			default:
				// Try to connect if not connected
				if h.rollupBoostConn == nil {
					conn, _, err := websocket.DefaultDialer.Dial(h.cfg.RollupBoostWsURL, nil)
					if err != nil {
						h.log.Error("failed to connect to rollup boost WebSocket, will retry", "err", err, "backoff", backoff)
						time.Sleep(backoff)
						// After each failed connection attempt, increases the wait time by multiplying by 1.5
						// Caps the maximum wait time at 30 seconds
						backoff = time.Duration(min(float64(backoff)*1.5, float64(maxBackoff)))
						continue
					}
					h.rollupBoostConn = conn
					h.log.Info("connected to rollup boost WebSocket", "url", h.cfg.RollupBoostWsURL)
					backoff = time.Second // Reset backoff on successful connection
				}

				// Read messages from the connection
				_, message, err := h.rollupBoostConn.ReadMessage()
				if err != nil {
					h.log.Error("error reading from rollup boost WebSocket, reconnecting", "err", err)
					h.rollupBoostConn.Close()
					h.rollupBoostConn = nil
					continue
				}

				h.handleRollupBoostMessage(ctx, message)
			}
		}
	}()

	return nil
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

// handleWebSocketRequest processes new WebSocket connection requests
func (h *Handler) handleWebSocketRequest(w http.ResponseWriter, r *http.Request, upgrader websocket.Upgrader, ctx context.Context) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("failed to upgrade connection", "err", err)
		return
	}

	// Store the client connection
	h.wsClientMu.Lock()
	// Add the new connection to our list of clients
	h.wsClients = append(h.wsClients, conn)
	h.wsClientMu.Unlock()

	h.log.Info("WebSocket proxy connected", "remote", conn.RemoteAddr())

	// Handle the connection in a separate goroutine
	go h.handleWebSocketConnection(ctx, conn)
}

// startWebSocketServer initializes and starts a WebSocket server
func (h *Handler) startWebSocketServer(ctx context.Context) error {
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
		h.handleWebSocketRequest(w, r, upgrader, ctx)
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

// handleWebSocketConnection manages a WebSocket connection until it's closed
func (h *Handler) handleWebSocketConnection(ctx context.Context, conn *websocket.Conn) {
	defer func() {
		// Clean up the connection
		h.wsClientMu.Lock()
		// Remove this connection from our list of clients
		for i, client := range h.wsClients {
			if client == conn {
				// Remove this client from the slice
				h.wsClients = append(h.wsClients[:i], h.wsClients[i+1:]...)
				break
			}
		}
		h.wsClientMu.Unlock()
		conn.Close()
		h.log.Info("WebSocket proxy disconnected", "remote", conn.RemoteAddr())
	}()

	// Read messages until an error occurs (connection closed) or context is cancelled
	for {
		select {
		case <-ctx.Done():
			// Context cancelled, conductor is shutting down
			h.log.Info("closing WebSocket connection due to shutdown", "remote", conn.RemoteAddr())
			return
		default:
			// Try to read a message with a timeout to periodically check context
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err) || !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					h.log.Warn("WebSocket connection error", "err", err, "remote", conn.RemoteAddr())
				}
				return
			}
			// Clear the deadline for the next iteration
			conn.SetReadDeadline(time.Time{})
		}
	}
}
