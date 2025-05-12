package ws

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

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
				if retryCount >= maxReconnectAttempts {
					h.log.Error("exceeded maximum reconnection attempts to rollup boost WebSocket", "maxRetries", maxReconnectAttempts)
					return
				}

				h.log.Info("attempting to connect to rollup boost WebSocket", "url", h.cfg.RollupBoostWsURL, "attempt", retryCount+1)
				conn, _, err := websocket.DefaultDialer.Dial(h.cfg.RollupBoostWsURL, nil)
				if err != nil {
					retryCount++
					h.log.Warn("failed to connect to rollup boost WebSocket, will retry", "err", err, "retryIn", reconnectDelay)
					time.Sleep(reconnectDelay)
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
