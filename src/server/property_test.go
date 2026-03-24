package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"pgregory.net/rapid"
)

// Feature: earthworm-improvements, Property 1: Server config round-trip from environment
// **Validates: Requirements 2.1**
func TestProperty1_ConfigRoundTripFromEnvironment(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		port := rapid.IntRange(1, 65535).Draw(t, "port")
		logFile := rapid.StringMatching(`[a-z]{1,10}\.log`).Draw(t, "logFile")
		origin1 := rapid.StringMatching(`https?://[a-z]{3,8}\.[a-z]{2,4}`).Draw(t, "origin1")
		origin2 := rapid.StringMatching(`https?://[a-z]{3,8}\.[a-z]{2,4}`).Draw(t, "origin2")
		corsOrigins := origin1 + "," + origin2
		storeType := rapid.SampledFrom([]string{"memory", "redis"}).Draw(t, "storeType")
		redisAddr := rapid.StringMatching(`[a-z]{3,8}:[0-9]{4,5}`).Draw(t, "redisAddr")
		warningThreshold := rapid.IntRange(1, 100).Draw(t, "warningThreshold")
		criticalThreshold := rapid.IntRange(warningThreshold+1, 200).Draw(t, "criticalThreshold")
		webhookURL := rapid.StringMatching(`https?://[a-z]{3,10}\.[a-z]{2,4}/[a-z]{2,8}`).Draw(t, "webhookURL")

		os.Setenv("EARTHWORM_PORT", strconv.Itoa(port))
		os.Setenv("EARTHWORM_LOG_FILE", logFile)
		os.Setenv("EARTHWORM_CORS_ORIGINS", corsOrigins)
		os.Setenv("EARTHWORM_STORE", storeType)
		os.Setenv("EARTHWORM_REDIS_ADDR", redisAddr)
		os.Setenv("EARTHWORM_WARNING_THRESHOLD", strconv.Itoa(warningThreshold))
		os.Setenv("EARTHWORM_CRITICAL_THRESHOLD", strconv.Itoa(criticalThreshold))
		os.Setenv("EARTHWORM_WEBHOOK_URL", webhookURL)
		defer func() {
			os.Unsetenv("EARTHWORM_PORT")
			os.Unsetenv("EARTHWORM_LOG_FILE")
			os.Unsetenv("EARTHWORM_CORS_ORIGINS")
			os.Unsetenv("EARTHWORM_STORE")
			os.Unsetenv("EARTHWORM_REDIS_ADDR")
			os.Unsetenv("EARTHWORM_WARNING_THRESHOLD")
			os.Unsetenv("EARTHWORM_CRITICAL_THRESHOLD")
			os.Unsetenv("EARTHWORM_WEBHOOK_URL")
		}()

		cfg := LoadConfig()

		if cfg.Port != port {
			t.Fatalf("Port: got %d, want %d", cfg.Port, port)
		}
		if cfg.LogFilePath != logFile {
			t.Fatalf("LogFilePath: got %q, want %q", cfg.LogFilePath, logFile)
		}
		expectedOrigins := strings.Split(corsOrigins, ",")
		if len(cfg.CORSOrigins) != len(expectedOrigins) {
			t.Fatalf("CORSOrigins length: got %d, want %d", len(cfg.CORSOrigins), len(expectedOrigins))
		}
		for i, o := range cfg.CORSOrigins {
			if o != expectedOrigins[i] {
				t.Fatalf("CORSOrigins[%d]: got %q, want %q", i, o, expectedOrigins[i])
			}
		}
		if cfg.StoreType != storeType {
			t.Fatalf("StoreType: got %q, want %q", cfg.StoreType, storeType)
		}
		if cfg.RedisAddr != redisAddr {
			t.Fatalf("RedisAddr: got %q, want %q", cfg.RedisAddr, redisAddr)
		}
		if cfg.WarningThresholdS != warningThreshold {
			t.Fatalf("WarningThresholdS: got %d, want %d", cfg.WarningThresholdS, warningThreshold)
		}
		if cfg.CriticalThresholdS != criticalThreshold {
			t.Fatalf("CriticalThresholdS: got %d, want %d", cfg.CriticalThresholdS, criticalThreshold)
		}
		if cfg.WebhookURL != webhookURL {
			t.Fatalf("WebhookURL: got %q, want %q", cfg.WebhookURL, webhookURL)
		}
	})
}


// Feature: earthworm-improvements, Property 2: Invalid HTTP method returns structured 405
// **Validates: Requirements 4.1**
func TestProperty2_InvalidHTTPMethodReturns405(t *testing.T) {
	// Set up a local store for the handler
	origStore := store
	store = NewMemoryStore()
	defer func() { store = origStore }()

	rapid.Check(t, func(t *rapid.T) {
		method := rapid.SampledFrom([]string{"GET", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "TRACE", "CONNECT"}).Draw(t, "method")

		req := httptest.NewRequest(method, "/api/heartbeat", nil)
		rec := httptest.NewRecorder()
		heartbeatHandler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("method %s: got status %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}

		var errResp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
			t.Fatalf("method %s: failed to decode JSON error response: %v", method, err)
		}
		if errResp.Error == "" {
			t.Fatalf("method %s: error field is empty", method)
		}
	})
}

// Feature: earthworm-improvements, Property 3: Malformed JSON returns structured 400
// **Validates: Requirements 4.2**
func TestProperty3_MalformedJSONReturns400(t *testing.T) {
	origStore := store
	store = NewMemoryStore()
	defer func() { store = origStore }()

	rapid.Check(t, func(t *rapid.T) {
		// Generate strings that are definitely not valid JSON objects
		kind := rapid.IntRange(0, 3).Draw(t, "kind")
		var body string
		switch kind {
		case 0:
			body = rapid.StringMatching(`[a-zA-Z]{1,20}`).Draw(t, "alphaBody")
		case 1:
			body = rapid.StringMatching(`\{[a-z]+`).Draw(t, "brokenJSON")
		case 2:
			body = rapid.StringMatching(`[0-9]{1,10}`).Draw(t, "numericBody")
		case 3:
			body = rapid.StringMatching(`<[a-z]+>`).Draw(t, "xmlLike")
		}

		req := httptest.NewRequest(http.MethodPost, "/api/heartbeat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		heartbeatHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q: got status %d, want %d", body, rec.Code, http.StatusBadRequest)
		}

		var errResp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
			t.Fatalf("body %q: failed to decode JSON error response: %v", body, err)
		}
		if errResp.Error == "" {
			t.Fatalf("body %q: error field is empty", body)
		}
	})
}

// Feature: earthworm-improvements, Property 4: Logging middleware captures request metadata
// **Validates: Requirements 4.3**
func TestProperty4_LoggingMiddlewareCapturesMetadata(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		method := rapid.SampledFrom([]string{"GET", "POST", "PUT", "DELETE", "PATCH"}).Draw(t, "method")
		pathSeg := rapid.StringMatching(`/[a-z]{1,10}`).Draw(t, "path")

		// Capture log output
		var logBuf bytes.Buffer
		origOutput := log.Writer()
		log.SetOutput(&logBuf)
		defer log.SetOutput(origOutput)

		statusCode := rapid.SampledFrom([]int{200, 201, 400, 404, 500}).Draw(t, "statusCode")

		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(statusCode)
		})

		handler := LoggingMiddleware(inner)
		req := httptest.NewRequest(method, pathSeg, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		logOutput := logBuf.String()

		if !strings.Contains(logOutput, fmt.Sprintf("method=%s", method)) {
			t.Fatalf("log missing method=%s in: %s", method, logOutput)
		}
		if !strings.Contains(logOutput, fmt.Sprintf("path=%s", pathSeg)) {
			t.Fatalf("log missing path=%s in: %s", pathSeg, logOutput)
		}
		if !strings.Contains(logOutput, fmt.Sprintf("status=%d", statusCode)) {
			t.Fatalf("log missing status=%d in: %s", statusCode, logOutput)
		}
		if !strings.Contains(logOutput, "duration=") {
			t.Fatalf("log missing duration= in: %s", logOutput)
		}
	})
}


// Feature: earthworm-improvements, Property 5: WebSocket broadcast of heartbeat events
// **Validates: Requirements 5.2, 13.2**
func TestProperty5_WebSocketBroadcastHeartbeat(t *testing.T) {
	// Set up server infrastructure
	origStore := store
	origHub := hub
	origDetector := detector
	origDispatcher := dispatcher

	store = NewMemoryStore()
	hub = NewHub()
	go hub.Run()
	detector = nil
	dispatcher = nil
	defer func() {
		store = origStore
		hub = origHub
		detector = origDetector
		dispatcher = origDispatcher
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/heartbeat", heartbeatHandler)
	mux.HandleFunc("/ws/heartbeats", func(w http.ResponseWriter, r *http.Request) {
		ServeWS(hub, w, r)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	rapid.Check(t, func(t *rapid.T) {
		nodeName := rapid.StringMatching(`node-[a-z0-9]{1,8}`).Draw(t, "nodeName")
		namespace := rapid.StringMatching(`ns-[a-z]{1,6}`).Draw(t, "namespace")
		status := rapid.SampledFrom([]string{"Ready", "NotReady"}).Draw(t, "status")
		ts := time.Now().UTC().Truncate(time.Second)

		// Connect WS client
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/heartbeats"
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("WS dial failed: %v", err)
		}
		defer ws.Close()

		// Give the hub time to register the client
		time.Sleep(50 * time.Millisecond)

		// POST heartbeat
		hb := Heartbeat{NodeName: nodeName, Namespace: namespace, Timestamp: ts, Status: status}
		body, _ := json.Marshal(hb)
		resp, err := http.Post(server.URL+"/api/heartbeat", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("POST status: got %d, want %d", resp.StatusCode, http.StatusCreated)
		}

		// Read WS message with 500ms timeout
		ws.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, msgBytes, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("WS read failed: %v", err)
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(msgBytes, &wsMsg); err != nil {
			t.Fatalf("WS unmarshal failed: %v", err)
		}
		if wsMsg.Type != "heartbeat" {
			t.Fatalf("WS message type: got %q, want %q", wsMsg.Type, "heartbeat")
		}

		// Verify payload contains the node name
		payloadBytes, _ := json.Marshal(wsMsg.Payload)
		var receivedHB Heartbeat
		json.Unmarshal(payloadBytes, &receivedHB)
		if receivedHB.NodeName != nodeName {
			t.Fatalf("WS payload nodeName: got %q, want %q", receivedHB.NodeName, nodeName)
		}
	})
}

// Feature: earthworm-improvements, Property 8: Anomaly detection emits correct severity alerts
// **Validates: Requirements 7.2, 7.3, 7.5**
func TestProperty8_AnomalyDetectionSeverity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		warningS := rapid.IntRange(5, 30).Draw(t, "warningS")
		criticalS := rapid.IntRange(warningS+1, 120).Draw(t, "criticalS")
		gapS := rapid.IntRange(0, 200).Draw(t, "gapS")
		nodeName := rapid.StringMatching(`node-[a-z0-9]{1,6}`).Draw(t, "nodeName")
		namespace := rapid.StringMatching(`ns-[a-z]{1,4}`).Draw(t, "namespace")

		memStore := NewMemoryStore()
		baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

		// Save a baseline heartbeat
		baseline := Heartbeat{NodeName: nodeName, Namespace: namespace, Timestamp: baseTime, Status: "Ready"}
		memStore.Save(context.Background(), baseline)

		det := NewAnomalyDetector(memStore, warningS, criticalS)

		// Create incoming event with the specified gap
		incoming := Heartbeat{
			NodeName:  nodeName,
			Namespace: namespace,
			Timestamp: baseTime.Add(time.Duration(gapS) * time.Second),
			Status:    "Ready",
		}

		alert := det.Evaluate(incoming)

		warningThreshold := time.Duration(warningS) * time.Second
		criticalThreshold := time.Duration(criticalS) * time.Second
		gap := time.Duration(gapS) * time.Second

		if gap <= warningThreshold {
			if alert != nil {
				t.Fatalf("gap=%ds <= warning=%ds: expected no alert, got severity=%q", gapS, warningS, alert.Severity)
			}
		} else if gap <= criticalThreshold {
			if alert == nil {
				t.Fatalf("gap=%ds in warning range: expected warning alert, got nil", gapS)
			}
			if alert.Severity != "warning" {
				t.Fatalf("gap=%ds: expected severity=warning, got %q", gapS, alert.Severity)
			}
			if alert.NodeName != nodeName {
				t.Fatalf("alert NodeName: got %q, want %q", alert.NodeName, nodeName)
			}
			if alert.Namespace != namespace {
				t.Fatalf("alert Namespace: got %q, want %q", alert.Namespace, namespace)
			}
			if alert.Gap != float64(gapS) {
				t.Fatalf("alert Gap: got %f, want %f", alert.Gap, float64(gapS))
			}
		} else {
			if alert == nil {
				t.Fatalf("gap=%ds > critical=%ds: expected critical alert, got nil", gapS, criticalS)
			}
			if alert.Severity != "critical" {
				t.Fatalf("gap=%ds: expected severity=critical, got %q", gapS, alert.Severity)
			}
			if alert.NodeName != nodeName {
				t.Fatalf("alert NodeName: got %q, want %q", alert.NodeName, nodeName)
			}
		}
	})
}


// Feature: earthworm-improvements, Property 9: Alerts broadcast to WebSocket clients
// **Validates: Requirements 7.6**
func TestProperty9_AlertsBroadcastToWSClients(t *testing.T) {
	testHub := NewHub()
	go testHub.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/heartbeats", func(w http.ResponseWriter, r *http.Request) {
		ServeWS(testHub, w, r)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	rapid.Check(t, func(t *rapid.T) {
		nodeName := rapid.StringMatching(`node-[a-z0-9]{1,6}`).Draw(t, "nodeName")
		namespace := rapid.StringMatching(`ns-[a-z]{1,4}`).Draw(t, "namespace")
		gapSeconds := rapid.Float64Range(1.0, 200.0).Draw(t, "gapSeconds")
		severity := rapid.SampledFrom([]string{"warning", "critical"}).Draw(t, "severity")

		// Connect WS client
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/heartbeats"
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("WS dial failed: %v", err)
		}
		defer ws.Close()

		time.Sleep(50 * time.Millisecond)

		alert := Alert{
			NodeName:  nodeName,
			Namespace: namespace,
			Gap:       gapSeconds,
			Severity:  severity,
			Timestamp: time.Now().UTC(),
		}

		testHub.BroadcastAlert(alert)

		ws.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, msgBytes, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("WS read failed: %v", err)
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(msgBytes, &wsMsg); err != nil {
			t.Fatalf("WS unmarshal failed: %v", err)
		}
		if wsMsg.Type != "alert" {
			t.Fatalf("WS message type: got %q, want %q", wsMsg.Type, "alert")
		}

		payloadBytes, _ := json.Marshal(wsMsg.Payload)
		var receivedAlert Alert
		json.Unmarshal(payloadBytes, &receivedAlert)
		if receivedAlert.NodeName != nodeName {
			t.Fatalf("alert NodeName: got %q, want %q", receivedAlert.NodeName, nodeName)
		}
		if receivedAlert.Severity != severity {
			t.Fatalf("alert Severity: got %q, want %q", receivedAlert.Severity, severity)
		}
	})
}

// Feature: earthworm-improvements, Property 15: Heartbeat POST-then-GET round-trip
// **Validates: Requirements 11.6, 13.1**
func TestProperty15_HeartbeatPostThenGetRoundTrip(t *testing.T) {
	origStore := store
	origHub := hub
	origDetector := detector
	origDispatcher := dispatcher

	store = NewMemoryStore()
	hub = NewHub()
	go hub.Run()
	detector = nil
	dispatcher = nil
	defer func() {
		store = origStore
		hub = origHub
		detector = origDetector
		dispatcher = origDispatcher
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/heartbeat", heartbeatHandler)
	mux.HandleFunc("/api/heartbeats", getHeartbeatsHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	rapid.Check(t, func(t *rapid.T) {
		nodeName := rapid.StringMatching(`node-[a-z0-9]{1,8}`).Draw(t, "nodeName")
		namespace := rapid.StringMatching(`ns-[a-z]{1,6}`).Draw(t, "namespace")
		status := rapid.SampledFrom([]string{"Ready", "NotReady"}).Draw(t, "status")
		ts := time.Now().UTC().Truncate(time.Second)

		hb := Heartbeat{NodeName: nodeName, Namespace: namespace, Timestamp: ts, Status: status}
		body, _ := json.Marshal(hb)

		// POST
		resp, err := http.Post(server.URL+"/api/heartbeat", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("POST status: got %d, want %d", resp.StatusCode, http.StatusCreated)
		}

		// GET
		getResp, err := http.Get(server.URL + "/api/heartbeats")
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer getResp.Body.Close()

		var heartbeats []Heartbeat
		respBody, _ := io.ReadAll(getResp.Body)
		if err := json.Unmarshal(respBody, &heartbeats); err != nil {
			t.Fatalf("GET unmarshal failed: %v", err)
		}

		found := false
		for _, h := range heartbeats {
			if h.NodeName == nodeName && h.Status == status && h.Timestamp.Equal(ts) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("GET response does not contain posted heartbeat: nodeName=%q, status=%q, ts=%v", nodeName, status, ts)
		}
	})
}
