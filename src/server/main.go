package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// Heartbeat represents a heartbeat event from a Kubernetes node.
type Heartbeat struct {
	NodeName  string    `json:"nodeName"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

// In-memory storage for heartbeat events.
var (
	heartbeats []Heartbeat
	mu         sync.Mutex
)

// Handler to receive heartbeat data (POST).
func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var hb Heartbeat
	if err := json.NewDecoder(r.Body).Decode(&hb); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	mu.Lock()
	heartbeats = append(heartbeats, hb)
	mu.Unlock()
	w.WriteHeader(http.StatusCreated)
}

// Handler to serve heartbeat data (GET).
func getHeartbeatsHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(heartbeats)
}

func main() {
	http.HandleFunc("/api/heartbeat", heartbeatHandler)
	http.HandleFunc("/api/heartbeats", getHeartbeatsHandler)
	log.Println("Earthworm server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
