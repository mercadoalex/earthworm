package main

import (
    "encoding/json"
    "net/http"
    "sync"
)

// HeartbeatData represents the structure of the heartbeat data received from the Kubernetes monitor.
type HeartbeatData struct {
    ClusterName string `json:"cluster_name"` // Name of the Kubernetes cluster
    Timestamp   int64  `json:"timestamp"`    // Timestamp of the heartbeat signal
    Status      string `json:"status"`       // Status of the cluster (e.g., healthy, unhealthy)
}

// Server holds the state of the HTTP server and the heartbeat data.
type Server struct {
    mu      sync.Mutex             // Mutex to protect shared data
    data    []HeartbeatData        // Slice to store received heartbeat data
}

// NewServer initializes a new Server instance.
func NewServer() *Server {
    return &Server{
        data: make([]HeartbeatData, 0), // Initialize the data slice
    }
}

// handleHeartbeat is the HTTP handler for receiving heartbeat data.
func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
    // Only allow POST requests
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var heartbeat HeartbeatData
    // Decode the JSON body into the HeartbeatData struct
    if err := json.NewDecoder(r.Body).Decode(&heartbeat); err != nil {
        http.Error(w, "Bad request", http.StatusBadRequest)
        return
    }

    // Lock the mutex to safely update the shared data
    s.mu.Lock()
    s.data = append(s.data, heartbeat) // Append the new heartbeat data
    s.mu.Unlock()

    // Respond with a success message
    w.WriteHeader(http.StatusAccepted)
}

// getHeartbeats is the HTTP handler for retrieving the heartbeat data.
func (s *Server) getHeartbeats(w http.ResponseWriter, r *http.Request) {
    s.mu.Lock()
    defer s.mu.Unlock() // Ensure the mutex is unlocked after the function completes

    // Encode the current heartbeat data as JSON and send it in the response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(s.data)
}

// main function initializes the server and starts listening for requests.
func main() {
    server := NewServer() // Create a new server instance

    // Set up HTTP routes
    http.HandleFunc("/heartbeat", server.handleHeartbeat) // Endpoint to receive heartbeat data
    http.HandleFunc("/heartbeats", server.getHeartbeats)   // Endpoint to retrieve heartbeat data

    // Start the HTTP server on port 8080
    if err := http.ListenAndServe(":8080", nil); err != nil {
        panic(err) // Panic if the server fails to start
    }
}