// handle_ebpf.go
// This file provides the handler function to correlate eBPF heartbeat events with Kubernetes pods/nodes.

package kubernetes

import (
	"fmt"
	"time"
)

// HeartbeatData represents the structure sent from eBPF (should match heartbeat_data in C)
type HeartbeatData struct {
	PID        uint32
	PPID       uint32
	Comm       string
	CgroupPath string
	Timestamp  uint64 // nanoseconds since boot
}

// handleEBPFHeartbeat correlates an eBPF event with Kubernetes pod info and prints the result.
// podInfos: slice of PodInfo built from Kubernetes API
// event: HeartbeatData received from eBPF
func handleEBPFHeartbeat(event HeartbeatData, podInfos []PodInfo) {
	pod := CorrelateEBPFEvent(podInfos, event.CgroupPath)
	ts := time.Unix(0, int64(event.Timestamp)) // convert ns to time.Time
	if pod != nil {
		fmt.Printf(
			"Heartbeat for pod: %s (namespace: %s, node: %s) at %v\n",
			pod.PodName, pod.Namespace, pod.NodeName, ts,
		)
	} else {
		fmt.Printf(
			"Unmatched heartbeat event: PID=%d, Comm=%s, cgroup=%s at %v\n",
			event.PID, event.Comm, event.CgroupPath, ts,
		)
	}
}
