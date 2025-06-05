package ws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/ethereum-optimism/optimism/op-conductor/metrics"
	"github.com/ethereum/go-ethereum/log"
)

// testHub extends Hub with event notifications for testing
type testHub struct {
	*Hub
	clientRegistered   chan *Client
	clientUnregistered chan *Client
	shutdownComplete   chan struct{}
}

func newTestHub() *testHub {
	return &testHub{
		Hub:                newHub(),
		clientRegistered:   make(chan *Client, 10),
		clientUnregistered: make(chan *Client, 10),
		shutdownComplete:   make(chan struct{}),
	}
}

func (th *testHub) run() {
	defer close(th.shutdownComplete) // Signal when run() exits

	for {
		select {
		case <-th.done:
			// Close all remaining client connections
			for client := range th.clients {
				client.Close()
				delete(th.clients, client)
			}
			return
		case client := <-th.register:
			th.clients[client] = true
			clientCount := len(th.clients)
			th.log.Info("Client registered with hub", "totalClients", clientCount)
			// Notify test of registration
			select {
			case th.clientRegistered <- client:
			default:
			}
		case client := <-th.unregister:
			if _, ok := th.clients[client]; ok {
				delete(th.clients, client)
				client.Close()
				// Notify test of unregistration
				select {
				case th.clientUnregistered <- client:
				default:
				}
			}
		case message := <-th.broadcast:
			successCount := 0
			dropCount := 0

			for client := range th.clients {
				select {
				case client.send <- message:
					successCount++
				default:
					th.log.Debug("Failed to send message to client, channel full")
					dropCount++
				}
			}
			if dropCount > 0 {
				th.log.Warn("Failed to send message to all clients, dropped", "successCount", successCount, "dropCount", dropCount)
			}
		}
	}
}

// testClient wraps Client with additional test functionality
type testClient struct {
	*Client
	messagesReceived chan []byte
	pingsReceived    chan struct{}
	pongsReceived    chan struct{}
	conn             *websocket.Conn
}

func newTestClient(ctx context.Context, wsURL string) (*testClient, error) {
	tc := &testClient{
		messagesReceived: make(chan []byte, 100),
		pingsReceived:    make(chan struct{}, 10),
		pongsReceived:    make(chan struct{}, 10),
	}

	opts := &websocket.DialOptions{
		OnPingReceived: func(ctx context.Context, payload []byte) bool {
			select {
			case tc.pingsReceived <- struct{}{}:
			default:
			}
			return true // Send pong response
		},
		OnPongReceived: func(ctx context.Context, payload []byte) {
			select {
			case tc.pongsReceived <- struct{}{}:
			default:
			}
		},
	}

	conn, resp, err := websocket.Dial(ctx, wsURL, opts)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		conn.CloseNow()
		return nil, errors.New("unexpected status code")
	}

	tc.conn = conn

	// Start message reader
	go tc.readMessages(ctx)

	return tc, nil
}

func (tc *testClient) readMessages(ctx context.Context) {
	defer tc.conn.CloseNow()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := tc.conn.Read(ctx)
			if err != nil {
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					// Connection closed or other error
				}
				return
			}
			select {
			case tc.messagesReceived <- message:
			default:
				// Buffer full, drop message
			}
		}
	}
}

func (tc *testClient) Close() error {
	return tc.conn.Close(websocket.StatusNormalClosure, "test complete")
}

func (tc *testClient) Ping(ctx context.Context) error {
	return tc.conn.Ping(ctx)
}

func (tc *testClient) Write(ctx context.Context, data []byte) error {
	return tc.conn.Write(ctx, websocket.MessageText, data)
}

// setupTestServer creates a test WebSocket server with event notifications
func setupTestServer(t *testing.T) (*Handler, *testHub, *httptest.Server, func()) {
	t.Helper()

	cfg := Config{
		WebsocketServerPort: 8080,
		RollupBoostWsURL:    "ws://mock-url",
	}

	logger := log.New("test", "websocket")
	isLeaderFn := func(ctx context.Context) bool { return true }

	handler := &Handler{
		cfg:        cfg,
		log:        logger,
		isLeaderFn: isLeaderFn,
		metrics:    &metrics.NoopMetricsImpl{},
	}

	// Create test hub with event notifications
	testHub := newTestHub()
	handler.hub = testHub.Hub
	go testHub.run()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handler.handleWebSocket)
	server := httptest.NewServer(mux)

	cleanup := func() {
		select {
		case <-testHub.done:
		default:
			close(testHub.done)
		}
		server.Close()
	}

	return handler, testHub, server, cleanup
}

// waitForClientCount waits for the expected number of clients with timeout
func waitForClientCount(t *testing.T, hub *testHub, expected int, timeout time.Duration, msg string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	currentCount := len(hub.clients)
	if currentCount == expected {
		return // Already at expected count
	}

	for currentCount != expected {
		select {
		case <-ctx.Done():
			t.Fatalf("%s: timeout waiting for %d clients, got %d", msg, expected, currentCount)
		case <-hub.clientRegistered:
			currentCount = len(hub.clients)
		case <-hub.clientUnregistered:
			currentCount = len(hub.clients)
		}
	}
}

// TestPingPongMechanism tests the actual ping/pong keepalive mechanism
func TestPingPongMechanism(t *testing.T) {
	_, testHub, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create test client
	client, err := newTestClient(ctx, wsURL)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}
	defer client.Close()

	// Wait for client registration
	waitForClientCount(t, testHub, 1, 2*time.Second, "Initial connection")

	// Send ping from client to server
	err = client.Ping(ctx)
	if err != nil {
		t.Fatalf("Failed to send ping: %v", err)
	}

	// Wait for pong response
	select {
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for pong response")
	case <-client.pongsReceived:
		t.Log("Client received pong response")
	}

	// Verify client is still connected
	if len(testHub.clients) != 1 {
		t.Errorf("Expected 1 client after ping/pong, got %d", len(testHub.clients))
	}
}

// TestServerInitiatedPing tests that the server sends pings to clients
func TestServerInitiatedPing(t *testing.T) {
	_, testHub, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Create test client
	client, err := newTestClient(ctx, wsURL)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}
	defer client.Close()

	// Wait for client registration
	waitForClientCount(t, testHub, 1, 2*time.Second, "Initial connection")

	// Wait for server to send a ping (server sends pings every 15 seconds in real code)
	// For testing, we might need to adjust the ping interval or trigger it manually
	select {
	case <-time.After(17 * time.Second): // Wait a bit longer than ping interval
		t.Error("Timeout waiting for server ping")
	case <-client.pingsReceived:
		t.Log("Client received ping from server")
	}
}

// TestClientTimeout tests what happens when a client doesn't respond
func TestClientTimeout(t *testing.T) {
	_, testHub, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Create client connection but don't process messages
	conn, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("Expected status code %d, got %d", http.StatusSwitchingProtocols, resp.StatusCode)
	}

	// Wait for client registration
	waitForClientCount(t, testHub, 1, 2*time.Second, "Initial connection")

	// Abruptly close the connection to simulate unresponsive client
	conn.CloseNow()

	// Wait for client unregistration
	waitForClientCount(t, testHub, 0, 5*time.Second, "Client cleanup after timeout")
}

// TestMultipleClientsBroadcast tests broadcast functionality with multiple clients
func TestMultipleClientsBroadcast(t *testing.T) {
	handler, testHub, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	numClients := 3
	var clients []*testClient

	// Connect multiple clients
	for i := 0; i < numClients; i++ {
		client, err := newTestClient(ctx, wsURL)
		if err != nil {
			t.Fatalf("Failed to create client %d: %v", i, err)
		}
		clients = append(clients, client)
	}

	// Clean up connections
	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	// Wait for all clients to be registered
	waitForClientCount(t, testHub, numClients, 5*time.Second, "All clients connected")

	// Send broadcast messages
	testMessages := []string{
		"Hello World!",
		"Broadcast Test Message 1",
		"Another test message",
	}

	for _, msg := range testMessages {
		handler.BroadcastMessage([]byte(msg))
	}

	// Verify all clients received all messages
	for i, client := range clients {
		receivedMessages := make([]string, 0)

		// Collect messages with timeout
		msgTimeout := time.After(2 * time.Second)
		for len(receivedMessages) < len(testMessages) {
			select {
			case msg := <-client.messagesReceived:
				receivedMessages = append(receivedMessages, string(msg))
			case <-msgTimeout:
				t.Errorf("Client %d: timeout waiting for messages, got %d/%d", i, len(receivedMessages), len(testMessages))
				goto nextClient
			}
		}

		// Verify message content
		for j, expected := range testMessages {
			if j >= len(receivedMessages) || receivedMessages[j] != expected {
				t.Errorf("Client %d: message %d mismatch, expected %q, got %q", i, j, expected, receivedMessages[j])
			}
		}

	nextClient:
	}
}

// TestConcurrentConnections tests concurrent client connections and disconnections
func TestConcurrentConnections(t *testing.T) {
	_, testHub, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	numClients := 10
	var wg sync.WaitGroup

	// Connect clients concurrently
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientIdx int) {
			defer wg.Done()

			client, err := newTestClient(ctx, wsURL)
			if err != nil {
				t.Errorf("Client %d connection failed: %v", clientIdx, err)
				return
			}
			defer client.Close()

			// Send a test message
			err = client.Write(ctx, []byte("test message"))
			if err != nil {
				t.Errorf("Client %d write failed: %v", clientIdx, err)
			}

			// Keep connection alive for a bit
			time.Sleep(100 * time.Millisecond)
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Wait for all clients to disconnect
	waitForClientCount(t, testHub, 0, 5*time.Second, "All clients disconnected")
}

// TestBroadcastWithSlowClient tests broadcast behavior when one client is slow
func TestBroadcastWithSlowClient(t *testing.T) {
	handler, testHub, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create fast client
	fastClient, err := newTestClient(ctx, wsURL)
	if err != nil {
		t.Fatalf("Failed to create fast client: %v", err)
	}
	defer fastClient.Close()

	// Create slow client (don't read messages)
	slowConn, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to create slow client: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("Expected status code %d, got %d", http.StatusSwitchingProtocols, resp.StatusCode)
	}
	defer slowConn.CloseNow()

	// Wait for both clients to be registered
	waitForClientCount(t, testHub, 2, 3*time.Second, "Both clients connected")

	// Send many messages to fill up the slow client's buffer
	for i := 0; i < 300; i++ {
		message := []byte("Large message to fill buffer " + string(rune('0'+i%10)))
		handler.BroadcastMessage(message)
	}

	// Fast client should still receive some messages
	select {
	case <-time.After(2 * time.Second):
		t.Error("Fast client didn't receive any messages")
	case <-fastClient.messagesReceived:
		t.Log("Fast client received messages despite slow client")
	}

	// Both clients should still be connected initially
	if len(testHub.clients) != 2 {
		t.Logf("Expected 2 clients, got %d (slow client may have been cleaned up)", len(testHub.clients))
	}
}

// TestHubShutdown tests graceful hub shutdown
func TestHubShutdown(t *testing.T) {
	_, testHub, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect a client
	client, err := newTestClient(ctx, wsURL)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}
	defer client.Close()

	// Wait for client registration
	waitForClientCount(t, testHub, 1, 2*time.Second, "Client connected before shutdown")

	// Trigger shutdown
	close(testHub.done)

	// Wait for hub.run() to complete shutdown
	select {
	case <-testHub.shutdownComplete:
		// Hub has completed shutdown
	case <-time.After(2 * time.Second):
		t.Fatal("Hub shutdown timed out")
	}

	// Verify clients were cleaned up
	if len(testHub.clients) != 0 {
		t.Errorf("Expected 0 clients after shutdown, got %d", len(testHub.clients))
	}

	t.Log("Hub shutdown completed successfully")
}
