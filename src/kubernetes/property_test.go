package kubernetes

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"
)

// Feature: earthworm-improvements, Property 13: eBPF event correlation
// **Validates: Requirements 11.3**
func TestProperty13_EBPFEventCorrelation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a slice of PodInfo entries
		numPods := rapid.IntRange(0, 10).Draw(t, "numPods")
		podInfos := make([]PodInfo, numPods)
		for i := 0; i < numPods; i++ {
			numCgroups := rapid.IntRange(1, 3).Draw(t, fmt.Sprintf("numCgroups_%d", i))
			cgroupPaths := make([]string, numCgroups)
			for j := 0; j < numCgroups; j++ {
				seg := rapid.StringMatching(`[a-z0-9]{4,12}`).Draw(t, fmt.Sprintf("cgroup_%d_%d", i, j))
				cgroupPaths[j] = fmt.Sprintf("/sys/fs/cgroup/kubepods/%s", seg)
			}
			podInfos[i] = PodInfo{
				PodName:     rapid.StringMatching(`pod-[a-z0-9]{3,8}`).Draw(t, fmt.Sprintf("podName_%d", i)),
				Namespace:   rapid.StringMatching(`ns-[a-z]{2,6}`).Draw(t, fmt.Sprintf("namespace_%d", i)),
				NodeName:    rapid.StringMatching(`node-[0-9]{2}`).Draw(t, fmt.Sprintf("nodeName_%d", i)),
				CgroupPaths: cgroupPaths,
			}
		}

		// Decide whether to test a matching or non-matching path
		testMatch := rapid.Bool().Draw(t, "testMatch")

		if testMatch && numPods > 0 {
			// Pick a random pod and one of its cgroup paths
			podIdx := rapid.IntRange(0, numPods-1).Draw(t, "podIdx")
			cgIdx := rapid.IntRange(0, len(podInfos[podIdx].CgroupPaths)-1).Draw(t, "cgIdx")
			targetPath := podInfos[podIdx].CgroupPaths[cgIdx]

			result := CorrelateEBPFEvent(podInfos, targetPath)
			if result == nil {
				t.Fatalf("expected match for path %q, got nil", targetPath)
			}

			// The result should be the first pod that has this cgroup path
			found := false
			for _, pod := range podInfos {
				for _, cg := range pod.CgroupPaths {
					if cg == targetPath {
						if result.PodName != pod.PodName {
							t.Fatalf("expected first matching pod %q, got %q", pod.PodName, result.PodName)
						}
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		} else {
			// Use a path that doesn't exist in any pod
			nonExistentPath := "/sys/fs/cgroup/kubepods/nonexistent-" + rapid.StringMatching(`[a-z0-9]{8}`).Draw(t, "nonExistentSeg")
			result := CorrelateEBPFEvent(podInfos, nonExistentPath)
			if result != nil {
				t.Fatalf("expected nil for non-existent path %q, got pod %q", nonExistentPath, result.PodName)
			}
		}
	})
}

// Feature: earthworm-improvements, Property 14: Mock eBPF event generation count and fields
// **Validates: Requirements 11.5**
func TestProperty14_MockEBPFEventGenerationCountAndFields(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numNodes := rapid.IntRange(1, 20).Draw(t, "numNodes")
		nodes := make([]MockNode, numNodes)
		for i := 0; i < numNodes; i++ {
			nodes[i] = MockNode{
				Name:   rapid.StringMatching(`node-[a-z0-9]{2,8}`).Draw(t, fmt.Sprintf("nodeName_%d", i)),
				Status: "Ready",
			}
		}

		n := rapid.IntRange(1, 100).Draw(t, "numEvents")
		events := GenerateMockEBPFEvents(nodes, n)

		if len(events) != n {
			t.Fatalf("expected %d events, got %d", n, len(events))
		}

		for i, ev := range events {
			if ev.PID == 0 {
				t.Fatalf("event[%d]: PID is zero", i)
			}

			// Check Comm is non-empty (trim null bytes)
			comm := bytesToString(ev.Comm[:])
			if comm == "" {
				t.Fatalf("event[%d]: Comm is empty", i)
			}

			// Check CgroupPath is non-empty
			cgroupPath := bytesToString(ev.CgroupPath[:])
			if cgroupPath == "" {
				t.Fatalf("event[%d]: CgroupPath is empty", i)
			}

			if ev.Timestamp == 0 {
				t.Fatalf("event[%d]: Timestamp is zero", i)
			}
		}
	})
}
