package conductor

import (
	"context"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

// StartFlashblocksHandler initializes the flashblocks handler and starts the rollup boost listener
func (oc *OpConductor) StartFlashblocksHandler(ctx context.Context) error {
	// Start the rollup boost listener
	if err := oc.startRollupBoostListener(ctx); err != nil {
		return err
	}

	return nil
}

// startRollupBoostListener establishes a WebSocket connection to the rollup boost service
// and listens for messages to forward to the websocket proxy when this node is the leader
func (oc *OpConductor) startRollupBoostListener(ctx context.Context) error {
	if oc.cfg.RollupBoostWsURL == "" {
		oc.log.Info("rollup boost listener disabled, no WebSocket URL configured")
		return nil
	}

	// Establish WebSocket connection to rollup boost
	rollupBoostConn, _, err := websocket.DefaultDialer.Dial(oc.cfg.RollupBoostWsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to rollup boost: %w", err)
	}

	// Keep track of the connection for later use
	oc.rollupBoostConn = rollupBoostConn

	// Also establish connection to websocket proxy if configured
	if oc.cfg.WebsocketProxyURL != "" {
		proxyConn, _, err := websocket.DefaultDialer.Dial(oc.cfg.WebsocketProxyURL, nil)
		if err != nil {
			oc.log.Warn("failed to connect to websocket proxy, will retry later", "err", err)
		} else {
			oc.proxyConn = proxyConn
			oc.log.Info("established connection to websocket proxy")
		}
	}

	// Start a goroutine to listen for messages
	go func() {
		defer rollupBoostConn.Close()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Read message from rollup boost
				_, message, err := rollupBoostConn.ReadMessage()
				if err != nil {
					oc.log.Error("error reading from rollup boost websocket", "err", err)

					// Try to reconnect
					time.Sleep(5 * time.Second)
					newConn, _, err := websocket.DefaultDialer.Dial(oc.cfg.RollupBoostWsURL, nil)
					if err != nil {
						oc.log.Error("failed to reconnect to rollup boost", "err", err)
						continue
					}

					rollupBoostConn = newConn
					oc.rollupBoostConn = newConn
					continue
				}

				// Process the message
				oc.handleRollupBoostMessage(message)
			}
		}
	}()

	return nil
}

// handleRollupBoostMessage processes messages received from rollup boost
// and forwards them to the websocket proxy if this node is the leader
func (oc *OpConductor) handleRollupBoostMessage(message []byte) {
	ctx := context.Background()

	// Only forward messages if this node is the leader
	if oc.Leader(ctx) && oc.cfg.WebsocketProxyURL != "" {
		// Ensure we have an active connection to the websocket proxy
		if oc.proxyConn == nil {
			// Establish connection to websocket proxy if not already connected
			conn, _, err := websocket.DefaultDialer.Dial(oc.cfg.WebsocketProxyURL, nil)
			if err != nil {
				oc.log.Error("failed to connect to websocket proxy", "err", err)
				return
			}
			oc.proxyConn = conn
			oc.log.Info("established connection to websocket proxy")
		}

		// Send the message to the proxy
		err := oc.proxyConn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			oc.log.Error("error sending message to websocket proxy", "err", err)
		}
	}
}

// CloseConnections closes any open WebSocket connections
// This should only be called during shutdown
func (oc *OpConductor) CloseConnections() {
	if oc.rollupBoostConn != nil {
		oc.rollupBoostConn.Close()
		oc.rollupBoostConn = nil
	}

	if oc.proxyConn != nil {
		oc.proxyConn.Close()
		oc.proxyConn = nil
	}
}
