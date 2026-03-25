package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	// CLI flags
	serverURL := flag.String("server-url", "http://localhost:8080", "Earthworm server URL for event forwarding")
	pollInterval := flag.Duration("poll-interval", 100*time.Millisecond, "Ring buffer poll interval")
	ringBufferSize := flag.Int("ring-buffer-size", 256, "Ring buffer size in KB")
	cgroupRefresh := flag.Duration("cgroup-refresh", 30*time.Second, "Cgroup-to-pod cache refresh interval")
	nodeName := flag.String("node-name", "", "Node name (defaults to hostname)")
	kubeletURL := flag.String("kubelet-url", "http://localhost:10255", "Kubelet API URL for cgroup resolution")
	flag.Parse()

	if *nodeName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatalf("failed to get hostname: %v", err)
		}
		*nodeName = hostname
	}

	log.Printf("Earthworm agent starting on node %s", *nodeName)
	log.Printf("  server-url:       %s", *serverURL)
	log.Printf("  poll-interval:    %v", *pollInterval)
	log.Printf("  ring-buffer-size: %d KB", *ringBufferSize)
	log.Printf("  cgroup-refresh:   %v", *cgroupRefresh)

	// Set up context with signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Initialize components
	resolver := NewCgroupResolver(*nodeName, *kubeletURL, *cgroupRefresh)
	eventCh := make(chan EnrichedEvent, 1024)
	pm := NewProbeManager(resolver, eventCh, *pollInterval)

	loader := NewBPFLoader()
	if err := loader.Load(); err != nil {
		log.Printf("BPF loader failed (continuing without eBPF): %v", err)
	} else {
		log.Println("BPF programs loaded successfully")
	}

	// Start background goroutines
	var wg sync.WaitGroup

	// Cgroup resolver refresh loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := resolver.StartRefresh(ctx); err != nil && ctx.Err() == nil {
			log.Printf("CgroupResolver error: %v", err)
		}
	}()

	// Probe manager polling loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := pm.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("ProbeManager error: %v", err)
		}
	}()

	// Event forwarder — batches events and sends to server via HTTP POST
	wg.Add(1)
	go func() {
		defer wg.Done()
		forwardEvents(ctx, *serverURL, eventCh)
	}()

	// Wait for shutdown signal
	select {
	case sig := <-sigCh:
		log.Printf("Received signal %v, shutting down...", sig)
	case <-ctx.Done():
	}

	// Graceful shutdown with 5-second timeout
	cancel()

	shutdownDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		log.Println("All goroutines stopped")
	case <-time.After(5 * time.Second):
		log.Println("Shutdown timeout exceeded, forcing exit")
	}

	// Clean up BPF resources
	if err := loader.Close(); err != nil {
		log.Printf("BPF loader cleanup error: %v", err)
	}

	log.Println("Earthworm agent stopped")
}

// forwardEvents batches enriched events and sends them to the server.
func forwardEvents(ctx context.Context, serverURL string, eventCh <-chan EnrichedEvent) {
	client := &http.Client{Timeout: 10 * time.Second}
	batchSize := 100
	flushInterval := 1 * time.Second

	var batch []EnrichedEvent
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Flush remaining events
			if len(batch) > 0 {
				sendBatch(client, serverURL, batch)
			}
			return

		case evt := <-eventCh:
			batch = append(batch, evt)
			if len(batch) >= batchSize {
				sendBatch(client, serverURL, batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				sendBatch(client, serverURL, batch)
				batch = batch[:0]
			}
		}
	}
}

// sendBatch sends a batch of events to the server via HTTP POST.
func sendBatch(client *http.Client, serverURL string, events []EnrichedEvent) {
	url := fmt.Sprintf("%s/api/ebpf/events", serverURL)

	data, err := json.Marshal(events)
	if err != nil {
		log.Printf("Failed to marshal events: %v", err)
		return
	}

	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("Failed to send events to server: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("Server rejected events (status %d)", resp.StatusCode)
	}
}
