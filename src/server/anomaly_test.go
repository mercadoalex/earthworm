package main

import (
	"context"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Feature: ebpf-kernel-observability, Property 14: Enhanced alert with kernel events
// — alerts include correlated events from 120s window
// **Validates: Requirements 11.3**
func TestProperty14_EnhancedAlertWithKernelEvents(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		memStore := NewMemoryStore()
		ctx := context.Background()

		nodeName := rapid.StringMatching(`node-[a-z0-9]{3,6}`).Draw(t, "nodeName")
		namespace := rapid.StringMatching(`ns-[a-z]{2,4}`).Draw(t, "namespace")
		warningS := rapid.IntRange(5, 15).Draw(t, "warningS")
		criticalS := rapid.IntRange(warningS+1, 60).Draw(t, "criticalS")

		baseTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

		// Save a baseline heartbeat
		baseline := Heartbeat{
			NodeName:  nodeName,
			Namespace: namespace,
			Timestamp: baseTime,
			Status:    "Ready",
		}
		memStore.Save(ctx, baseline)

		// Generate kernel events in the 120s window before the alert
		alertTime := baseTime.Add(time.Duration(criticalS+10) * time.Second)
		numKernelEvents := rapid.IntRange(0, 15).Draw(t, "numKernelEvents")

		var expectedInWindow []EnrichedEvent
		for i := 0; i < numKernelEvents; i++ {
			// Some events within 120s window, some outside
			offsetSec := rapid.IntRange(-180, 0).Draw(t, "offsetSec")
			ts := alertTime.Add(time.Duration(offsetSec) * time.Second)

			e := EnrichedEvent{
				Timestamp: ts,
				PID:       rapid.Uint32Range(1, 65535).Draw(t, "pid"),
				Comm:      "kubelet",
				EventType: "syscall",
				NodeName:  nodeName,
			}
			memStore.SaveKernelEvent(ctx, e)

			// Track which events should be in the 120s window
			windowStart := alertTime.Add(-120 * time.Second)
			if !ts.Before(windowStart) && !ts.After(alertTime) {
				expectedInWindow = append(expectedInWindow, e)
			}
		}

		// Also store events for a different node (should not appear)
		for i := 0; i < 3; i++ {
			e := EnrichedEvent{
				Timestamp: alertTime.Add(-30 * time.Second),
				PID:       uint32(80000 + i),
				Comm:      "kubelet",
				EventType: "syscall",
				NodeName:  nodeName + "-other",
			}
			memStore.SaveKernelEvent(ctx, e)
		}

		det := NewAnomalyDetector(memStore, warningS, criticalS)

		incoming := Heartbeat{
			NodeName:  nodeName,
			Namespace: namespace,
			Timestamp: alertTime,
			Status:    "NotReady",
		}

		alert := det.Evaluate(incoming)

		// Alert should be generated (gap > critical threshold)
		if alert == nil {
			t.Fatal("expected alert, got nil")
		}

		// If there were kernel events in the window, they should be attached
		if len(expectedInWindow) > 0 {
			if len(alert.KernelEvents) == 0 {
				t.Fatal("expected kernel events in alert, got none")
			}

			// All attached events must be within the 120s window
			windowStart := alertTime.Add(-120 * time.Second)
			for i, e := range alert.KernelEvents {
				if e.Timestamp.Before(windowStart) {
					t.Fatalf("kernel event %d timestamp %v before window start %v", i, e.Timestamp, windowStart)
				}
				if e.Timestamp.After(alertTime) {
					t.Fatalf("kernel event %d timestamp %v after alert time %v", i, e.Timestamp, alertTime)
				}
			}

			// All attached events must match the alert's node name
			for i, e := range alert.KernelEvents {
				if e.NodeName != nodeName {
					t.Fatalf("kernel event %d NodeName %q does not match alert node %q", i, e.NodeName, nodeName)
				}
			}

			// Count should match expected
			if len(alert.KernelEvents) != len(expectedInWindow) {
				t.Fatalf("kernel events count: got %d, want %d", len(alert.KernelEvents), len(expectedInWindow))
			}
		}

		// Alert metadata should be correct
		if alert.NodeName != nodeName {
			t.Fatalf("alert NodeName: got %q, want %q", alert.NodeName, nodeName)
		}
		if alert.Severity != "critical" {
			t.Fatalf("alert Severity: got %q, want %q", alert.Severity, "critical")
		}
	})
}
