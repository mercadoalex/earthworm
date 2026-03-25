package main

import (
	"context"
	"time"
)

// Alert represents an anomaly alert for a node.
type Alert struct {
	NodeName     string          `json:"nodeName"`
	Namespace    string          `json:"namespace"`
	Gap          float64         `json:"gapSeconds"`
	Severity     string          `json:"severity"` // "warning" or "critical"
	Timestamp    time.Time       `json:"timestamp"`
	KernelEvents []EnrichedEvent `json:"kernelEvents,omitempty"`
}

// AnomalyDetector evaluates heartbeat gaps against thresholds.
type AnomalyDetector struct {
	store             Store
	warningThreshold  time.Duration
	criticalThreshold time.Duration
}

// NewAnomalyDetector creates a new detector with the given thresholds.
func NewAnomalyDetector(store Store, warningSeconds, criticalSeconds int) *AnomalyDetector {
	return &AnomalyDetector{
		store:             store,
		warningThreshold:  time.Duration(warningSeconds) * time.Second,
		criticalThreshold: time.Duration(criticalSeconds) * time.Second,
	}
}

// Evaluate checks the gap between the incoming event and the latest stored event for the same node.
// Returns an Alert if the gap exceeds a threshold, or nil if normal.
// When correlated kernel events exist in the preceding 120s window, they are included in the alert.
func (ad *AnomalyDetector) Evaluate(event Heartbeat) *Alert {
	latest, err := ad.store.GetLatestByNode(context.Background(), event.NodeName)
	if err != nil || latest == nil {
		return nil
	}

	gap := event.Timestamp.Sub(latest.Timestamp)
	if gap <= ad.warningThreshold {
		return nil
	}

	severity := "warning"
	if gap > ad.criticalThreshold {
		severity = "critical"
	}

	alert := &Alert{
		NodeName:  event.NodeName,
		Namespace: event.Namespace,
		Gap:       gap.Seconds(),
		Severity:  severity,
		Timestamp: event.Timestamp,
	}

	// Attach correlated kernel events from the preceding 120s window
	from := event.Timestamp.Add(-120 * time.Second)
	kernelEvents, err := ad.store.GetKernelEvents(context.Background(), event.NodeName, from, event.Timestamp)
	if err == nil && len(kernelEvents) > 0 {
		alert.KernelEvents = kernelEvents
	}

	return alert
}
