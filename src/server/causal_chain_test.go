package main

import (
	"context"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Feature: ebpf-kernel-observability, Property 7: Causal chain invariants
// — events within 120s window, chronologically ordered, non-empty summary
// **Validates: Requirements 6.1, 6.2, 6.3**
func TestProperty7_CausalChainInvariants(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		memStore := NewMemoryStore()
		ctx := context.Background()
		testHub := NewHub()
		go testHub.Run()

		builder := NewCausalChainBuilder(memStore, testHub)

		nodeName := rapid.StringMatching(`node-[a-z0-9]{3,6}`).Draw(t, "nodeName")
		transitionTime := time.Date(2025, 6, 15, 12, 5, 0, 0, time.UTC)

		// Generate events: some within the 120s window, some outside
		numEvents := rapid.IntRange(0, 20).Draw(t, "numEvents")
		for i := 0; i < numEvents; i++ {
			// Events within [-180s, +10s] of transition time (some outside window)
			offsetSec := rapid.IntRange(-180, 10).Draw(t, "offsetSec")
			ts := transitionTime.Add(time.Duration(offsetSec) * time.Second)

			e := EnrichedEvent{
				Timestamp: ts,
				PID:       rapid.Uint32Range(1, 65535).Draw(t, "pid"),
				Comm:      rapid.SampledFrom([]string{"kubelet", "containerd"}).Draw(t, "comm"),
				EventType: rapid.SampledFrom([]string{"syscall", "process", "network"}).Draw(t, "eventType"),
				NodeName:  nodeName,
			}
			memStore.SaveKernelEvent(ctx, e)
		}

		// Also store events for a different node (should not appear in chain)
		otherNode := nodeName + "-other"
		for i := 0; i < 3; i++ {
			e := EnrichedEvent{
				Timestamp: transitionTime.Add(-30 * time.Second),
				PID:       uint32(90000 + i),
				Comm:      "kubelet",
				EventType: "syscall",
				NodeName:  otherNode,
			}
			memStore.SaveKernelEvent(ctx, e)
		}

		chain, err := builder.OnNotReady(nodeName, transitionTime)
		if err != nil {
			t.Fatalf("OnNotReady failed: %v", err)
		}

		// Invariant 1: All events in the chain must have timestamps within [T-120s, T]
		windowStart := transitionTime.Add(-120 * time.Second)
		for i, e := range chain.Events {
			if e.Timestamp.Before(windowStart) {
				t.Fatalf("event %d timestamp %v is before window start %v", i, e.Timestamp, windowStart)
			}
			if e.Timestamp.After(transitionTime) {
				t.Fatalf("event %d timestamp %v is after transition time %v", i, e.Timestamp, transitionTime)
			}
		}

		// Invariant 2: Events must be chronologically ordered
		for i := 1; i < len(chain.Events); i++ {
			if chain.Events[i].Timestamp.Before(chain.Events[i-1].Timestamp) {
				t.Fatalf("events not chronologically ordered at index %d: %v > %v",
					i, chain.Events[i-1].Timestamp, chain.Events[i].Timestamp)
			}
		}

		// Invariant 3: Summary must be non-empty
		if chain.Summary == "" {
			t.Fatal("chain summary is empty")
		}

		// Invariant 4: NodeName must match
		if chain.NodeName != nodeName {
			t.Fatalf("chain NodeName: got %q, want %q", chain.NodeName, nodeName)
		}

		// Invariant 5: All events must belong to the correct node
		for i, e := range chain.Events {
			if e.NodeName != nodeName {
				t.Fatalf("event %d NodeName %q does not match chain node %q", i, e.NodeName, nodeName)
			}
		}

		// Invariant 6: If no events, root cause should be "unknown_cause"
		if len(chain.Events) == 0 && chain.RootCause != "unknown_cause" {
			t.Fatalf("empty chain should have rootCause=unknown_cause, got %q", chain.RootCause)
		}

		// Invariant 7: Chain should be stored
		chains, err := memStore.GetCausalChains(ctx, nodeName, transitionTime.Add(-1*time.Second), transitionTime.Add(1*time.Second))
		if err != nil {
			t.Fatalf("GetCausalChains failed: %v", err)
		}
		if len(chains) != 1 {
			t.Fatalf("expected 1 stored chain, got %d", len(chains))
		}
	})
}
