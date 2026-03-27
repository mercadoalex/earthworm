package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const defaultLookbackWindow = 120 * time.Second

// CausalChainBuilder correlates kernel events into causal chains.
type CausalChainBuilder struct {
	store      Store
	hub        *Hub
	windowSize time.Duration // 120 seconds lookback
}

// NewCausalChainBuilder creates a new builder with the default 120s lookback.
func NewCausalChainBuilder(store Store, hub *Hub) *CausalChainBuilder {
	return &CausalChainBuilder{
		store:      store,
		hub:        hub,
		windowSize: defaultLookbackWindow,
	}
}

// OnNotReady is called when a node transitions to NotReady.
// It queries kernel events from the preceding window, builds a causal chain,
// stores it, and broadcasts via WebSocket.
func (ccb *CausalChainBuilder) OnNotReady(nodeName string, transitionTime time.Time) (*CausalChain, error) {
	from := transitionTime.Add(-ccb.windowSize)
	to := transitionTime

	events, err := ccb.store.GetKernelEvents(context.Background(), nodeName, from, to)
	if err != nil {
		return nil, fmt.Errorf("query kernel events: %w", err)
	}

	chain := ccb.buildChain(nodeName, transitionTime, events)

	if err := ccb.store.SaveCausalChain(context.Background(), chain); err != nil {
		return nil, fmt.Errorf("save causal chain: %w", err)
	}

	if ccb.hub != nil {
		ccb.hub.BroadcastCausalChain(chain)
	}

	return &chain, nil
}

// buildChain constructs a CausalChain from the given events.
func (ccb *CausalChainBuilder) buildChain(nodeName string, transitionTime time.Time, events []EnrichedEvent) CausalChain {
	// Sort events chronologically
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	chain := CausalChain{
		NodeName:  nodeName,
		Timestamp: transitionTime,
		Events:    events,
	}

	if len(events) == 0 {
		chain.Summary = fmt.Sprintf("Node %s went NotReady at %s with no correlated kernel events found",
			nodeName, transitionTime.Format(time.RFC3339))
		chain.RootCause = "unknown_cause"
		return chain
	}

	chain.Summary = generateSummary(nodeName, transitionTime, events)
	chain.RootCause = detectRootCause(events)

	return chain
}

// generateSummary creates a human-readable summary of the causal chain.
func generateSummary(nodeName string, transitionTime time.Time, events []EnrichedEvent) string {
	var parts []string

	var slowSyscalls, criticalExits, netEvents int
	var fsIOEvents, memPressureEvents, dnsEvents, cgroupEvents, auditEvents int
	for _, e := range events {
		switch {
		case e.SlowSyscall:
			slowSyscalls++
		case e.CriticalExit:
			criticalExits++
		case e.EventType == "network":
			netEvents++
		}
		switch e.EventType {
		case "filesystem_io":
			fsIOEvents++
		case "memory_pressure":
			memPressureEvents++
		case "dns_resolution":
			dnsEvents++
		case "cgroup_resource":
			cgroupEvents++
		case "network_audit":
			auditEvents++
		}
	}

	parts = append(parts, fmt.Sprintf("Node %s went NotReady at %s.",
		nodeName, transitionTime.Format(time.RFC3339)))

	if slowSyscalls > 0 {
		parts = append(parts, fmt.Sprintf("%d slow syscall(s) detected", slowSyscalls))
	}
	if criticalExits > 0 {
		parts = append(parts, fmt.Sprintf("%d critical process exit(s)", criticalExits))
	}
	if netEvents > 0 {
		parts = append(parts, fmt.Sprintf("%d network event(s)", netEvents))
	}
	if fsIOEvents > 0 {
		parts = append(parts, fmt.Sprintf("%d filesystem I/O event(s)", fsIOEvents))
	}
	if memPressureEvents > 0 {
		parts = append(parts, fmt.Sprintf("%d memory pressure event(s)", memPressureEvents))
	}
	if dnsEvents > 0 {
		parts = append(parts, fmt.Sprintf("%d DNS resolution event(s)", dnsEvents))
	}
	if cgroupEvents > 0 {
		parts = append(parts, fmt.Sprintf("%d cgroup resource event(s)", cgroupEvents))
	}
	if auditEvents > 0 {
		parts = append(parts, fmt.Sprintf("%d network audit event(s)", auditEvents))
	}

	parts = append(parts, fmt.Sprintf("Total %d kernel events in 120s window", len(events)))

	return strings.Join(parts, ". ")
}

// detectRootCause identifies the most likely root cause from the events.
// Priority order: critical_exit > oom_kill > filesystem_io_bottleneck > dns_timeout > slow_syscall > network_degradation > unknown_cause
func detectRootCause(events []EnrichedEvent) string {
	for _, e := range events {
		if e.CriticalExit {
			return fmt.Sprintf("critical_exit: %s (pid=%d, exit_code=%d)", e.Comm, e.PID, e.ExitCode)
		}
	}
	for _, e := range events {
		if e.EventType == "memory_pressure" && e.OOMSubType == "oom_kill" {
			return fmt.Sprintf("oom_kill: %s (killed_pid=%d)", e.KilledComm, e.KilledPID)
		}
	}
	for _, e := range events {
		if e.EventType == "filesystem_io" && e.SlowIO {
			return fmt.Sprintf("filesystem_io_bottleneck: %s (latency=%dns)", e.FilePath, e.IOLatencyNs)
		}
	}
	for _, e := range events {
		if e.EventType == "dns_resolution" && e.TimedOut {
			return fmt.Sprintf("dns_timeout: %s", e.Domain)
		}
	}
	for _, e := range events {
		if e.SlowSyscall {
			return fmt.Sprintf("slow_syscall: %s (latency=%dns)", e.Comm, e.LatencyNs)
		}
	}
	for _, e := range events {
		if e.EventType == "network" && e.NetEventType == "rtt_high" {
			return fmt.Sprintf("network_degradation: high RTT %dus", e.RTTUs)
		}
	}
	return "unknown_cause"
}
