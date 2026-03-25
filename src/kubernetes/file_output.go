package kubernetes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// EbpfEventJSON is the JSON-serializable format for eBPF events matching the visualizer's expected format.
type EbpfEventJSON struct {
	Timestamp int64  `json:"timestamp"`
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	PID       uint32 `json:"pid"`
	Comm      string `json:"comm"`
	Syscall   string `json:"syscall"`
}

// WriteSimulationOutput writes all simulation result files to the given output directory.
// It creates lease JSON files, eBPF JSON files, and manifest files.
func WriteSimulationOutput(result *SimulationResult, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write lease files
	for _, lf := range result.LeaseFiles {
		path := filepath.Join(outputDir, lf.Filename)
		data, err := json.MarshalIndent(lf.Data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal lease file %s: %w", lf.Filename, err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("failed to write lease file %s: %w", lf.Filename, err)
		}
	}

	// Write eBPF files
	for _, ef := range result.EbpfFiles {
		path := filepath.Join(outputDir, ef.Filename)
		// Convert to visualizer-compatible JSON format
		events := make([]EbpfEventJSON, len(ef.Events))
		for i, evt := range ef.Events {
			events[i] = EbpfEventJSON{
				Timestamp: evt.Timestamp.UnixNano() / 1e6, // ms
				Namespace: evt.Namespace,
				Pod:       evt.NodeName,
				PID:       evt.PID,
				Comm:      evt.Comm,
				Syscall:   evt.Syscall,
			}
		}
		data, err := json.MarshalIndent(events, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal ebpf file %s: %w", ef.Filename, err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("failed to write ebpf file %s: %w", ef.Filename, err)
		}
	}

	// Write lease manifest
	manifestPath := filepath.Join(outputDir, "leases.manifest.json")
	manifestData, err := json.MarshalIndent(result.Manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lease manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return fmt.Errorf("failed to write lease manifest: %w", err)
	}

	// Write eBPF manifest
	ebpfManifestPath := filepath.Join(outputDir, "ebpf-leases.manifest.json")
	ebpfManifestData, err := json.MarshalIndent(result.EbpfManifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ebpf manifest: %w", err)
	}
	if err := os.WriteFile(ebpfManifestPath, ebpfManifestData, 0644); err != nil {
		return fmt.Errorf("failed to write ebpf manifest: %w", err)
	}

	return nil
}
