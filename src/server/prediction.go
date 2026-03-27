package main

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"sync"
	"time"
)

// Prediction represents a predictive failure alert.
type Prediction struct {
	NodeName      string    `json:"nodeName"`
	Confidence    float64   `json:"confidence"`    // 0.0 to 1.0
	TimeToFailure float64   `json:"ttfSeconds"`    // predicted seconds until NotReady
	Timestamp     time.Time `json:"timestamp"`
	Patterns      []string  `json:"patterns"`      // detected pattern names
	Outcome       string    `json:"outcome"`       // "pending", "true_positive", "false_positive"
}

// AccuracyMetrics holds prediction accuracy statistics.
type AccuracyMetrics struct {
	TotalPredictions int     `json:"totalPredictions"`
	TruePositives    int     `json:"truePositives"`
	FalsePositives   int     `json:"falsePositives"`
	TrueNegatives    int     `json:"trueNegatives"`
	FalseNegatives   int     `json:"falseNegatives"`
	TruePositiveRate float64 `json:"truePositiveRate"`
	FalsePositiveRate float64 `json:"falsePositiveRate"`
}

// PredictionEngine analyzes kernel event patterns for failure prediction.
type PredictionEngine struct {
	store       Store
	hub         *Hub
	windowSize  time.Duration
	predictions []Prediction
	mu          sync.Mutex
}

// NewPredictionEngine creates a new prediction engine.
func NewPredictionEngine(store Store, hub *Hub) *PredictionEngine {
	return &PredictionEngine{
		store:      store,
		hub:        hub,
		windowSize: 5 * time.Minute,
	}
}

// Analyze evaluates the latest event window for a node and returns a prediction
// if failure patterns are detected.
func (pe *PredictionEngine) Analyze(nodeName string, events []EnrichedEvent) *Prediction {
	if len(events) == 0 {
		return nil
	}

	var patterns []string
	var confidence float64

	// Pattern 1: Increasing syscall latency trend
	if score := detectSyscallLatencyTrend(events); score > 0 {
		patterns = append(patterns, "syscall_latency_trend")
		confidence += score
	}

	// Pattern 2: TCP retransmit spikes
	if score := detectRetransmitSpike(events); score > 0 {
		patterns = append(patterns, "retransmit_spike")
		confidence += score
	}

	// Pattern 3: Critical process exits
	if score := detectCriticalExits(events); score > 0 {
		patterns = append(patterns, "critical_exit")
		confidence += score
	}

	// Pattern 4: High RTT events
	if score := detectHighRTT(events); score > 0 {
		patterns = append(patterns, "high_rtt")
		confidence += score
	}

	// Pattern 5: Filesystem I/O degradation
	if score := detectFilesystemIODegradation(events); score > 0 {
		patterns = append(patterns, "filesystem_io_degradation")
		confidence += score
	}

	// Pattern 6: Memory pressure escalation
	if score := detectMemoryPressureEscalation(events); score > 0 {
		patterns = append(patterns, "memory_pressure_escalation")
		confidence += score
	}

	// Pattern 7: DNS resolution degradation
	if score := detectDNSResolutionDegradation(events); score > 0 {
		patterns = append(patterns, "dns_resolution_degradation")
		confidence += score
	}

	if len(patterns) == 0 {
		return nil
	}

	// Enforce minimum confidence of 0.7 when ≥3 distinct patterns fire
	if len(patterns) >= 3 {
		confidence = math.Max(confidence, 0.7)
	}

	// Clamp confidence to [0.0, 1.0]
	confidence = math.Min(confidence, 1.0)
	confidence = math.Max(confidence, 0.0)

	// Estimate time to failure based on pattern severity
	ttf := estimateTTF(confidence)

	pred := &Prediction{
		NodeName:      nodeName,
		Confidence:    confidence,
		TimeToFailure: ttf,
		Timestamp:     time.Now().UTC(),
		Patterns:      patterns,
		Outcome:       "pending",
	}

	pe.mu.Lock()
	pe.predictions = append(pe.predictions, *pred)
	pe.mu.Unlock()

	if pe.hub != nil {
		pe.hub.BroadcastPrediction(*pred)
	}

	return pred
}

// detectSyscallLatencyTrend checks for increasing syscall latencies.
func detectSyscallLatencyTrend(events []EnrichedEvent) float64 {
	var latencies []uint64
	for _, e := range events {
		if e.EventType == "syscall" && e.LatencyNs > 0 {
			latencies = append(latencies, e.LatencyNs)
		}
	}
	if len(latencies) < 3 {
		return 0
	}

	// Check if latencies are generally increasing
	increasing := 0
	for i := 1; i < len(latencies); i++ {
		if latencies[i] > latencies[i-1] {
			increasing++
		}
	}
	ratio := float64(increasing) / float64(len(latencies)-1)
	if ratio > 0.6 {
		return ratio * 0.4
	}
	return 0
}

// detectRetransmitSpike checks for TCP retransmit bursts.
func detectRetransmitSpike(events []EnrichedEvent) float64 {
	retransmits := 0
	for _, e := range events {
		if e.EventType == "network" && e.NetEventType == "retransmit" {
			retransmits++
		}
	}
	if retransmits >= 5 {
		return math.Min(float64(retransmits)/10.0, 0.5)
	}
	return 0
}

// detectCriticalExits checks for critical process exits.
func detectCriticalExits(events []EnrichedEvent) float64 {
	for _, e := range events {
		if e.CriticalExit {
			return 0.8
		}
	}
	return 0
}

// detectHighRTT checks for high round-trip time events.
func detectHighRTT(events []EnrichedEvent) float64 {
	highRTT := 0
	for _, e := range events {
		if e.EventType == "network" && e.NetEventType == "rtt_high" {
			highRTT++
		}
	}
	if highRTT >= 3 {
		return math.Min(float64(highRTT)/8.0, 0.4)
	}
	return 0
}

// estimateTTF estimates time-to-failure in seconds based on confidence.
// Higher confidence → shorter TTF.
func estimateTTF(confidence float64) float64 {
	if confidence <= 0 {
		return 60.0
	}
	// Scale from 60s (low confidence) to 10s (high confidence)
	ttf := 60.0 - (confidence * 50.0)
	if ttf < 1.0 {
		ttf = 1.0
	}
	return ttf
}

// RecordOutcome updates a prediction's outcome after observing the actual result.
func (pe *PredictionEngine) RecordOutcome(nodeName string, predictionTime time.Time, outcome string) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	for i := range pe.predictions {
		if pe.predictions[i].NodeName == nodeName &&
			pe.predictions[i].Timestamp.Equal(predictionTime) &&
			pe.predictions[i].Outcome == "pending" {
			pe.predictions[i].Outcome = outcome
			break
		}
	}
}

// Accuracy computes prediction accuracy metrics from recorded outcomes.
func (pe *PredictionEngine) Accuracy() AccuracyMetrics {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	return ComputeAccuracy(pe.predictions)
}

// ComputeAccuracy calculates accuracy metrics from a slice of predictions.
func ComputeAccuracy(predictions []Prediction) AccuracyMetrics {
	var tp, fp, tn, fn int
	for _, p := range predictions {
		switch p.Outcome {
		case "true_positive":
			tp++
		case "false_positive":
			fp++
		case "true_negative":
			tn++
		case "false_negative":
			fn++
		}
	}

	metrics := AccuracyMetrics{
		TotalPredictions: len(predictions),
		TruePositives:   tp,
		FalsePositives:   fp,
		TrueNegatives:   tn,
		FalseNegatives:   fn,
	}

	if tp+fn > 0 {
		metrics.TruePositiveRate = float64(tp) / float64(tp+fn)
	}
	if fp+tn > 0 {
		metrics.FalsePositiveRate = float64(fp) / float64(fp+tn)
	}

	return metrics
}

// AnalyzeFromStore fetches recent events from the store and runs analysis.
func (pe *PredictionEngine) AnalyzeFromStore(nodeName string) *Prediction {
	to := time.Now().UTC()
	from := to.Add(-pe.windowSize)
	events, err := pe.store.GetKernelEvents(context.Background(), nodeName, from, to)
	if err != nil {
		return nil
	}
	return pe.Analyze(nodeName, events)
}

// detectFilesystemIODegradation detects increasing VFS latencies.
// Returns a positive score when ≥3 filesystem_io events have monotonically increasing ioLatencyNs.
func detectFilesystemIODegradation(events []EnrichedEvent) float64 {
	var latencies []uint64
	for _, e := range events {
		if e.EventType == "filesystem_io" && e.IOLatencyNs > 0 {
			latencies = append(latencies, e.IOLatencyNs)
		}
	}
	if len(latencies) < 3 {
		return 0
	}

	// Check for monotonically increasing latencies
	increasing := 0
	for i := 1; i < len(latencies); i++ {
		if latencies[i] > latencies[i-1] {
			increasing++
		}
	}
	ratio := float64(increasing) / float64(len(latencies)-1)
	if ratio > 0.6 {
		return ratio * 0.4
	}
	return 0
}

// detectMemoryPressureEscalation detects OOM kills or sustained memory pressure.
// Returns a positive score when OOM kill events or sustained memoryPressure flags are present.
func detectMemoryPressureEscalation(events []EnrichedEvent) float64 {
	oomKills := 0
	pressureCount := 0
	for _, e := range events {
		if e.EventType == "memory_pressure" && e.OOMSubType == "oom_kill" {
			oomKills++
		}
		if e.EventType == "cgroup_resource" && e.MemoryPressure {
			pressureCount++
		}
	}
	if oomKills > 0 {
		return math.Min(0.8+float64(oomKills)*0.05, 1.0)
	}
	if pressureCount >= 2 {
		return math.Min(float64(pressureCount)*0.15, 0.6)
	}
	return 0
}

// detectDNSResolutionDegradation detects increasing DNS latencies or timeouts.
// Returns a positive score when ≥3 dns_resolution events have increasing latencies or any timedOut.
func detectDNSResolutionDegradation(events []EnrichedEvent) float64 {
	var latencies []uint64
	timedOutCount := 0
	for _, e := range events {
		if e.EventType == "dns_resolution" {
			if e.TimedOut {
				timedOutCount++
			}
			if e.DNSLatencyNs > 0 {
				latencies = append(latencies, e.DNSLatencyNs)
			}
		}
	}
	if timedOutCount > 0 {
		return math.Min(0.5+float64(timedOutCount)*0.1, 0.8)
	}
	if len(latencies) >= 3 {
		increasing := 0
		for i := 1; i < len(latencies); i++ {
			if latencies[i] > latencies[i-1] {
				increasing++
			}
		}
		ratio := float64(increasing) / float64(len(latencies)-1)
		if ratio > 0.6 {
			return ratio * 0.35
		}
	}
	return 0
}

// predictionAccuracyHandler serves GET /api/predictions/accuracy.
func predictionAccuracyHandler(pe *PredictionEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		metrics := pe.Accuracy()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	}
}
