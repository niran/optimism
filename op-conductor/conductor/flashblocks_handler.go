package conductor

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// StartFlashblocksHandler initializes the flashblocks handler and starts the rollup boost listener
func (oc *OpConductor) StartFlashblocksHandler(ctx context.Context) error {
	// Start the WebSocket server
	if err := oc.startWebSocketServer(ctx); err != nil {
		return err
	}

	// Start the rollup boost listener
	if err := oc.startRollupBoostListener(ctx); err != nil {
		return err
	}

	return nil
}

// startRollupBoostListener initializes and starts a WebSocket client to listen for rollup boost messages
func (oc *OpConductor) startRollupBoostListener(_ context.Context) error {
	if oc.cfg.RollupBoostWsURL == "" {
		oc.log.Info("rollup boost WebSocket disabled, no URL configured")
		return nil
	}

	oc.log.Info("connecting to rollup boost WebSocket", "url", oc.cfg.RollupBoostWsURL)

	// Connect to the rollup boost WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(oc.cfg.RollupBoostWsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to rollup boost WebSocket: %w", err)
	}

	oc.rollupBoostConn = conn
	oc.log.Info("connected to rollup boost WebSocket", "url", oc.cfg.RollupBoostWsURL)

	// Start a goroutine to read messages from the rollup boost WebSocket
	go func() {
		defer func() {
			oc.log.Info("closing rollup boost WebSocket connection")
			conn.Close()
			oc.rollupBoostConn = nil
		}()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				oc.log.Error("error reading from rollup boost WebSocket", "err", err)
				return
			}

			oc.handleRollupBoostMessage(message)
		}
	}()

	return nil
}

// handleRollupBoostMessage processes a message received from rollup boost
func (oc *OpConductor) handleRollupBoostMessage(message []byte) {
	// Only forward messages if we're the leader
	if !oc.leader.Load() {
		oc.log.Debug("not forwarding rollup boost message, not the leader")
		return
	}

	// Forward the message to connected clients
	oc.broadcastToClients(message)
}

// startWebSocketServer initializes and starts a WebSocket server
func (oc *OpConductor) startWebSocketServer(ctx context.Context) error {
	if oc.cfg.WebsocketServerPort <= 0 {
		oc.log.Info("WebSocket server disabled, no port configured")
		return nil
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all connections
		},
	}

	// Create HTTP server with WebSocket endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			oc.log.Error("failed to upgrade connection", "err", err)
			return
		}

		// Store the client connection
		oc.wsClientMu.Lock()
		// If we already have a connection, reject the new one
		if oc.wsClient != nil {
			oc.wsClientMu.Unlock()
			oc.log.Warn("rejecting new WebSocket connection, already have one", "remote", conn.RemoteAddr())
			conn.Close()
			return
		}
		oc.wsClient = conn
		oc.wsClientMu.Unlock()

		oc.log.Info("WebSocket proxy connected", "remote", conn.RemoteAddr())

		// Create a channel to detect connection errors
		errCh := make(chan error, 1)

		// Start a goroutine to read messages (just to detect disconnection)
		go func() {
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					errCh <- err
					return
				}
			}
		}()

		// Wait for either context cancellation or connection error
		select {
		case <-ctx.Done():
			// Context cancelled, conductor is shutting down
			oc.log.Info("closing WebSocket connection due to shutdown", "remote", conn.RemoteAddr())
		case err := <-errCh:
			// Connection error occurred
			oc.log.Warn("WebSocket connection error", "err", err, "remote", conn.RemoteAddr())
		}

		// Clean up the connection
		oc.wsClientMu.Lock()
		if oc.wsClient == conn {
			oc.wsClient = nil
		}
		oc.wsClientMu.Unlock()
		conn.Close()
		oc.log.Info("WebSocket proxy disconnected", "remote", conn.RemoteAddr())
	})

	// Start HTTP server
	server := &http.Server{
		Addr: fmt.Sprintf(":%d", oc.cfg.WebsocketServerPort),
	}

	oc.log.Info("starting WebSocket server", "port", oc.cfg.WebsocketServerPort)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			oc.log.Error("WebSocket server error", "err", err)
		}
	}()

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			oc.log.Error("error shutting down WebSocket server", "err", err)
		}
	}()

	return nil
}

// broadcastToClients sends a message to all connected WebSocket clients
func (oc *OpConductor) broadcastToClients(message []byte) {
	oc.wsClientMu.Lock()
	defer oc.wsClientMu.Unlock()

	if oc.wsClient == nil {
		oc.log.Debug("no WebSocket clients connected, not broadcasting message")
		return
	}

	oc.log.Info("Broadcasting message to WebSocket clients", "message", string(message))

	err := oc.wsClient.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		oc.log.Error("error writing to WebSocket client", "err", err)
		// Close the connection on error
		oc.wsClient.Close()
		oc.wsClient = nil
	}
}

// CloseConnections closes any open WebSocket connections
// This should only be called during shutdown
func (oc *OpConductor) CloseConnections() {
	if oc.rollupBoostConn != nil {
		oc.rollupBoostConn.Close()
		oc.rollupBoostConn = nil
	}

	// Close WebSocket proxy connection
	oc.wsClientMu.Lock()
	if oc.wsClient != nil {
		oc.wsClient.Close()
		oc.wsClient = nil
	}
	oc.wsClientMu.Unlock()
}
