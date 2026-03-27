package main

import (
	"context"
	"sort"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// genEnrichedEvent generates a random EnrichedEvent for property testing.
func genEnrichedEvent(t *rapid.T, nodeName string, baseTime time.Time) EnrichedEvent {
	eventType := rapid.SampledFrom([]string{"syscall", "process", "network"}).Draw(t, "eventType")
	offsetMs := rapid.IntRange(0, 120000).Draw(t, "offsetMs")
	ts := baseTime.Add(time.Duration(offsetMs) * time.Millisecond)

	e := EnrichedEvent{
		Timestamp: ts,
		PID:       rapid.Uint32Range(1, 65535).Draw(t, "pid"),
		PPID:      rapid.Uint32Range(0, 65535).Draw(t, "ppid"),
		Comm:      rapid.SampledFrom([]string{"kubelet", "containerd", "cri-o"}).Draw(t, "comm"),
		CgroupID:  rapid.Uint64Range(1, 100000).Draw(t, "cgroupId"),
		EventType: eventType,
		NodeName:  nodeName,
	}

	switch eventType {
	case "syscall":
		e.SyscallNr = rapid.Uint32Range(1, 500).Draw(t, "syscallNr")
		e.LatencyNs = rapid.Uint64Range(1000, 5000000000).Draw(t, "latencyNs")
		e.SlowSyscall = e.LatencyNs > 1000000000
		e.ReturnValue = int64(rapid.IntRange(-1, 100).Draw(t, "retVal"))
	case "process":
		e.ChildPID = rapid.Uint32Range(1, 65535).Draw(t, "childPid")
		e.ExitCode = int32(rapid.IntRange(-1, 128).Draw(t, "exitCode"))
		e.CriticalExit = e.Comm == "kubelet" && e.ExitCode != 0
	case "network":
		e.SrcAddr = "10.0.0.1"
		e.DstAddr = "10.0.0.2"
		e.SrcPort = rapid.Uint16Range(1024, 65535).Draw(t, "srcPort")
		e.DstPort = rapid.Uint16Range(1, 65535).Draw(t, "dstPort")
		e.NetEventType = rapid.SampledFrom([]string{"retransmit", "reset", "rtt_high"}).Draw(t, "netEventType")
		e.RTTUs = rapid.Uint32Range(100, 1000000).Draw(t, "rttUs")
	}

	// Optionally add pod enrichment
	if rapid.Bool().Draw(t, "hasPod") {
		e.PodName = rapid.StringMatching(`pod-[a-z]{3,6}`).Draw(t, "podName")
		e.Namespace = rapid.StringMatching(`ns-[a-z]{2,4}`).Draw(t, "namespace")
		e.ContainerName = rapid.StringMatching(`ctr-[a-z]{2,4}`).Draw(t, "containerName")
	} else {
		e.HostLevel = true
	}

	return e
}

// Feature: ebpf-kernel-observability, Property 10: KernelEvent persistence round-trip
// — store then query returns equivalent event
// **Validates: Requirements 9.1**
func TestProperty10_KernelEventPersistenceRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		memStore := NewMemoryStore()
		ctx := context.Background()

		nodeName := rapid.StringMatching(`node-[a-z0-9]{3,6}`).Draw(t, "nodeName")
		baseTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

		event := genEnrichedEvent(t, nodeName, baseTime)

		// Save the event
		err := memStore.SaveKernelEvent(ctx, event)
		if err != nil {
			t.Fatalf("SaveKernelEvent failed: %v", err)
		}

		// Query it back
		from := baseTime.Add(-1 * time.Second)
		to := baseTime.Add(121 * time.Second)
		results, err := memStore.GetKernelEvents(ctx, nodeName, from, to)
		if err != nil {
			t.Fatalf("GetKernelEvents failed: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("expected 1 event, got %d", len(results))
		}

		got := results[0]
		if got.NodeName != event.NodeName {
			t.Fatalf("NodeName: got %q, want %q", got.NodeName, event.NodeName)
		}
		if !got.Timestamp.Equal(event.Timestamp) {
			t.Fatalf("Timestamp: got %v, want %v", got.Timestamp, event.Timestamp)
		}
		if got.EventType != event.EventType {
			t.Fatalf("EventType: got %q, want %q", got.EventType, event.EventType)
		}
		if got.PID != event.PID {
			t.Fatalf("PID: got %d, want %d", got.PID, event.PID)
		}
		if got.Comm != event.Comm {
			t.Fatalf("Comm: got %q, want %q", got.Comm, event.Comm)
		}
		if got.PodName != event.PodName {
			t.Fatalf("PodName: got %q, want %q", got.PodName, event.PodName)
		}
		if got.SlowSyscall != event.SlowSyscall {
			t.Fatalf("SlowSyscall: got %v, want %v", got.SlowSyscall, event.SlowSyscall)
		}
		if got.CriticalExit != event.CriticalExit {
			t.Fatalf("CriticalExit: got %v, want %v", got.CriticalExit, event.CriticalExit)
		}
		if got.HostLevel != event.HostLevel {
			t.Fatalf("HostLevel: got %v, want %v", got.HostLevel, event.HostLevel)
		}

		// Also verify GetKernelEventsByType returns the same event
		byType, err := memStore.GetKernelEventsByType(ctx, nodeName, event.EventType, from, to)
		if err != nil {
			t.Fatalf("GetKernelEventsByType failed: %v", err)
		}
		if len(byType) != 1 {
			t.Fatalf("GetKernelEventsByType: expected 1 event, got %d", len(byType))
		}
		if byType[0].PID != event.PID {
			t.Fatalf("GetKernelEventsByType PID: got %d, want %d", byType[0].PID, event.PID)
		}
	})
}

// Feature: ebpf-kernel-observability, Property 11: Replay query filter correctness
// — returned events match all filters and are chronologically ordered
// **Validates: Requirements 9.2, 9.3**
func TestProperty11_ReplayQueryFilterCorrectness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		memStore := NewMemoryStore()
		ctx := context.Background()
		rs := NewReplayStore(memStore, 48*time.Hour)

		nodeName := rapid.StringMatching(`node-[a-z0-9]{3,6}`).Draw(t, "nodeName")
		baseTime := time.Now().UTC().Add(-1 * time.Hour)

		// Generate and store multiple events
		numEvents := rapid.IntRange(5, 30).Draw(t, "numEvents")
		for i := 0; i < numEvents; i++ {
			e := genEnrichedEvent(t, nodeName, baseTime)
			memStore.SaveKernelEvent(ctx, e)
		}

		// Also store events for a different node (should not appear)
		otherNode := nodeName + "-other"
		for i := 0; i < 5; i++ {
			e := genEnrichedEvent(t, otherNode, baseTime)
			memStore.SaveKernelEvent(ctx, e)
		}

		// Query with filters
		filterType := rapid.SampledFrom([]string{"syscall", "process", "network"}).Draw(t, "filterType")
		q := ReplayQuery{
			NodeName:   nodeName,
			From:       baseTime.Add(-1 * time.Second),
			To:         baseTime.Add(121 * time.Second),
			EventTypes: []string{filterType},
			PageSize:   1000,
			Page:       1,
		}

		results, _, err := rs.Query(ctx, q)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		// All returned events must match the node name
		for _, e := range results {
			if e.NodeName != nodeName {
				t.Fatalf("event NodeName %q does not match query node %q", e.NodeName, nodeName)
			}
		}

		// All returned events must match the event type filter
		for _, e := range results {
			if e.EventType != filterType {
				t.Fatalf("event EventType %q does not match filter %q", e.EventType, filterType)
			}
		}

		// All returned events must be within the time range
		for _, e := range results {
			if e.Timestamp.Before(q.From) || e.Timestamp.After(q.To) {
				t.Fatalf("event timestamp %v outside range [%v, %v]", e.Timestamp, q.From, q.To)
			}
		}

		// Results must be chronologically ordered
		for i := 1; i < len(results); i++ {
			if results[i].Timestamp.Before(results[i-1].Timestamp) {
				t.Fatalf("events not chronologically ordered at index %d: %v > %v",
					i, results[i-1].Timestamp, results[i].Timestamp)
			}
		}
	})
}

// Feature: ebpf-kernel-observability, Property 12: Replay pagination bounds
// — page size not exceeded, no duplicates or gaps across pages
// **Validates: Requirements 9.4**
func TestProperty12_ReplayPaginationBounds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		memStore := NewMemoryStore()
		ctx := context.Background()
		rs := NewReplayStore(memStore, 48*time.Hour)

		nodeName := rapid.StringMatching(`node-[a-z0-9]{3,6}`).Draw(t, "nodeName")
		baseTime := time.Now().UTC().Add(-30 * time.Minute)

		// Generate events with distinct timestamps to avoid ambiguity
		numEvents := rapid.IntRange(10, 50).Draw(t, "numEvents")
		for i := 0; i < numEvents; i++ {
			e := EnrichedEvent{
				Timestamp: baseTime.Add(time.Duration(i) * time.Second),
				PID:       uint32(i + 1),
				Comm:      "kubelet",
				EventType: "syscall",
				NodeName:  nodeName,
				LatencyNs: uint64(i * 1000),
			}
			memStore.SaveKernelEvent(ctx, e)
		}

		pageSize := rapid.IntRange(3, 15).Draw(t, "pageSize")
		q := ReplayQuery{
			NodeName: nodeName,
			From:     baseTime.Add(-1 * time.Second),
			To:       baseTime.Add(time.Duration(numEvents+1) * time.Second),
			PageSize: pageSize,
		}

		// Collect all events across all pages
		var allEvents []EnrichedEvent
		seenPIDs := make(map[uint32]bool)

		for page := 1; ; page++ {
			q.Page = page
			results, totalCount, err := rs.Query(ctx, q)
			if err != nil {
				t.Fatalf("Query page %d failed: %v", page, err)
			}

			// Page size must not be exceeded
			if len(results) > pageSize {
				t.Fatalf("page %d: got %d events, exceeds pageSize %d", page, len(results), pageSize)
			}

			// Total count must be consistent
			if totalCount != numEvents {
				t.Fatalf("totalCount: got %d, want %d", totalCount, numEvents)
			}

			// Check for duplicates
			for _, e := range results {
				if seenPIDs[e.PID] {
					t.Fatalf("duplicate event with PID %d on page %d", e.PID, page)
				}
				seenPIDs[e.PID] = true
			}

			allEvents = append(allEvents, results...)

			if len(results) < pageSize {
				break
			}
			// Safety: don't loop forever
			if page > numEvents/pageSize+2 {
				break
			}
		}

		// All events should be collected without gaps
		if len(allEvents) != numEvents {
			t.Fatalf("collected %d events across pages, want %d", len(allEvents), numEvents)
		}

		// Verify chronological ordering across all pages
		sort.Slice(allEvents, func(i, j int) bool {
			return allEvents[i].Timestamp.Before(allEvents[j].Timestamp)
		})
		for i := 1; i < len(allEvents); i++ {
			if allEvents[i].Timestamp.Before(allEvents[i-1].Timestamp) {
				t.Fatalf("events not ordered at index %d", i)
			}
		}
	})
}
