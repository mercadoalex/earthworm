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
	"pgregory.net/rapid"
)

// Feature: ebpf-kernel-observability, Property 13: WSMessage envelope compliance
// — broadcast messages have correct type and payload structure
// **Validates: Requirements 11.1**
func TestProperty13_WSMessageEnvelopeCompliance(t *testing.T) {
	testHub := NewHub()
	go testHub.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/heartbeats", func(w http.ResponseWriter, r *http.Request) {
		ServeWS(testHub, w, r)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	rapid.Check(t, func(t *rapid.T) {
		// Connect WS client
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/heartbeats"
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("WS dial failed: %v", err)
		}
		defer ws.Close()

		time.Sleep(50 * time.Millisecond)

		// Choose a message type to broadcast
		msgType := rapid.SampledFrom([]string{"ebpf_event", "causal_chain", "prediction"}).Draw(t, "msgType")

		switch msgType {
		case "ebpf_event":
			event := EnrichedEvent{
				Timestamp: time.Now().UTC(),
				PID:       rapid.Uint32Range(1, 65535).Draw(t, "pid"),
				Comm:      rapid.SampledFrom([]string{"kubelet", "containerd"}).Draw(t, "comm"),
				EventType: rapid.SampledFrom([]string{"syscall", "process", "network"}).Draw(t, "eventType"),
				NodeName:  rapid.StringMatching(`node-[a-z0-9]{3,6}`).Draw(t, "nodeName"),
			}
			testHub.BroadcastEbpfEvent(event)

		case "causal_chain":
			chain := CausalChain{
				NodeName:  rapid.StringMatching(`node-[a-z0-9]{3,6}`).Draw(t, "nodeName"),
				Timestamp: time.Now().UTC(),
				Summary:   "test causal chain summary",
				RootCause: "unknown_cause",
				Events:    []EnrichedEvent{},
			}
			testHub.BroadcastCausalChain(chain)

		case "prediction":
			pred := Prediction{
				NodeName:      rapid.StringMatching(`node-[a-z0-9]{3,6}`).Draw(t, "nodeName"),
				Confidence:    rapid.Float64Range(0.0, 1.0).Draw(t, "confidence"),
				TimeToFailure: rapid.Float64Range(1.0, 60.0).Draw(t, "ttf"),
				Timestamp:     time.Now().UTC(),
				Patterns:      []string{"test_pattern"},
				Outcome:       "pending",
			}
			testHub.BroadcastPrediction(pred)
		}

		// Read the WS message
		ws.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, msgBytes, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("WS read failed: %v", err)
		}

		// Verify WSMessage envelope structure
		var wsMsg WSMessage
		if err := json.Unmarshal(msgBytes, &wsMsg); err != nil {
			t.Fatalf("WS unmarshal failed: %v", err)
		}

		// Type must match what we broadcast
		if wsMsg.Type != msgType {
			t.Fatalf("WS message type: got %q, want %q", wsMsg.Type, msgType)
		}

		// Payload must not be nil
		if wsMsg.Payload == nil {
			t.Fatal("WS message payload is nil")
		}

		// Verify the raw JSON has both "type" and "payload" fields
		var rawMsg map[string]json.RawMessage
		if err := json.Unmarshal(msgBytes, &rawMsg); err != nil {
			t.Fatalf("raw unmarshal failed: %v", err)
		}
		if _, ok := rawMsg["type"]; !ok {
			t.Fatal("WS message missing 'type' field")
		}
		if _, ok := rawMsg["payload"]; !ok {
			t.Fatal("WS message missing 'payload' field")
		}
	})
}

// --- Task 5.3: Unit test verifying WebSocket clients receive ebpf_event messages ---
// Validates: Requirements 9.3

func TestUnit_WebSocketClient_ReceivesEbpfEventMessages(t *testing.T) {
	// Set up isolated server infrastructure
	testStore := NewMemoryStore()
	testHub := NewHub()
	go testHub.Run()

	origStore := store
	origHub := hub
	origPredEngine := predEngine
	store = testStore
	hub = testHub
	predEngine = nil
	defer func() {
		store = origStore
		hub = origHub
		predEngine = origPredEngine
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ebpf/events", ebpfEventsHandler)
	mux.HandleFunc("/ws/heartbeats", func(w http.ResponseWriter, r *http.Request) {
		ServeWS(testHub, w, r)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	// Connect WebSocket client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/heartbeats"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WS dial failed: %v", err)
	}
	defer ws.Close()

	// Give the hub time to register the client
	time.Sleep(50 * time.Millisecond)

	// POST a batch of EnrichedEvents
	events := []EnrichedEvent{
		{
			Timestamp: time.Now().UTC().Truncate(time.Second),
			PID:       1234,
			Comm:      "kubelet",
			EventType: "syscall",
			NodeName:  "node-01",
		},
		{
			Timestamp: time.Now().UTC().Truncate(time.Second),
			PID:       5678,
			Comm:      "containerd",
			EventType: "process",
			NodeName:  "node-02",
		},
		{
			Timestamp: time.Now().UTC().Truncate(time.Second),
			PID:       9012,
			Comm:      "coredns",
			EventType: "network",
			NodeName:  "node-03",
		},
	}

	body, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("failed to marshal events: %v", err)
	}

	resp, err := http.Post(server.URL+"/api/ebpf/events", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST status: got %d, want 201", resp.StatusCode)
	}

	// Read all expected WebSocket messages
	for i, expected := range events {
		ws.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msgBytes, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("event %d: WS read failed: %v", i, err)
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(msgBytes, &wsMsg); err != nil {
			t.Fatalf("event %d: WS unmarshal failed: %v", i, err)
		}

		if wsMsg.Type != "ebpf_event" {
			t.Fatalf("event %d: WS message type: got %q, want %q", i, wsMsg.Type, "ebpf_event")
		}

		// Decode the payload to verify it matches the posted event
		payloadBytes, _ := json.Marshal(wsMsg.Payload)
		var received EnrichedEvent
		if err := json.Unmarshal(payloadBytes, &received); err != nil {
			t.Fatalf("event %d: payload unmarshal failed: %v", i, err)
		}

		if received.PID != expected.PID {
			t.Fatalf("event %d: PID: got %d, want %d", i, received.PID, expected.PID)
		}
		if received.Comm != expected.Comm {
			t.Fatalf("event %d: Comm: got %q, want %q", i, received.Comm, expected.Comm)
		}
		if received.EventType != expected.EventType {
			t.Fatalf("event %d: EventType: got %q, want %q", i, received.EventType, expected.EventType)
		}
		if received.NodeName != expected.NodeName {
			t.Fatalf("event %d: NodeName: got %q, want %q", i, received.NodeName, expected.NodeName)
		}
	}
}
