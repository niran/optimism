package ws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/ethereum-optimism/optimism/op-conductor/metrics"
	"github.com/ethereum/go-ethereum/log"
)

// setupTestServer creates a test WebSocket server for ping/pong testing
func setupTestServer(_ *testing.T) (*Handler, *httptest.Server, func()) {
	// Create a mock config (we don't need rollup boost for ping/pong tests)
	cfg := Config{
		WebsocketServerPort: 8080,
		RollupBoostWsURL:    "ws://mock-url", // Not used in ping/pong tests
	}

	// Create logger
	logger := log.New("test", "ping-pong")

	// Mock leader function that always returns true
	isLeaderFn := func(ctx context.Context) bool { return true }

	// Create handler without establishing rollup boost connection
	handler := &Handler{
		cfg:        cfg,
		log:        logger,
		isLeaderFn: isLeaderFn,
		metrics:    &metrics.NoopMetricsImpl{},
	}

	// Create hub manually for testing
	handler.hub = newHub()
	go handler.hub.run()

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handler.handleWebSocket)
	server := httptest.NewServer(mux)

	// Cleanup function
	cleanup := func() {
		if handler.hub != nil {
			close(handler.hub.done)
		}
		server.Close()
	}

	return handler, server, cleanup
}

// TestPingPongMechanism tests the ping/pong keepalive mechanism
func TestPingPongMechanism(t *testing.T) {
	handler, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test complete")

	// Wait a bit for the client to be registered
	time.Sleep(200 * time.Millisecond)

	// Verify client is registered
	handler.hub.clientsMu.RLock()
	clientCount := len(handler.hub.clients)
	handler.hub.clientsMu.RUnlock()

	if clientCount != 1 {
		t.Errorf("Expected 1 client, got %d", clientCount)
		return
	}

	t.Log("Client successfully registered")

	// Set up connection monitoring
	connectionClosed := make(chan bool, 1)

	// Read messages in a separate goroutine to handle control frames and monitor connection
	go func() {
		defer func() {
			t.Log("Read goroutine exiting")
			connectionClosed <- true
		}()

		for {
			select {
			case <-ctx.Done():
				t.Log("Context cancelled in read goroutine")
				return
			default:
				// Use a shorter timeout for reads to be more responsive
				readCtx, readCancel := context.WithTimeout(ctx, time.Second*2)
				messageType, message, err := conn.Read(readCtx)
				readCancel()

				if err != nil {
					// Check if it's a close error
					if websocket.CloseStatus(err) != -1 {
						t.Logf("WebSocket connection closed: status=%d, err=%v", websocket.CloseStatus(err), err)
						return
					}
					// Check if it's a context cancellation
					if ctx.Err() != nil {
						t.Log("Context cancelled during read")
						return
					}
					// Timeout errors are expected when no messages are sent
					if !errors.Is(err, context.DeadlineExceeded) {
						t.Logf("Read error (non-timeout): %v", err)
						return
					}
					continue
				}

				// Log any messages received
				if messageType == websocket.MessageText || messageType == websocket.MessageBinary {
					t.Logf("Received message type: %v, content: %s", messageType, string(message))
				}
			}
		}
	}()

	// Monitor client count periodically
	monitorTicker := time.NewTicker(time.Second * 5)
	defer monitorTicker.Stop()

	testDuration := time.Second * 25 // Wait for at least one full ping cycle
	testTimer := time.NewTimer(testDuration)
	defer testTimer.Stop()

	t.Logf("Starting ping/pong monitoring for %v", testDuration)

	for {
		select {
		case <-testTimer.C:
			t.Log("Test duration completed")
			goto testComplete

		case <-connectionClosed:
			t.Log("Connection closed signal received")
			goto testComplete

		case <-monitorTicker.C:
			handler.hub.clientsMu.RLock()
			currentClientCount := len(handler.hub.clients)
			handler.hub.clientsMu.RUnlock()

			t.Logf("Current client count: %d", currentClientCount)

			if currentClientCount == 0 {
				t.Log("Client disconnected during monitoring")
				goto testComplete
			}
		}
	}

testComplete:
	// Final verification
	handler.hub.clientsMu.RLock()
	finalClientCount := len(handler.hub.clients)
	handler.hub.clientsMu.RUnlock()

	t.Logf("Final client count: %d", finalClientCount)

	if finalClientCount != 1 {
		t.Logf("Expected client to remain connected after ping/pong cycles, but client count is %d", finalClientCount)

		// Let's also check if there are any errors by trying to send a message
		testMsg := []byte("connection test")
		writeCtx, writeCancel := context.WithTimeout(context.Background(), time.Second*5)
		writeErr := conn.Write(writeCtx, websocket.MessageText, testMsg)
		writeCancel()

		if writeErr != nil {
			t.Logf("Connection appears to be closed: %v", writeErr)
		} else {
			t.Log("Connection is still writable")
		}

		// This is now a logged error rather than a hard failure to help debug
		t.Errorf("Ping/pong mechanism may not be working correctly")
	} else {
		t.Log("Ping/pong mechanism test completed successfully")
	}
}

// TestPingTimeout tests what happens when a client doesn't respond to pings
func TestPingTimeout(t *testing.T) {
	handler, server, cleanup := setupTestServer(t)
	defer cleanup()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect to the WebSocket server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}

	// Wait for client to be registered
	time.Sleep(100 * time.Millisecond)

	// Verify client is registered
	handler.hub.clientsMu.RLock()
	clientCount := len(handler.hub.clients)
	handler.hub.clientsMu.RUnlock()

	if clientCount != 1 {
		t.Errorf("Expected 1 client, got %d", clientCount)
	}

	// Simulate an unresponsive client by closing the connection abruptly
	// This should trigger the ping timeout mechanism
	conn.Close(websocket.StatusAbnormalClosure, "simulating unresponsive client")

	// Wait for the server to detect the dead connection
	// This might take up to pingInterval + pongTimeout
	maxWaitTime := time.Second * 30
	startTime := time.Now()

	for time.Since(startTime) < maxWaitTime {
		handler.hub.clientsMu.RLock()
		currentClientCount := len(handler.hub.clients)
		handler.hub.clientsMu.RUnlock()

		if currentClientCount == 0 {
			t.Log("Dead client successfully detected and removed")
			return
		}

		time.Sleep(time.Millisecond * 500)
	}

	// Check final state
	handler.hub.clientsMu.RLock()
	finalClientCount := len(handler.hub.clients)
	handler.hub.clientsMu.RUnlock()

	if finalClientCount != 0 {
		t.Errorf("Expected dead client to be removed, but %d clients remain", finalClientCount)
	}
}

// TestMultipleClientsPingPong tests ping/pong with multiple clients
func TestMultipleClientsPingPong(t *testing.T) {
	handler, server, cleanup := setupTestServer(t)
	defer cleanup()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	numClients := 3
	connections := make([]*websocket.Conn, numClients)

	// Connect multiple clients
	for i := 0; i < numClients; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		cancel()

		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		connections[i] = conn
	}

	// Clean up connections
	defer func() {
		for _, conn := range connections {
			if conn != nil {
				conn.Close(websocket.StatusNormalClosure, "test complete")
			}
		}
	}()

	// Wait for all clients to be registered
	time.Sleep(200 * time.Millisecond)

	// Verify all clients are registered
	handler.hub.clientsMu.RLock()
	clientCount := len(handler.hub.clients)
	handler.hub.clientsMu.RUnlock()

	if clientCount != numClients {
		t.Errorf("Expected %d clients, got %d", numClients, clientCount)
	}

	// Wait for ping/pong cycles
	time.Sleep(time.Second * 20)

	// Verify all clients are still connected
	handler.hub.clientsMu.RLock()
	finalClientCount := len(handler.hub.clients)
	handler.hub.clientsMu.RUnlock()

	if finalClientCount != numClients {
		t.Errorf("Expected %d clients to remain connected after ping/pong cycles, got %d", numClients, finalClientCount)
	}

	t.Logf("Multiple clients ping/pong test completed successfully with %d clients", numClients)
}
