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

// testConfig holds configurable timeouts and parameters for WebSocket tests
type testConfig struct {
	pingInterval    time.Duration
	pongTimeout     time.Duration
	testDuration    time.Duration
	setupDelay      time.Duration
	readTimeout     time.Duration
	monitorInterval time.Duration
}

// defaultTestConfig returns a testConfig with reasonable default values for testing
func defaultTestConfig() testConfig {
	return testConfig{
		pingInterval:    2 * time.Second,
		pongTimeout:     3 * time.Second,
		testDuration:    10 * time.Second,
		setupDelay:      200 * time.Millisecond,
		readTimeout:     2 * time.Second,
		monitorInterval: 2 * time.Second,
	}
}

// verifyClientCount verifies the number of connected clients
func verifyClientCount(t *testing.T, handler *Handler, expected int, msg string) {
	t.Helper()
	count := len(handler.hub.clients)
	if count != expected {
		t.Errorf("%s: expected %d client(s), got %d", msg, expected, count)
	}
}

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

	// Create test server with WebSocket handler
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
	cfg := defaultTestConfig()

	// Create channels for test synchronization
	pingReceived := make(chan struct{}, 1)
	pongReceived := make(chan struct{}, 1)

	// Setup client with ping/pong callbacks
	opts := &websocket.DialOptions{
		OnPingReceived: func(ctx context.Context, payload []byte) bool {
			t.Log("Client received ping")
			pingReceived <- struct{}{}
			return true // Send pong response
		},
		OnPongReceived: func(ctx context.Context, payload []byte) {
			t.Log("Client received pong")
			pongReceived <- struct{}{}
		},
	}

	// Create test client
	client, resp, err := websocket.Dial(context.Background(), wsURL, opts)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("Expected status code %d, got %d", http.StatusSwitchingProtocols, resp.StatusCode)
	}
	defer client.CloseNow()

	// Wait for client registration
	time.Sleep(cfg.setupDelay)

	// Verify initial client connection
	verifyClientCount(t, handler, 1, "Initial connection")
	t.Log("Client successfully registered")

	// Start a goroutine to read messages
	ctx, cancel := context.WithTimeout(context.Background(), cfg.testDuration)
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, _, err := client.Read(ctx)
				if err != nil {
					if !errors.Is(err, context.DeadlineExceeded) {
						t.Logf("Read error: %v", err)
					}
					return
				}
			}
		}
	}()

	// Send a ping from client to server
	err = client.Ping(ctx)
	if err != nil {
		t.Fatalf("Failed to send ping: %v", err)
	}
	t.Log("Client sent ping")

	// Wait for pong response
	select {
	case <-time.After(cfg.pongTimeout):
		t.Error("Timeout waiting for pong response")
	case <-pongReceived:
		t.Log("Client received pong response")
	}

	// Verify client is still connected
	verifyClientCount(t, handler, 1, "After ping/pong cycle")
	t.Log("Ping/pong mechanism test completed successfully")
}

// TestPingTimeout tests what happens when a client doesn't respond to pings
func TestPingTimeout(t *testing.T) {
	handler, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	cfg := defaultTestConfig()

	// Create test client
	client, resp, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("Expected status code %d, got %d", http.StatusSwitchingProtocols, resp.StatusCode)
	}

	// Wait for client registration
	time.Sleep(cfg.setupDelay)

	// Verify initial client connection
	verifyClientCount(t, handler, 1, "Initial connection")

	// Simulate an unresponsive client by closing the connection abruptly
	client.CloseNow()

	// Wait for the server to detect the dead connection
	// The server needs some time to detect and clean up the dead connection
	success := false
	deadline := time.Now().Add(cfg.testDuration)
	for time.Now().Before(deadline) {
		if len(handler.hub.clients) == 0 {
			success = true
			t.Log("Dead client successfully detected and removed")
			break
		}
		time.Sleep(cfg.monitorInterval)
	}

	if !success {
		t.Error("Server failed to detect and remove dead client")
	}
}

// TestMultipleClientsPingPong tests ping/pong with multiple clients
func TestMultipleClientsPingPong(t *testing.T) {
	handler, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	cfg := defaultTestConfig()

	numClients := 3
	var clients []*websocket.Conn

	// Connect multiple clients
	for i := 0; i < numClients; i++ {
		client, resp, err := websocket.Dial(context.Background(), wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to create client %d: %v", i, err)
		}
		if resp.StatusCode != http.StatusSwitchingProtocols {
			t.Fatalf("Client %d: expected status code %d, got %d", i, http.StatusSwitchingProtocols, resp.StatusCode)
		}
		clients = append(clients, client)
		time.Sleep(cfg.setupDelay)
	}

	// Clean up connections
	defer func() {
		for _, client := range clients {
			client.CloseNow()
		}
	}()

	// Verify all clients are registered
	verifyClientCount(t, handler, numClients, "Multiple clients connected")

	// Wait for ping/pong cycles
	time.Sleep(cfg.testDuration)

	// Verify all clients are still connected
	verifyClientCount(t, handler, numClients, "After ping/pong cycles")
	t.Logf("Multiple clients ping/pong test completed successfully with %d clients", numClients)
}
