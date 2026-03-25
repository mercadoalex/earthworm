package main

import (
	"testing"

	"pgregory.net/rapid"
)

// genPodIdentity generates a random PodIdentity with all fields populated.
func genPodIdentity(t *rapid.T) PodIdentity {
	return PodIdentity{
		PodName:       rapid.StringMatching(`[a-z0-9\-]{5,30}`).Draw(t, "podName"),
		Namespace:     rapid.StringMatching(`[a-z0-9\-]{3,20}`).Draw(t, "namespace"),
		ContainerName: rapid.StringMatching(`[a-z0-9\-]{3,20}`).Draw(t, "containerName"),
		NodeName:      rapid.StringMatching(`node\-[a-z0-9]{3,10}`).Draw(t, "nodeName"),
	}
}

// TestCgroupEnrichmentCompleteness tests Property 5: Cgroup enrichment completeness.
// **Validates: Requirements 5.1, 5.3**
// Feature: ebpf-kernel-observability, Property 5: Cgroup enrichment completeness —
// known cgroup IDs produce complete pod identity.
func TestCgroupEnrichmentCompleteness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		resolver := NewCgroupResolver("test-node", "", 0)

		// Generate a random cgroup ID and pod identity
		cgroupID := rapid.Uint64Min(1).Draw(t, "cgroupID")
		pod := genPodIdentity(t)

		// Add the mapping to the cache
		resolver.UpdateCache(cgroupID, pod)

		// Create an event with this cgroup ID
		evt := &KernelEvent{
			Timestamp: rapid.Uint64Min(1).Draw(t, "timestamp"),
			PID:       rapid.Uint32Min(1).Draw(t, "pid"),
			PPID:      rapid.Uint32Min(1).Draw(t, "ppid"),
			TGID:      rapid.Uint32Min(1).Draw(t, "tgid"),
			CgroupID:  cgroupID,
			Comm:      genMonitoredComm(t),
			EventType: uint8(rapid.IntRange(0, 2).Draw(t, "eventType")),
		}

		// Enrich the event
		enriched := resolver.Enrich(evt)

		// Verify all pod identity fields are populated
		if enriched.PodName == "" {
			t.Error("enriched event has empty PodName for known cgroup")
		}
		if enriched.Namespace == "" {
			t.Error("enriched event has empty Namespace for known cgroup")
		}
		if enriched.ContainerName == "" {
			t.Error("enriched event has empty ContainerName for known cgroup")
		}
		if enriched.NodeName == "" {
			t.Error("enriched event has empty NodeName for known cgroup")
		}
		if enriched.HostLevel {
			t.Error("enriched event has HostLevel=true for known cgroup")
		}

		// Verify the pod identity matches what we put in
		if enriched.PodName != pod.PodName {
			t.Errorf("PodName mismatch: got %q, want %q", enriched.PodName, pod.PodName)
		}
		if enriched.Namespace != pod.Namespace {
			t.Errorf("Namespace mismatch: got %q, want %q", enriched.Namespace, pod.Namespace)
		}
		if enriched.ContainerName != pod.ContainerName {
			t.Errorf("ContainerName mismatch: got %q, want %q", enriched.ContainerName, pod.ContainerName)
		}
	})
}

// TestUnknownCgroupHostLevelLabeling tests Property 6: Unknown cgroup host-level labeling.
// **Validates: Requirements 5.4**
// Feature: ebpf-kernel-observability, Property 6: Unknown cgroup host-level labeling —
// unknown cgroup IDs produce hostLevel=true.
func TestUnknownCgroupHostLevelLabeling(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		resolver := NewCgroupResolver("test-node", "", 0)

		// Optionally populate the cache with some known cgroup IDs
		numKnown := rapid.IntRange(0, 5).Draw(t, "numKnown")
		knownIDs := make(map[uint64]bool)
		for i := 0; i < numKnown; i++ {
			id := rapid.Uint64Min(1).Draw(t, "knownCgroupID")
			knownIDs[id] = true
			resolver.UpdateCache(id, genPodIdentity(t))
		}

		// Generate an unknown cgroup ID (not in the known set)
		unknownID := rapid.Uint64Min(1).Draw(t, "unknownCgroupID")
		for knownIDs[unknownID] {
			unknownID = rapid.Uint64Min(1).Draw(t, "unknownCgroupIDRetry")
		}

		// Create an event with the unknown cgroup ID
		evt := &KernelEvent{
			Timestamp: rapid.Uint64Min(1).Draw(t, "timestamp"),
			PID:       rapid.Uint32Min(1).Draw(t, "pid"),
			PPID:      rapid.Uint32Min(1).Draw(t, "ppid"),
			TGID:      rapid.Uint32Min(1).Draw(t, "tgid"),
			CgroupID:  unknownID,
			Comm:      genMonitoredComm(t),
			EventType: uint8(rapid.IntRange(0, 2).Draw(t, "eventType")),
		}

		// Enrich the event
		enriched := resolver.Enrich(evt)

		// Verify host-level labeling
		if !enriched.HostLevel {
			t.Error("enriched event has HostLevel=false for unknown cgroup")
		}

		// Comm should be preserved as the workload identifier
		if enriched.Comm == "" {
			t.Error("enriched event has empty Comm for unknown cgroup")
		}
		if enriched.Comm != evt.CommString() {
			t.Errorf("Comm mismatch: got %q, want %q", enriched.Comm, evt.CommString())
		}

		// Pod identity fields should be empty for host-level events
		if enriched.PodName != "" {
			t.Errorf("PodName should be empty for unknown cgroup, got %q", enriched.PodName)
		}
		if enriched.Namespace != "" {
			t.Errorf("Namespace should be empty for unknown cgroup, got %q", enriched.Namespace)
		}
		if enriched.ContainerName != "" {
			t.Errorf("ContainerName should be empty for unknown cgroup, got %q", enriched.ContainerName)
		}
	})
}
