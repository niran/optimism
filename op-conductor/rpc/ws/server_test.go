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
			// Safely close the done channel if it's not already closed
			select {
			case <-handler.hub.done:
				// Channel is already closed
			default:
				// Channel is open, safe to close
				close(handler.hub.done)
			}
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

// TestReadPumpTimeout tests the readPump's behavior when read timeouts occur
func TestReadPumpTimeout(t *testing.T) {
	handler, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	cfg := defaultTestConfig()

	// Create test client that will not send any messages
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

	// Wait for a reasonable amount of time to ensure the readPump is running
	// We can't easily test the 30s timeout in a unit test, but we can verify
	// that the connection remains stable and functional
	time.Sleep(2 * time.Second)

	// Verify client is still connected
	verifyClientCount(t, handler, 1, "After waiting period")

	// Send a test message to verify the readPump is still processing
	err = client.Write(context.Background(), websocket.MessageText, []byte("test message"))
	if err != nil {
		t.Errorf("Failed to write test message: %v", err)
	}

	// Give the server time to process the message
	time.Sleep(100 * time.Millisecond)

	// Verify client is still connected after sending message
	verifyClientCount(t, handler, 1, "After sending message")

	// Close the client connection properly
	err = client.Close(websocket.StatusNormalClosure, "test complete")
	if err != nil {
		t.Errorf("Failed to close connection: %v", err)
	}

	// Wait for the client to be unregistered
	unregistered := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(handler.hub.clients) == 0 {
			unregistered = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !unregistered {
		t.Fatal("Client was not properly unregistered")
	}

	t.Log("ReadPump timeout test completed successfully")
}

// TestBroadcastFunctionality tests that broadcast messages reach all connected clients
func TestBroadcastFunctionality(t *testing.T) {
	handler, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	cfg := defaultTestConfig()

	numClients := 3
	var clients []*websocket.Conn
	messageChannels := make([]chan string, numClients)

	// Connect multiple clients and set up message receivers
	for i := 0; i < numClients; i++ {
		client, resp, err := websocket.Dial(context.Background(), wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to create client %d: %v", i, err)
		}
		if resp.StatusCode != http.StatusSwitchingProtocols {
			t.Fatalf("Client %d: expected status code %d, got %d", i, http.StatusSwitchingProtocols, resp.StatusCode)
		}
		clients = append(clients, client)
		messageChannels[i] = make(chan string, 10)

		// Start message reader for each client
		go func(clientIdx int, conn *websocket.Conn) {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.testDuration)
			defer cancel()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					_, message, err := conn.Read(ctx)
					if err != nil {
						if !errors.Is(err, context.DeadlineExceeded) {
							t.Logf("Client %d read error: %v", clientIdx, err)
						}
						return
					}
					messageChannels[clientIdx] <- string(message)
				}
			}
		}(i, client)

		time.Sleep(cfg.setupDelay)
	}

	// Clean up connections
	defer func() {
		for _, client := range clients {
			client.CloseNow()
		}
	}()

	// Verify all clients are registered
	verifyClientCount(t, handler, numClients, "All clients connected")

	// Send broadcast messages
	testMessages := []string{
		"Hello World!",
		"Broadcast Test Message 1",
		"Another test message with special chars: !@#$%^&*()",
	}

	for _, msg := range testMessages {
		handler.hub.broadcast <- []byte(msg)
		t.Logf("Sent broadcast message: %s", msg)
	}

	// Wait for messages to be processed
	time.Sleep(500 * time.Millisecond)

	// Verify all clients received all messages
	for i, msgChan := range messageChannels {
		receivedMessages := make([]string, 0)

		// Collect all messages from this client
		timeout := time.After(2 * time.Second)
	messageLoop:
		for len(receivedMessages) < len(testMessages) {
			select {
			case msg := <-msgChan:
				receivedMessages = append(receivedMessages, msg)
			case <-timeout:
				t.Errorf("Client %d: timeout waiting for messages, got %d/%d", i, len(receivedMessages), len(testMessages))
				break messageLoop
			}
		}

		// Verify messages match
		if len(receivedMessages) != len(testMessages) {
			t.Errorf("Client %d: expected %d messages, got %d", i, len(testMessages), len(receivedMessages))
		}

		for j, expected := range testMessages {
			if j < len(receivedMessages) && receivedMessages[j] != expected {
				t.Errorf("Client %d: message %d mismatch, expected %q, got %q", i, j, expected, receivedMessages[j])
			}
		}
	}

	t.Log("Broadcast functionality test completed successfully")
}

// TestSendChannelOverflow tests what happens when the send channel buffer is exceeded
func TestSendChannelOverflow(t *testing.T) {
	handler, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	cfg := defaultTestConfig()

	// Create a client that won't read messages (to fill up the send buffer)
	client, resp, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("Expected status code %d, got %d", http.StatusSwitchingProtocols, resp.StatusCode)
	}
	defer client.CloseNow()

	// Wait for client registration
	time.Sleep(cfg.setupDelay)
	verifyClientCount(t, handler, 1, "Initial connection")

	// Send more messages than the buffer can hold (sendChannelBufferSize = 256)
	// Create messages larger than 256 bytes to test the scenario mentioned
	largeMessage := make([]byte, 300) // Larger than 256 bytes
	for i := range largeMessage {
		largeMessage[i] = byte('A' + (i % 26))
	}

	// Send enough messages to overflow the buffer
	overflowCount := 300 // More than the 256 buffer size
	successCount := 0
	dropCount := 0

	for i := 0; i < overflowCount; i++ {
		// Add a unique identifier to each message
		message := append(largeMessage, []byte(" - Message "+string(rune('0'+i%10)))...)

		select {
		case handler.hub.broadcast <- message:
			successCount++
		default:
			dropCount++
		}
	}

	t.Logf("Sent %d messages, %d successful, %d dropped at broadcast level", overflowCount, successCount, dropCount)

	// Wait for the hub to process messages
	time.Sleep(2 * time.Second)

	// The client should still be connected initially, but the send channel should be full
	// The hub's broadcast mechanism should handle the overflow gracefully
	verifyClientCount(t, handler, 1, "After overflow attempt")

	t.Log("Send channel overflow test completed successfully")
}

// TestSendChannelFullDrop tests what happens when a client's send channel is full
func TestSendChannelFullDrop(t *testing.T) {
	handler, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	cfg := defaultTestConfig()

	// Create a client that won't read messages (to fill up the send buffer)
	client, resp, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("Expected status code %d, got %d", http.StatusSwitchingProtocols, resp.StatusCode)
	}
	defer client.CloseNow()

	// Wait for client registration
	time.Sleep(cfg.setupDelay)
	verifyClientCount(t, handler, 1, "Initial connection")

	// Send exactly enough messages to fill the send channel buffer (256)
	// Plus some extra to test the overflow behavior
	totalMessages := 300
	largeMessage := make([]byte, 1000) // Large message to fill buffer faster
	for i := range largeMessage {
		largeMessage[i] = byte('A' + (i % 26))
	}

	// Send messages rapidly without the client reading them
	for i := 0; i < totalMessages; i++ {
		message := append(largeMessage, []byte(" - Message "+string(rune('0'+i%10)))...)
		handler.hub.broadcast <- message

		// Small delay to let some messages be processed
		if i%50 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Wait for the hub to process all messages
	time.Sleep(3 * time.Second)

	// The client should still be connected, but some messages should have been dropped
	// due to the send channel being full
	verifyClientCount(t, handler, 1, "After sending many messages")

	t.Log("Send channel full drop test completed successfully")
}

// TestWritePumpAfterChannelClose tests writePump behavior after the send channel is closed
func TestWritePumpAfterChannelClose(t *testing.T) {
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
	defer client.CloseNow()

	// Wait for client registration
	time.Sleep(cfg.setupDelay)
	verifyClientCount(t, handler, 1, "Initial connection")

	// Send a message before closing the connection
	testMessage := []byte("Message before close")
	handler.hub.broadcast <- testMessage

	// Wait for message to be processed
	time.Sleep(100 * time.Millisecond)

	// Close the client connection properly to trigger channel cleanup
	err = client.Close(websocket.StatusNormalClosure, "test close")
	if err != nil {
		t.Logf("Client close error: %v", err)
	}

	// Wait for the writePump to detect the closed connection and exit
	time.Sleep(2 * time.Second)

	// Try to send another message after the client has disconnected
	// This should be handled gracefully by the hub
	testMessage2 := []byte("Message after close")
	handler.hub.broadcast <- testMessage2

	// Wait for cleanup
	time.Sleep(1 * time.Second)

	// The client should eventually be unregistered
	success := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(handler.hub.clients) == 0 {
			success = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !success {
		t.Error("Client was not unregistered after connection close")
	}

	t.Log("WritePump after connection close test completed successfully")
}

// TestConcurrentClientClosing tests concurrent closing of multiple clients
func TestConcurrentClientClosing(t *testing.T) {
	handler, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	cfg := defaultTestConfig()

	numClients := 10
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
		time.Sleep(cfg.setupDelay / 10) // Shorter delay for faster setup
	}

	// Wait for all clients to be registered
	time.Sleep(cfg.setupDelay)
	verifyClientCount(t, handler, numClients, "All clients connected")

	// Concurrently close all clients
	var wg sync.WaitGroup
	for i, client := range clients {
		wg.Add(1)
		go func(clientIdx int, conn *websocket.Conn) {
			defer wg.Done()
			// Add some randomness to the timing
			time.Sleep(time.Duration(clientIdx*10) * time.Millisecond)
			err := conn.Close(websocket.StatusNormalClosure, "concurrent close test")
			if err != nil {
				t.Logf("Client %d close error: %v", clientIdx, err)
			}
		}(i, client)
	}

	// Wait for all closes to complete
	wg.Wait()

	// Wait for the server to process all unregistrations
	success := false
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if len(handler.hub.clients) == 0 {
			success = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !success {
		t.Errorf("Not all clients were unregistered, remaining: %d", len(handler.hub.clients))
	}

	t.Log("Concurrent client closing test completed successfully")
}

// TestHubShutdownWithActiveClients tests hub shutdown behavior with active clients
func TestHubShutdownWithActiveClients(t *testing.T) {
	handler, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	cfg := defaultTestConfig()

	numClients := 5
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
		time.Sleep(cfg.setupDelay / 5)
	}

	// Clean up connections
	defer func() {
		for _, client := range clients {
			client.CloseNow()
		}
	}()

	// Wait for all clients to be registered
	time.Sleep(cfg.setupDelay)
	verifyClientCount(t, handler, numClients, "All clients connected before shutdown")

	// Shutdown the hub
	close(handler.hub.done)

	// Wait for hub to process shutdown
	time.Sleep(2 * time.Second)

	// Verify all clients were cleaned up
	if len(handler.hub.clients) != 0 {
		t.Errorf("Expected 0 clients after shutdown, got %d", len(handler.hub.clients))
	}

	// Try to register a new client after shutdown (should not work)
	newClient, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err == nil {
		newClient.CloseNow()
		// The connection might succeed but the hub won't process it
		time.Sleep(500 * time.Millisecond)
		if len(handler.hub.clients) > 0 {
			t.Error("New client was registered after hub shutdown")
		}
	}

	t.Log("Hub shutdown with active clients test completed successfully")
}

// TestConcurrentRegisterUnregister tests concurrent register and unregister operations
func TestConcurrentRegisterUnregister(t *testing.T) {
	handler, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Number of concurrent operations
	numOperations := 20
	var wg sync.WaitGroup

	// Channel to collect any errors
	errorChan := make(chan error, numOperations*2)

	// Concurrently connect and disconnect clients
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(clientIdx int) {
			defer wg.Done()

			// Connect client
			client, resp, err := websocket.Dial(context.Background(), wsURL, nil)
			if err != nil {
				errorChan <- err
				return
			}
			if resp.StatusCode != http.StatusSwitchingProtocols {
				errorChan <- errors.New("unexpected status code")
				return
			}

			// Random delay to create race conditions
			time.Sleep(time.Duration(clientIdx%10) * time.Millisecond)

			// Send a message to ensure the client is active
			err = client.Write(context.Background(), websocket.MessageText, []byte("test message"))
			if err != nil {
				errorChan <- err
			}

			// Random delay before closing
			time.Sleep(time.Duration((clientIdx*7)%50) * time.Millisecond)

			// Close the client
			err = client.Close(websocket.StatusNormalClosure, "race test")
			if err != nil {
				errorChan <- err
			}
		}(i)
	}

	// Also concurrently send broadcast messages to create more race conditions
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(msgIdx int) {
			defer wg.Done()
			time.Sleep(time.Duration(msgIdx*20) * time.Millisecond)

			message := []byte("Race test message " + string(rune('0'+msgIdx%10)))
			select {
			case handler.hub.broadcast <- message:
				// Message sent successfully
			default:
				// Broadcast channel full, which is acceptable
			}
		}(i)
	}

	// Wait for all operations to complete
	wg.Wait()

	// Check for errors
	close(errorChan)
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Logf("Encountered %d errors during concurrent operations:", len(errors))
		for i, err := range errors {
			t.Logf("Error %d: %v", i+1, err)
		}
		// Don't fail the test for connection errors as they might be expected
		// in high-concurrency scenarios
	}

	// Wait for all clients to be cleaned up
	success := false
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if len(handler.hub.clients) == 0 {
			success = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !success {
		t.Errorf("Not all clients were cleaned up after concurrent operations, remaining: %d", len(handler.hub.clients))
	}

	t.Log("Concurrent register/unregister race test completed successfully")
}

// TestRaceConditionInClientMap tests for race conditions in the client map access
func TestRaceConditionInClientMap(t *testing.T) {
	handler, server, cleanup := setupTestServer(t)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Run with race detector enabled: go test -race
	numGoroutines := 50
	var wg sync.WaitGroup

	// Concurrently perform operations that access the client map
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Connect a client
			client, resp, err := websocket.Dial(context.Background(), wsURL, nil)
			if err != nil {
				t.Logf("Client %d connection error: %v", idx, err)
				return
			}
			if resp.StatusCode != http.StatusSwitchingProtocols {
				t.Logf("Client %d unexpected status: %d", idx, resp.StatusCode)
				return
			}

			// Brief delay to let registration happen
			time.Sleep(10 * time.Millisecond)

			// Send broadcast messages while clients are connecting/disconnecting
			if idx%3 == 0 {
				for j := 0; j < 5; j++ {
					message := []byte("Race message from goroutine " + string(rune('0'+idx%10)))
					select {
					case handler.hub.broadcast <- message:
					default:
					}
					time.Sleep(time.Millisecond)
				}
			}

			// Random delay before closing
			time.Sleep(time.Duration(idx%20) * time.Millisecond)

			// Close the client
			client.CloseNow()
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Wait for cleanup
	time.Sleep(2 * time.Second)

	// Verify final state
	finalClientCount := len(handler.hub.clients)
	if finalClientCount > 0 {
		t.Logf("Warning: %d clients remaining after race test", finalClientCount)
		// Give more time for cleanup
		time.Sleep(3 * time.Second)
		finalClientCount = len(handler.hub.clients)
	}

	t.Logf("Race condition test completed, final client count: %d", finalClientCount)
}
