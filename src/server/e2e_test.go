package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// --- Task 14.1: E2E tests for POST → GET → WS flow ---
// Validates: Requirements 13.1, 13.2

// setupE2EServer creates a test server with both API and WebSocket routes,
// mirroring the real server's mux layout. Returns the server and a cleanup func.
func setupE2EServer() (*httptest.Server, func()) {
	origStore := store
	origHub := hub
	origDetector := detector
	origDispatcher := dispatcher
	origChainBuilder := chainBuilder
	origPredEngine := predEngine
	origReplayStore := replayStore
	origTopoMap := topoMap

	store = NewMemoryStore()
	hub = NewHub()
	go hub.Run()
	detector = NewAnomalyDetector(store, 10, 40)
	dispatcher = NewAlertDispatcher("", hub.BroadcastAlert)
	chainBuilder = NewCausalChainBuilder(store, hub)
	predEngine = NewPredictionEngine(store, hub)
	replayStore = NewReplayStore(store, 24*time.Hour)
	topoMap = NewNetworkTopologyMap(5*time.Minute, hub)

	topMux := http.NewServeMux()
	topMux.HandleFunc("/ws/heartbeats", func(w http.ResponseWriter, r *http.Request) {
		ServeWS(hub, w, r)
	})

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/heartbeat", heartbeatHandler)
	apiMux.HandleFunc("/api/heartbeats", getHeartbeatsHandler)
	topMux.Handle("/api/", LoggingMiddleware(setCORS(apiMux, []string{"*"})))

	ts := httptest.NewServer(topMux)

	cleanup := func() {
		ts.Close()
		store = origStore
		hub = origHub
		detector = origDetector
		dispatcher = origDispatcher
		chainBuilder = origChainBuilder
		predEngine = origPredEngine
		replayStore = origReplayStore
		topoMap = origTopoMap
	}
	return ts, cleanup
}

// TestE2E_PostThenGet verifies the full POST → GET round-trip via HTTP.
// Validates: Property 15 (Heartbeat POST-then-GET round-trip)
func TestE2E_PostThenGet(t *testing.T) {
	ts, cleanup := setupE2EServer()
	defer cleanup()

	now := time.Now().UTC().Truncate(time.Second)
	posted := Heartbeat{
		NodeName:  "e2e-node-01",
		Namespace: "kube-system",
		Timestamp: now,
		Status:    "Ready",
	}
	body, err := json.Marshal(posted)
	if err != nil {
		t.Fatalf("failed to marshal heartbeat: %v", err)
	}

	// POST the heartbeat
	resp, err := http.Post(ts.URL+"/api/heartbeat", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/heartbeat failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected POST status 201, got %d", resp.StatusCode)
	}

	// GET heartbeats and verify the posted event is present
	resp, err = http.Get(ts.URL + "/api/heartbeats")
	if err != nil {
		t.Fatalf("GET /api/heartbeats failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected GET status 200, got %d", resp.StatusCode)
	}

	var heartbeats []Heartbeat
	if err := json.NewDecoder(resp.Body).Decode(&heartbeats); err != nil {
		t.Fatalf("failed to decode GET response: %v", err)
	}

	found := false
	for _, hb := range heartbeats {
		if hb.NodeName == posted.NodeName &&
			hb.Namespace == posted.Namespace &&
			hb.Timestamp.Equal(posted.Timestamp) &&
			hb.Status == posted.Status {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("posted heartbeat not found in GET response; got %+v", heartbeats)
	}
}

// TestE2E_PostThenWSReceive verifies that a heartbeat POSTed via HTTP is
// broadcast to a connected WebSocket client on /ws/heartbeats.
// Validates: Property 5 (WebSocket broadcast of heartbeat events)
func TestE2E_PostThenWSReceive(t *testing.T) {
	ts, cleanup := setupE2EServer()
	defer cleanup()

	// Connect a WebSocket client to /ws/heartbeats
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/heartbeats"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer wsConn.Close()

	// Give the hub a moment to register the client
	time.Sleep(50 * time.Millisecond)

	// POST a heartbeat
	now := time.Now().UTC().Truncate(time.Second)
	posted := Heartbeat{
		NodeName:  "e2e-ws-node-01",
		Namespace: "default",
		Timestamp: now,
		Status:    "Ready",
	}
	body, err := json.Marshal(posted)
	if err != nil {
		t.Fatalf("failed to marshal heartbeat: %v", err)
	}

	resp, err := http.Post(ts.URL+"/api/heartbeat", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/heartbeat failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected POST status 201, got %d", resp.StatusCode)
	}

	// Read from the WebSocket with a timeout
	wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msgBytes, err := wsConn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read WS message: %v", err)
	}

	// Parse the WSMessage envelope
	var wsMsg WSMessage
	if err := json.Unmarshal(msgBytes, &wsMsg); err != nil {
		t.Fatalf("failed to unmarshal WS message: %v", err)
	}

	if wsMsg.Type != "heartbeat" {
		t.Fatalf("expected WS message type 'heartbeat', got %q", wsMsg.Type)
	}

	// Re-marshal the payload to decode it as a Heartbeat
	payloadBytes, err := json.Marshal(wsMsg.Payload)
	if err != nil {
		t.Fatalf("failed to re-marshal WS payload: %v", err)
	}
	var received Heartbeat
	if err := json.Unmarshal(payloadBytes, &received); err != nil {
		t.Fatalf("failed to unmarshal WS payload as Heartbeat: %v", err)
	}

	if received.NodeName != posted.NodeName {
		t.Fatalf("WS heartbeat NodeName mismatch: expected %q, got %q", posted.NodeName, received.NodeName)
	}
	if received.Namespace != posted.Namespace {
		t.Fatalf("WS heartbeat Namespace mismatch: expected %q, got %q", posted.Namespace, received.Namespace)
	}
	if received.Status != posted.Status {
		t.Fatalf("WS heartbeat Status mismatch: expected %q, got %q", posted.Status, received.Status)
	}
	if !received.Timestamp.Equal(posted.Timestamp) {
		t.Fatalf("WS heartbeat Timestamp mismatch: expected %v, got %v", posted.Timestamp, received.Timestamp)
	}
}
