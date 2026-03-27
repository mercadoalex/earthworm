package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- Task 12.4: Integration test for POST-then-GET round-trip ---
// Validates: Requirements 11.6

// setupIntegrationGlobals initialises the global variables (store, hub,
// detector, dispatcher, etc.) with fresh in-memory instances and returns a
// teardown function that restores the originals.
func setupIntegrationGlobals() func() {
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

	return func() {
		store = origStore
		hub = origHub
		detector = origDetector
		dispatcher = origDispatcher
		chainBuilder = origChainBuilder
		predEngine = origPredEngine
		replayStore = origReplayStore
		topoMap = origTopoMap
	}
}

func TestIntegration_PostThenGet_RoundTrip(t *testing.T) {
	cleanup := setupIntegrationGlobals()
	defer cleanup()

	// Build the same mux the real server uses so the test exercises the full
	// handler chain (CORS + logging middleware).
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/heartbeat", heartbeatHandler)
	apiMux.HandleFunc("/api/heartbeats", getHeartbeatsHandler)

	topMux := http.NewServeMux()
	topMux.Handle("/api/", LoggingMiddleware(setCORS(apiMux, []string{"*"})))

	ts := httptest.NewServer(topMux)
	defer ts.Close()

	// --- POST a valid heartbeat ---
	now := time.Now().UTC().Truncate(time.Second)
	posted := Heartbeat{
		NodeName:  "integration-node-01",
		Namespace: "kube-system",
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

	// --- GET heartbeats and verify the posted event is present ---
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
