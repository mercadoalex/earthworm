package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"earthworm/src/kubernetes"
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

// Dummy PodInfo slice for correlation testing
var podInfos = []kubernetes.PodInfo{
	{
		PodName:      "demo-pod",
		Namespace:    "default",
		NodeName:     "node-01",
		ContainerIDs: []string{"node-01"},
		CgroupPaths:  []string{"/sys/fs/cgroup/kubepods/node-01"},
	},
}

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
	logFile, err := os.OpenFile("earthworm.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	log.SetOutput(logFile)

	// Start HTTP server for heartbeat API
	http.HandleFunc("/api/heartbeat", heartbeatHandler)
	http.HandleFunc("/api/heartbeats", getHeartbeatsHandler)
	log.Println("Earthworm server running on :8080")

	// Generate 50 mock nodes
	nodes := kubernetes.GenerateMockNodes()

	// Simulate eBPF activity and print correlation results
	kubernetes.SimulateEBPFActivity(nodes, podInfos)

	// Optionally, print summary of generated nodes
	fmt.Println("\nSummary of mock nodes:")
	for _, node := range nodes {
		fmt.Printf("Node: %s, LastLease: %v, Status: %s\n", node.Name, node.LastLease.Format("15:04:05"), node.Status)
	}

	log.Fatal(http.ListenAndServe(":8080", nil))
}
