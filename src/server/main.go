package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"earthworm/src/kubernetes"
)

// Global store, config, hub, anomaly detector, alert dispatcher, and eBPF components.
var (
	store        Store
	cfg          Config
	hub          *Hub
	detector     *AnomalyDetector
	dispatcher   *AlertDispatcher
	chainBuilder *CausalChainBuilder
	predEngine   *PredictionEngine
	replayStore  *ReplayStore
	ebpfEnabled  bool
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

// ebpfEventsHandler receives enriched kernel events from the agent via POST.
func ebpfEventsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var events []EnrichedEvent
	if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
		// Try single event
		var single EnrichedEvent
		if err2 := json.Unmarshal([]byte(err.Error()), &single); err2 != nil {
			writeJSONError(w, fmt.Sprintf("Invalid JSON: %s", err.Error()), http.StatusBadRequest)
			return
		}
		events = []EnrichedEvent{single}
	}

	for _, event := range events {
		if err := store.SaveKernelEvent(context.Background(), event); err != nil {
			log.Printf("Failed to save kernel event: %v", err)
			continue
		}
		if hub != nil {
			hub.BroadcastEbpfEvent(event)
		}
		if predEngine != nil {
			predEngine.Analyze(event.NodeName, []EnrichedEvent{event})
		}
	}

	w.WriteHeader(http.StatusCreated)
}

// heartbeatHandler receives heartbeat data via POST.
func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var hb Heartbeat
	if err := json.NewDecoder(r.Body).Decode(&hb); err != nil {
		writeJSONError(w, fmt.Sprintf("Invalid JSON: %s", err.Error()), http.StatusBadRequest)
		return
	}
	if err := store.Save(context.Background(), hb); err != nil {
		writeJSONError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Broadcast to WebSocket clients
	if hub != nil {
		hub.BroadcastHeartbeat(hb)
	}

	// Evaluate for anomalies
	if detector != nil {
		if alert := detector.Evaluate(hb); alert != nil {
			if dispatcher != nil {
				dispatcher.Dispatch(*alert)
			}
		}
	}

	w.WriteHeader(http.StatusCreated)
}

// getHeartbeatsHandler serves heartbeat data via GET.
func getHeartbeatsHandler(w http.ResponseWriter, r *http.Request) {
	// Use a wide time range to return all heartbeats
	from := time.Time{}
	to := time.Now().Add(24 * time.Hour)
	hbs, err := store.GetByTimeRange(context.Background(), from, to)
	if err != nil {
		writeJSONError(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}
	if hbs == nil {
		hbs = []Heartbeat{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hbs)
}

// setCORS adds CORS headers based on configured origins.
func setCORS(next http.Handler, origins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := "*"
		if len(origins) > 0 {
			origin = origins[0]
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// CLI flags for simulation mode
	simMode := flag.Bool("sim-mode", false, "Enable simulation mode (use SimulationEngine instead of GenerateMockNodes)")
	simDuration := flag.Duration("sim-duration", 4*time.Hour, "Simulation duration (e.g. 4h, 30m)")
	simNodes := flag.Int("sim-nodes", 50, "Number of simulated nodes")
	simOutput := flag.String("sim-output", "", "Output directory for simulation data files")
	simSeed := flag.Int64("sim-seed", 0, "Random seed for simulation (0 = time-based)")
	ebpfFlag := flag.Bool("ebpf", false, "Enable eBPF kernel observability (requires Linux 5.8+ with CAP_BPF)")
	flag.Parse()

	ebpfEnabled = *ebpfFlag

	cfg = LoadConfig()

	logFile, err := os.OpenFile(cfg.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	log.SetOutput(logFile)

	// Initialize store based on config
	switch cfg.StoreType {
	case "redis":
		store = NewRedisStore(cfg.RedisAddr)
	default:
		store = NewMemoryStore()
	}

	// Verify store connectivity
	if err := store.Ping(context.Background()); err != nil {
		log.Fatalf("Failed to connect to store: %v", err)
	}

	// Initialize WebSocket hub
	hub = NewHub()
	go hub.Run()

	// Initialize anomaly detector and alert dispatcher
	detector = NewAnomalyDetector(store, cfg.WarningThresholdS, cfg.CriticalThresholdS)
	dispatcher = NewAlertDispatcher(cfg.WebhookURL, hub.BroadcastAlert)

	// Initialize eBPF components (causal chain builder, prediction engine, replay store)
	chainBuilder = NewCausalChainBuilder(store, hub)
	predEngine = NewPredictionEngine(store, hub)
	replayStore = NewReplayStore(store, defaultRetention)

	if ebpfEnabled {
		log.Println("eBPF kernel observability enabled")
	} else {
		log.Println("eBPF kernel observability disabled, running in mock mode")
	}

	// Set up HTTP routes
	// WebSocket endpoint is registered directly (no CORS middleware — the upgrader handles origin checks)
	topMux := http.NewServeMux()
	topMux.HandleFunc("/ws/heartbeats", func(w http.ResponseWriter, r *http.Request) {
		ServeWS(hub, w, r)
	})

	// API routes go through CORS + logging middleware
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/heartbeat", heartbeatHandler)
	apiMux.HandleFunc("/api/heartbeats", getHeartbeatsHandler)
	apiMux.HandleFunc("/api/ebpf/events", ebpfEventsHandler)
	apiMux.HandleFunc("/api/replay", replayHandler(replayStore))
	apiMux.HandleFunc("/api/predictions/accuracy", predictionAccuracyHandler(predEngine))
	topMux.Handle("/api/", LoggingMiddleware(setCORS(apiMux, cfg.CORSOrigins)))

	handler := http.Handler(topMux)

	log.Printf("Earthworm server running on :%d", cfg.Port)

	var nodes []kubernetes.MockNode

	if *simMode {
		// Use SimulationEngine for realistic data generation
		simConfig := kubernetes.SimulationConfig{
			NodeCount: *simNodes,
			Duration:  *simDuration,
			Seed:      *simSeed,
			Scenarios: []kubernetes.ScenarioConfig{
				{
					Type:             "rolling_deployment",
					TriggerAt:        30 * time.Minute,
					NodeCount:        5,
					StaggerInterval:  30 * time.Second,
					ReplacementDelay: 15 * time.Second,
				},
			},
		}

		engine, err := kubernetes.NewSimulationEngine(simConfig)
		if err != nil {
			log.Fatalf("Failed to create simulation engine: %v", err)
		}

		fmt.Printf("Running simulation: %d nodes, %v duration, seed=%d\n", *simNodes, *simDuration, *simSeed)
		result, err := engine.Run()
		if err != nil {
			log.Fatalf("Simulation failed: %v", err)
		}

		fmt.Printf("Simulation complete: %d leases, %d eBPF events, %d files\n",
			result.Stats.TotalLeases, result.Stats.TotalEbpfEvents, len(result.LeaseFiles))

		// Write output files if output directory specified
		if *simOutput != "" {
			if err := kubernetes.WriteSimulationOutput(result, *simOutput); err != nil {
				log.Fatalf("Failed to write simulation output: %v", err)
			}
			fmt.Printf("Simulation output written to %s\n", *simOutput)
		}

		// Create mock nodes from simulation for live broadcast
		nodes = make([]kubernetes.MockNode, 0, result.Stats.TotalNodes)
		for i := 0; i < result.Stats.TotalNodes && i < *simNodes; i++ {
			nodes = append(nodes, kubernetes.MockNode{
				Name:      fmt.Sprintf("node-%03d", i),
				Status:    "Ready",
				LastLease: time.Now(),
			})
		}
	} else {
		// Generate 50 mock nodes (original behavior)
		nodes = kubernetes.GenerateMockNodes()

		// Simulate eBPF activity and print correlation results
		kubernetes.SimulateEBPFActivity(nodes, podInfos)

		// Print summary of generated nodes
		fmt.Println("\nSummary of mock nodes:")
		for _, node := range nodes {
			fmt.Printf("Node: %s, LastLease: %v, Status: %s\n", node.Name, node.LastLease.Format("15:04:05"), node.Status)
		}
	}

	// Start live heartbeat simulation — broadcasts a heartbeat from a random node every 3 seconds
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			idx := time.Now().UnixNano() % int64(len(nodes))
			node := nodes[idx]
			hb := Heartbeat{
				NodeName:  node.Name,
				Namespace: "default",
				Timestamp: time.Now(),
				Status:    node.Status,
			}
			_ = store.Save(context.Background(), hb)
			if hub != nil {
				hub.BroadcastHeartbeat(hb)
			}
			if detector != nil {
				if alert := detector.Evaluate(hb); alert != nil {
					if dispatcher != nil {
						dispatcher.Dispatch(*alert)
					}
				}
			}
		}
	}()

	fmt.Printf("\nServer running on :%d — broadcasting live heartbeats every 3s\n", cfg.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), handler))
}
