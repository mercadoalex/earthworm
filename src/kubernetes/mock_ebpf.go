package kubernetes

import (
	"fmt"
	"math/rand"
	"time"
)

// MockEBPFEvent simulates an eBPF event from the kernel
type MockEBPFEvent struct {
	PID        uint32
	PPID       uint32
	Comm       [16]byte
	CgroupPath [64]byte
	Timestamp  uint64
}

// MockNode represents a simulated Kubernetes node
type MockNode struct {
	Name      string
	LastLease time.Time
	Status    string
}

// GenerateMockNodes creates a slice of 50 mock nodes
func GenerateMockNodes() []MockNode {
	nodes := make([]MockNode, 50)
	for i := 0; i < 50; i++ {
		nodes[i] = MockNode{
			Name:      fmt.Sprintf("node-%02d", i+1),
			LastLease: time.Now().Add(-time.Duration(rand.Intn(10)) * time.Second),
			Status:    "Ready",
		}
	}
	return nodes
}

// GenerateMockEBPFEvents creates mock eBPF events for the given nodes
func GenerateMockEBPFEvents(nodes []MockNode, numEvents int) []MockEBPFEvent {
	events := make([]MockEBPFEvent, numEvents)
	for i := 0; i < numEvents; i++ {
		node := nodes[rand.Intn(len(nodes))]
		comm := [16]byte{}
		copy(comm[:], "kubelet")
		cgroup := [64]byte{}
		cgStr := fmt.Sprintf("/sys/fs/cgroup/kubepods/%s", node.Name)
		copy(cgroup[:], cgStr)
		events[i] = MockEBPFEvent{
			PID:        uint32(rand.Intn(5000) + 100),
			PPID:       uint32(rand.Intn(5000) + 100),
			Comm:       comm,
			CgroupPath: cgroup,
			Timestamp:  uint64(time.Now().Add(-time.Duration(rand.Intn(10)) * time.Second).UnixNano()),
		}
	}
	return events
}

// SimulateEBPFActivity prints mock eBPF events and demonstrates correlation logic
func SimulateEBPFActivity(nodes []MockNode, podInfos []PodInfo) {
	events := GenerateMockEBPFEvents(nodes, 100)
	for _, event := range events {
		fmt.Printf("Simulated eBPF event: PID=%d, PPID=%d, Comm=%s, cgroup=%s, Time=%v\n",
			event.PID, event.PPID, string(event.Comm[:]), string(event.CgroupPath[:]), time.Unix(0, int64(event.Timestamp)))
		pod := CorrelateEBPFEvent(podInfos, string(event.CgroupPath[:]))
		if pod != nil {
			fmt.Printf("→ Correlated to pod: %s (namespace: %s, node: %s)\n", pod.PodName, pod.Namespace, pod.NodeName)
		} else {
			fmt.Println("→ No matching pod found for eBPF event")
		}
	}
}
