package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// errorStore is a mock Store that returns configurable errors for testing 500/503 paths.
type errorStore struct {
	saveErr           error
	getByTimeRangeErr error
}

func (e *errorStore) Save(_ context.Context, _ Heartbeat) error {
	return e.saveErr
}
func (e *errorStore) GetByTimeRange(_ context.Context, _, _ time.Time) ([]Heartbeat, error) {
	return nil, e.getByTimeRangeErr
}
func (e *errorStore) GetLatestByNode(_ context.Context, _ string) (*Heartbeat, error) {
	return nil, nil
}
func (e *errorStore) Ping(_ context.Context) error { return nil }
func (e *errorStore) SaveKernelEvent(_ context.Context, _ EnrichedEvent) error {
	return nil
}
func (e *errorStore) GetKernelEvents(_ context.Context, _ string, _, _ time.Time) ([]EnrichedEvent, error) {
	return nil, nil
}
func (e *errorStore) GetKernelEventsByType(_ context.Context, _ string, _ string, _, _ time.Time) ([]EnrichedEvent, error) {
	return nil, nil
}
func (e *errorStore) SaveCausalChain(_ context.Context, _ CausalChain) error { return nil }
func (e *errorStore) GetCausalChains(_ context.Context, _ string, _, _ time.Time) ([]CausalChain, error) {
	return nil, nil
}

// --- Task 13.1: Unit tests for POST/GET heartbeat handlers ---
// Validates: Requirements 11.1, 11.2

func setupTestStore() func() {
	origStore := store
	origHub := hub
	origDetector := detector
	origDispatcher := dispatcher

	store = NewMemoryStore()
	hub = NewHub()
	go hub.Run()
	detector = nil
	dispatcher = nil

	return func() {
		store = origStore
		hub = origHub
		detector = origDetector
		dispatcher = origDispatcher
	}
}

func TestUnit_HeartbeatHandler_ValidPOST_Returns201(t *testing.T) {
	cleanup := setupTestStore()
	defer cleanup()

	hb := Heartbeat{
		NodeName:  "node-01",
		Namespace: "default",
		Timestamp: time.Now().UTC().Truncate(time.Second),
		Status:    "Ready",
	}
	body, _ := json.Marshal(hb)

	req := httptest.NewRequest(http.MethodPost, "/api/heartbeat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	heartbeatHandler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	// Verify the heartbeat was actually stored
	from := time.Time{}
	to := time.Now().Add(24 * time.Hour)
	stored, err := store.GetByTimeRange(context.Background(), from, to)
	if err != nil {
		t.Fatalf("failed to query store: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored heartbeat, got %d", len(stored))
	}
	if stored[0].NodeName != hb.NodeName || stored[0].Namespace != hb.Namespace || stored[0].Status != hb.Status {
		t.Fatalf("stored heartbeat does not match: got %+v", stored[0])
	}
}

func TestUnit_HeartbeatHandler_MalformedPOST_Returns400(t *testing.T) {
	cleanup := setupTestStore()
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/heartbeat", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	heartbeatHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error == "" {
		t.Fatal("expected non-empty error field")
	}
}

func TestUnit_HeartbeatHandler_WrongMethod_Returns405(t *testing.T) {
	cleanup := setupTestStore()
	defer cleanup()

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(method, "/api/heartbeat", nil)
		rec := httptest.NewRecorder()

		heartbeatHandler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("method %s: expected status 405, got %d", method, rec.Code)
		}

		var errResp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
			t.Fatalf("method %s: failed to decode error response: %v", method, err)
		}
		if errResp.Error == "" {
			t.Fatalf("method %s: expected non-empty error field", method)
		}
	}
}

func TestUnit_GetHeartbeatsHandler_EmptyStore_Returns200WithEmptyArray(t *testing.T) {
	cleanup := setupTestStore()
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/heartbeats", nil)
	rec := httptest.NewRecorder()

	getHeartbeatsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var heartbeats []Heartbeat
	if err := json.NewDecoder(rec.Body).Decode(&heartbeats); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(heartbeats) != 0 {
		t.Fatalf("expected empty array, got %d items", len(heartbeats))
	}
}

func TestUnit_GetHeartbeatsHandler_PopulatedStore_Returns200WithData(t *testing.T) {
	cleanup := setupTestStore()
	defer cleanup()

	ts := time.Now().UTC().Truncate(time.Second)
	hb := Heartbeat{
		NodeName:  "node-test",
		Namespace: "kube-system",
		Timestamp: ts,
		Status:    "Ready",
	}
	store.Save(context.Background(), hb)

	req := httptest.NewRequest(http.MethodGet, "/api/heartbeats", nil)
	rec := httptest.NewRecorder()

	getHeartbeatsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var heartbeats []Heartbeat
	if err := json.NewDecoder(rec.Body).Decode(&heartbeats); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(heartbeats) == 0 {
		t.Fatal("expected non-empty response")
	}

	found := false
	for _, h := range heartbeats {
		if h.NodeName == "node-test" && h.Namespace == "kube-system" && h.Status == "Ready" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find the stored heartbeat in response")
	}
}

func TestUnit_HeartbeatHandler_InternalError_Returns500(t *testing.T) {
	origStore := store
	origHub := hub
	origDetector := detector
	origDispatcher := dispatcher
	defer func() {
		store = origStore
		hub = origHub
		detector = origDetector
		dispatcher = origDispatcher
	}()

	store = &errorStore{saveErr: errors.New("disk full")}
	hub = NewHub()
	go hub.Run()
	detector = nil
	dispatcher = nil

	hb := Heartbeat{
		NodeName:  "node-01",
		Namespace: "default",
		Timestamp: time.Now().UTC().Truncate(time.Second),
		Status:    "Ready",
	}
	body, _ := json.Marshal(hb)

	req := httptest.NewRequest(http.MethodPost, "/api/heartbeat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	heartbeatHandler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error == "" {
		t.Fatal("expected non-empty error field")
	}
	// Should return generic message, not expose internal details
	if errResp.Error != "Internal server error" {
		t.Fatalf("expected generic error message, got %q", errResp.Error)
	}
}

func TestUnit_GetHeartbeatsHandler_StoreUnreachable_Returns503(t *testing.T) {
	origStore := store
	origHub := hub
	origDetector := detector
	origDispatcher := dispatcher
	defer func() {
		store = origStore
		hub = origHub
		detector = origDetector
		dispatcher = origDispatcher
	}()

	store = &errorStore{getByTimeRangeErr: errors.New("connection refused")}
	hub = NewHub()
	go hub.Run()
	detector = nil
	dispatcher = nil

	req := httptest.NewRequest(http.MethodGet, "/api/heartbeats", nil)
	rec := httptest.NewRecorder()

	getHeartbeatsHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error == "" {
		t.Fatal("expected non-empty error field")
	}
}

// --- Task 5.1: Unit test for server rejecting invalid JSON on /api/ebpf/events ---
// Validates: Requirements 9.2

func TestUnit_EbpfEventsHandler_MalformedJSON_Returns400(t *testing.T) {
	cleanup := setupTestStore()
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/ebpf/events", bytes.NewReader([]byte("this is not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	ebpfEventsHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error == "" {
		t.Fatal("expected non-empty error field in response")
	}
}
