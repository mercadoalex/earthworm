package kubernetes

import (
	"testing"
)

// --- Task 13.2: Unit tests for CorrelateEBPFEvent ---
// Validates: Requirements 11.3

func TestUnit_CorrelateEBPFEvent_MatchingCgroupPath(t *testing.T) {
	podInfos := []PodInfo{
		{
			PodName:     "pod-alpha",
			Namespace:   "default",
			NodeName:    "node-01",
			CgroupPaths: []string{"/sys/fs/cgroup/kubepods/abc123"},
		},
		{
			PodName:     "pod-beta",
			Namespace:   "kube-system",
			NodeName:    "node-02",
			CgroupPaths: []string{"/sys/fs/cgroup/kubepods/def456"},
		},
	}

	result := CorrelateEBPFEvent(podInfos, "/sys/fs/cgroup/kubepods/abc123")
	if result == nil {
		t.Fatal("expected a match, got nil")
	}
	if result.PodName != "pod-alpha" {
		t.Fatalf("expected pod-alpha, got %s", result.PodName)
	}
}

func TestUnit_CorrelateEBPFEvent_NonMatchingCgroupPath(t *testing.T) {
	podInfos := []PodInfo{
		{
			PodName:     "pod-alpha",
			Namespace:   "default",
			NodeName:    "node-01",
			CgroupPaths: []string{"/sys/fs/cgroup/kubepods/abc123"},
		},
	}

	result := CorrelateEBPFEvent(podInfos, "/sys/fs/cgroup/kubepods/nonexistent")
	if result != nil {
		t.Fatalf("expected nil for non-matching path, got pod %s", result.PodName)
	}
}

func TestUnit_CorrelateEBPFEvent_EmptyPodInfos(t *testing.T) {
	result := CorrelateEBPFEvent([]PodInfo{}, "/sys/fs/cgroup/kubepods/abc123")
	if result != nil {
		t.Fatalf("expected nil for empty pod list, got pod %s", result.PodName)
	}
}

func TestUnit_CorrelateEBPFEvent_MultipleCgroupPaths(t *testing.T) {
	podInfos := []PodInfo{
		{
			PodName:     "pod-multi",
			Namespace:   "default",
			NodeName:    "node-01",
			CgroupPaths: []string{"/sys/fs/cgroup/kubepods/path1", "/sys/fs/cgroup/kubepods/path2"},
		},
	}

	result := CorrelateEBPFEvent(podInfos, "/sys/fs/cgroup/kubepods/path2")
	if result == nil {
		t.Fatal("expected a match on second cgroup path, got nil")
	}
	if result.PodName != "pod-multi" {
		t.Fatalf("expected pod-multi, got %s", result.PodName)
	}
}

// --- Task 13.4: Unit tests for GenerateMockNodes ---
// Validates: Requirements 11.4

func TestUnit_GenerateMockNodes_Returns50Nodes(t *testing.T) {
	nodes := GenerateMockNodes()
	if len(nodes) != 50 {
		t.Fatalf("expected 50 nodes, got %d", len(nodes))
	}
}

func TestUnit_GenerateMockNodes_ValidFields(t *testing.T) {
	nodes := GenerateMockNodes()
	for i, node := range nodes {
		if node.Name == "" {
			t.Fatalf("node[%d]: Name is empty", i)
		}
		if node.Status == "" {
			t.Fatalf("node[%d]: Status is empty", i)
		}
	}
}

// --- Task 13.5: Unit tests for GenerateMockEBPFEvents ---
// Validates: Requirements 11.5

func TestUnit_GenerateMockEBPFEvents_ReturnsCorrectCount(t *testing.T) {
	nodes := GenerateMockNodes()
	events := GenerateMockEBPFEvents(nodes, 25)
	if len(events) != 25 {
		t.Fatalf("expected 25 events, got %d", len(events))
	}
}

func TestUnit_GenerateMockEBPFEvents_FieldPopulation(t *testing.T) {
	nodes := GenerateMockNodes()
	events := GenerateMockEBPFEvents(nodes, 10)

	for i, ev := range events {
		if ev.PID == 0 {
			t.Fatalf("event[%d]: PID is zero", i)
		}

		comm := bytesToString(ev.Comm[:])
		if comm == "" {
			t.Fatalf("event[%d]: Comm is empty", i)
		}

		cgroupPath := bytesToString(ev.CgroupPath[:])
		if cgroupPath == "" {
			t.Fatalf("event[%d]: CgroupPath is empty", i)
		}

		if ev.Timestamp == 0 {
			t.Fatalf("event[%d]: Timestamp is zero", i)
		}
	}
}
