package main

import (
	"fmt"
	"time"

	"earthworm/src/kubernetes"
)

func main() {
	config := kubernetes.SimulationConfig{
		NodeCount: 50,
		Duration:  4 * time.Hour,
		Seed:      42,
		Scenarios: []kubernetes.ScenarioConfig{
			{
				Type:             "rolling_deployment",
				TriggerAt:        30 * time.Minute,
				NodeCount:        5,
				StaggerInterval:  30 * time.Second,
				ReplacementDelay: 15 * time.Second,
			},
			{
				Type:             "rolling_deployment",
				TriggerAt:        2 * time.Hour,
				NodeCount:        3,
				StaggerInterval:  30 * time.Second,
				ReplacementDelay: 15 * time.Second,
			},
		},
	}

	engine, err := kubernetes.NewSimulationEngine(config)
	if err != nil {
		fmt.Printf("Failed to create simulation engine: %v\n", err)
		return
	}

	fmt.Println("Running simulation: 50 nodes, 4h duration, seed=42...")
	result, err := engine.Run()
	if err != nil {
		fmt.Printf("Simulation failed: %v\n", err)
		return
	}

	fmt.Printf("Simulation complete: %d leases, %d eBPF events, %d lease files, %d ebpf files\n",
		result.Stats.TotalLeases, result.Stats.TotalEbpfEvents,
		len(result.LeaseFiles), len(result.EbpfFiles))

	outputDir := "src/heartbeat-visualizer/public/mocking_data"
	if err := kubernetes.WriteSimulationOutput(result, outputDir); err != nil {
		fmt.Printf("Failed to write output: %v\n", err)
		return
	}

	fmt.Printf("Output written to %s\n", outputDir)
	fmt.Printf("Lease manifest: %d files\n", len(result.Manifest))
	fmt.Printf("eBPF manifest: %d files\n", len(result.EbpfManifest))
}
