package main

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Feature: ebpf-kernel-observability, Property 8: Prediction confidence bounds
// — confidence in [0.0, 1.0] and TTF positive
// **Validates: Requirements 7.2**
func TestProperty8_PredictionConfidenceBounds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		memStore := NewMemoryStore()
		testHub := NewHub()
		go testHub.Run()
		pe := NewPredictionEngine(memStore, testHub)

		nodeName := rapid.StringMatching(`node-[a-z0-9]{3,6}`).Draw(t, "nodeName")
		baseTime := time.Now().UTC()

		// Generate a mix of events that may or may not trigger patterns
		numEvents := rapid.IntRange(1, 30).Draw(t, "numEvents")
		var events []EnrichedEvent
		for i := 0; i < numEvents; i++ {
			eventType := rapid.SampledFrom([]string{"syscall", "process", "network"}).Draw(t, "eventType")
			e := EnrichedEvent{
				Timestamp: baseTime.Add(time.Duration(i) * time.Second),
				PID:       rapid.Uint32Range(1, 65535).Draw(t, "pid"),
				Comm:      rapid.SampledFrom([]string{"kubelet", "containerd"}).Draw(t, "comm"),
				EventType: eventType,
				NodeName:  nodeName,
			}

			switch eventType {
			case "syscall":
				e.LatencyNs = rapid.Uint64Range(1000, 5000000000).Draw(t, "latencyNs")
				e.SlowSyscall = e.LatencyNs > 1000000000
			case "process":
				e.ExitCode = int32(rapid.IntRange(-1, 128).Draw(t, "exitCode"))
				e.CriticalExit = e.Comm == "kubelet" && e.ExitCode != 0
			case "network":
				e.NetEventType = rapid.SampledFrom([]string{"retransmit", "reset", "rtt_high"}).Draw(t, "netEventType")
				e.RTTUs = rapid.Uint32Range(100, 1000000).Draw(t, "rttUs")
			}

			events = append(events, e)
		}

		pred := pe.Analyze(nodeName, events)

		// If a prediction was made, verify bounds
		if pred != nil {
			if pred.Confidence < 0.0 || pred.Confidence > 1.0 {
				t.Fatalf("confidence %f out of bounds [0.0, 1.0]", pred.Confidence)
			}
			if pred.TimeToFailure <= 0 {
				t.Fatalf("timeToFailure %f must be positive", pred.TimeToFailure)
			}
			if len(pred.Patterns) == 0 {
				t.Fatal("prediction has no patterns")
			}
			if pred.NodeName != nodeName {
				t.Fatalf("prediction NodeName: got %q, want %q", pred.NodeName, nodeName)
			}
			if pred.Outcome != "pending" {
				t.Fatalf("new prediction outcome should be 'pending', got %q", pred.Outcome)
			}
		}
	})
}

// Feature: ebpf-kernel-observability, Property 9: Prediction accuracy computation
// — TPR and FPR computed correctly from outcomes
// **Validates: Requirements 7.5**
func TestProperty9_PredictionAccuracyComputation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numPredictions := rapid.IntRange(1, 50).Draw(t, "numPredictions")
		var predictions []Prediction

		for i := 0; i < numPredictions; i++ {
			outcome := rapid.SampledFrom([]string{
				"true_positive", "false_positive", "true_negative", "false_negative", "pending",
			}).Draw(t, "outcome")

			predictions = append(predictions, Prediction{
				NodeName:      "node-test",
				Confidence:    0.5,
				TimeToFailure: 30.0,
				Timestamp:     time.Now().UTC(),
				Patterns:      []string{"test"},
				Outcome:       outcome,
			})
		}

		metrics := ComputeAccuracy(predictions)

		// Count expected values
		var expectedTP, expectedFP, expectedTN, expectedFN int
		for _, p := range predictions {
			switch p.Outcome {
			case "true_positive":
				expectedTP++
			case "false_positive":
				expectedFP++
			case "true_negative":
				expectedTN++
			case "false_negative":
				expectedFN++
			}
		}

		if metrics.TruePositives != expectedTP {
			t.Fatalf("TruePositives: got %d, want %d", metrics.TruePositives, expectedTP)
		}
		if metrics.FalsePositives != expectedFP {
			t.Fatalf("FalsePositives: got %d, want %d", metrics.FalsePositives, expectedFP)
		}
		if metrics.TrueNegatives != expectedTN {
			t.Fatalf("TrueNegatives: got %d, want %d", metrics.TrueNegatives, expectedTN)
		}
		if metrics.FalseNegatives != expectedFN {
			t.Fatalf("FalseNegatives: got %d, want %d", metrics.FalseNegatives, expectedFN)
		}

		// Verify TPR = TP / (TP + FN)
		if expectedTP+expectedFN > 0 {
			expectedTPR := float64(expectedTP) / float64(expectedTP+expectedFN)
			if metrics.TruePositiveRate != expectedTPR {
				t.Fatalf("TruePositiveRate: got %f, want %f", metrics.TruePositiveRate, expectedTPR)
			}
		} else {
			if metrics.TruePositiveRate != 0 {
				t.Fatalf("TruePositiveRate should be 0 when TP+FN=0, got %f", metrics.TruePositiveRate)
			}
		}

		// Verify FPR = FP / (FP + TN)
		if expectedFP+expectedTN > 0 {
			expectedFPR := float64(expectedFP) / float64(expectedFP+expectedTN)
			if metrics.FalsePositiveRate != expectedFPR {
				t.Fatalf("FalsePositiveRate: got %f, want %f", metrics.FalsePositiveRate, expectedFPR)
			}
		} else {
			if metrics.FalsePositiveRate != 0 {
				t.Fatalf("FalsePositiveRate should be 0 when FP+TN=0, got %f", metrics.FalsePositiveRate)
			}
		}

		// Both rates must be in [0.0, 1.0]
		if metrics.TruePositiveRate < 0 || metrics.TruePositiveRate > 1.0 {
			t.Fatalf("TruePositiveRate %f out of bounds [0.0, 1.0]", metrics.TruePositiveRate)
		}
		if metrics.FalsePositiveRate < 0 || metrics.FalsePositiveRate > 1.0 {
			t.Fatalf("FalsePositiveRate %f out of bounds [0.0, 1.0]", metrics.FalsePositiveRate)
		}
	})
}
