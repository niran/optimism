package ws

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/gorilla/websocket"
)

const (
	// maxClientFailures is the number of consecutive failures before dropping a client
	maxClientFailures = 5
	// maxReconnectAttempts is the maximum number of reconnection attempts
	maxReconnectAttempts = 10
	// reconnectDelay is the delay between reconnection attempts
	reconnectDelay = 5 * time.Second
	// clientSendBufferSize is the buffer size for client send channels
	clientSendBufferSize = 1
	// broadcastBufferSize is the buffer size for the broadcast channel
	broadcastBufferSize = 256
	// unregisterBufferSize is the buffer size for the unregister channel
	unregisterBufferSize = 64
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

	// If rollup boost URL is not configured, return early
	if cfg.RollupBoostWsURL == "" {
		log.Info("rollup boost WebSocket disabled, no URL configured")
		return handler, nil
	}

	// retry logic for initial connection
	var conn *websocket.Conn
	var lastErr error

	// Create a context with timeout for the initial connection attempts
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to connect with retries
	for retryCount := range maxReconnectAttempts {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout connecting to rollup boost WebSocket: %w", ctx.Err())
		default:
			conn, _, lastErr = websocket.DefaultDialer.Dial(cfg.RollupBoostWsURL, nil)
			if lastErr == nil {
				// Successfully connected
				handler.rollupBoostConn = conn
				log.Info("connected to rollup boost WebSocket", "url", cfg.RollupBoostWsURL)
				return handler, nil
			}

			// Log the error and retry after delay
			log.Warn("failed to connect to rollup boost WebSocket, will retry",
				"err", lastErr,
				"attempt", retryCount+1,
				"maxAttempts", maxReconnectAttempts,
				"retryIn", reconnectDelay)

			// Don't sleep on the last attempt
			if retryCount < maxReconnectAttempts-1 {
				time.Sleep(reconnectDelay)
			}
		}
	}

	// If we've exhausted all retry attempts without success, return the error
	return nil, fmt.Errorf("failed to connect to rollup boost WebSocket after %d attempts: %w",
		maxReconnectAttempts, lastErr)
}

// BroadcastMessage sends a message to all connected WebSocket clients
func (h *Handler) BroadcastMessage(message []byte) {
	h.log.Trace("Broadcasting message to WebSocket clients")
	h.hub.broadcast <- message
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
