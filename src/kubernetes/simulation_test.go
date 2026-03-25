package kubernetes

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// --- Task 1.4: Unit tests for config validation edge cases ---

func TestValidateConfig_ZeroNodes(t *testing.T) {
	cfg := SimulationConfig{
		NodeCount: 0,
		Duration:  1 * time.Hour,
	}
	err := ValidateConfig(&cfg)
	if err == nil {
		t.Fatal("expected error for zero nodes, got nil")
	}
}

func TestValidateConfig_NegativeNodes(t *testing.T) {
	cfg := SimulationConfig{
		NodeCount: -5,
		Duration:  1 * time.Hour,
	}
	err := ValidateConfig(&cfg)
	if err == nil {
		t.Fatal("expected error for negative nodes, got nil")
	}
}

func TestValidateConfig_ZeroDuration(t *testing.T) {
	cfg := SimulationConfig{
		NodeCount: 10,
		Duration:  0,
	}
	err := ValidateConfig(&cfg)
	if err == nil {
		t.Fatal("expected error for zero duration, got nil")
	}
}

func TestValidateConfig_DurationExceeds7Days(t *testing.T) {
	cfg := SimulationConfig{
		NodeCount: 10,
		Duration:  8 * 24 * time.Hour,
	}
	err := ValidateConfig(&cfg)
	if err == nil {
		t.Fatal("expected error for duration > 7 days, got nil")
	}
}

func TestValidateConfig_DurationExactly7Days(t *testing.T) {
	cfg := SimulationConfig{
		NodeCount: 10,
		Duration:  7 * 24 * time.Hour,
	}
	err := ValidateConfig(&cfg)
	if err != nil {
		t.Fatalf("expected no error for exactly 7 days, got: %v", err)
	}
}

func TestValidateConfig_RatiosNormalization(t *testing.T) {
	cfg := SimulationConfig{
		NodeCount: 10,
		Duration:  1 * time.Hour,
		NamespaceRatios: map[string]float64{
			"ns-a": 1.0,
			"ns-b": 1.0,
			"ns-c": 1.0,
		},
	}
	err := ValidateConfig(&cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Each ratio should be ~0.333
	for ns, ratio := range cfg.NamespaceRatios {
		if math.Abs(ratio-1.0/3.0) > 0.01 {
			t.Errorf("namespace %q ratio = %f, want ~0.333", ns, ratio)
		}
	}
}

func TestValidateConfig_RatiosSumToOneAlready(t *testing.T) {
	cfg := SimulationConfig{
		NodeCount: 10,
		Duration:  1 * time.Hour,
		NamespaceRatios: map[string]float64{
			"kube-system": 0.2,
			"production":  0.5,
			"staging":     0.3,
		},
	}
	err := ValidateConfig(&cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Ratios should remain unchanged (already sum to 1.0)
	if math.Abs(cfg.NamespaceRatios["kube-system"]-0.2) > 0.001 {
		t.Errorf("kube-system ratio = %f, want 0.2", cfg.NamespaceRatios["kube-system"])
	}
	if math.Abs(cfg.NamespaceRatios["production"]-0.5) > 0.001 {
		t.Errorf("production ratio = %f, want 0.5", cfg.NamespaceRatios["production"])
	}
	if math.Abs(cfg.NamespaceRatios["staging"]-0.3) > 0.001 {
		t.Errorf("staging ratio = %f, want 0.3", cfg.NamespaceRatios["staging"])
	}
}

func TestValidateConfig_DefaultsApplied(t *testing.T) {
	cfg := SimulationConfig{
		NodeCount: 10,
		Duration:  1 * time.Hour,
	}
	err := ValidateConfig(&cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseInterval != 10*time.Second {
		t.Errorf("BaseInterval = %v, want 10s", cfg.BaseInterval)
	}
	if cfg.JitterStdDev != 500*time.Millisecond {
		t.Errorf("JitterStdDev = %v, want 500ms", cfg.JitterStdDev)
	}
	if cfg.FileSegmentWindow != 5*time.Minute {
		t.Errorf("FileSegmentWindow = %v, want 5m", cfg.FileSegmentWindow)
	}
	if cfg.MaxDriftIncrease != 3*time.Second {
		t.Errorf("MaxDriftIncrease = %v, want 3s", cfg.MaxDriftIncrease)
	}
}

func TestNewSimulationEngine_InvalidConfig(t *testing.T) {
	_, err := NewSimulationEngine(SimulationConfig{
		NodeCount: 0,
		Duration:  1 * time.Hour,
	})
	if err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
}

func TestNewSimulationEngine_NodeDistribution(t *testing.T) {
	engine, err := NewSimulationEngine(SimulationConfig{
		NodeCount: 100,
		Duration:  1 * time.Hour,
		Seed:      42,
		NamespaceRatios: map[string]float64{
			"kube-system": 0.2,
			"production":  0.5,
			"staging":     0.3,
		},
		NamespaceProfiles: map[string]NodeHealthProfile{
			"kube-system": ProfileStable,
			"production":  ProfileNormal,
			"staging":     ProfileVolatile,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(engine.nodes) != 100 {
		t.Fatalf("expected 100 nodes, got %d", len(engine.nodes))
	}

	// Count nodes per namespace
	nsCounts := make(map[string]int)
	for _, n := range engine.nodes {
		nsCounts[n.Namespace]++
	}

	// Check distribution is within 5% of configured ratios
	for ns, ratio := range engine.config.NamespaceRatios {
		expected := ratio * 100
		actual := float64(nsCounts[ns])
		if math.Abs(actual-expected) > 5 {
			t.Errorf("namespace %q: got %d nodes, expected ~%.0f", ns, nsCounts[ns], expected)
		}
	}
}

func TestNewSimulationEngine_HealthProfileAssignment(t *testing.T) {
	engine, err := NewSimulationEngine(SimulationConfig{
		NodeCount: 50,
		Duration:  1 * time.Hour,
		Seed:      42,
		NamespaceRatios: map[string]float64{
			"kube-system": 0.2,
			"production":  0.5,
			"staging":     0.3,
		},
		NamespaceProfiles: map[string]NodeHealthProfile{
			"kube-system": ProfileStable,
			"production":  ProfileNormal,
			"staging":     ProfileVolatile,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, node := range engine.nodes {
		// Every node must have a valid profile
		switch node.Profile {
		case ProfileStable, ProfileNormal, ProfileDrifting, ProfileVolatile:
			// ok
		default:
			t.Errorf("node %q has invalid profile %q", node.Name, node.Profile)
		}

		// Profile must match namespace config
		expectedProfile := engine.config.NamespaceProfiles[node.Namespace]
		if node.Profile != expectedProfile {
			t.Errorf("node %q in namespace %q: profile = %q, want %q",
				node.Name, node.Namespace, node.Profile, expectedProfile)
		}

		// All nodes start Ready
		if node.Status != "Ready" {
			t.Errorf("node %q: initial status = %q, want Ready", node.Name, node.Status)
		}
	}
}

func TestNewSimulationEngine_AllNodesHaveUniqueNames(t *testing.T) {
	engine, err := NewSimulationEngine(SimulationConfig{
		NodeCount: 50,
		Duration:  1 * time.Hour,
		Seed:      42,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	seen := make(map[string]bool)
	for _, node := range engine.nodes {
		if seen[node.Name] {
			t.Errorf("duplicate node name: %q", node.Name)
		}
		seen[node.Name] = true
	}
}

// --- Property-Based Tests for Task 2 ---

// Feature: realistic-data-and-visualizations, Property 5: Stable nodes never go NotReady
// **Validates: Requirements 1.5**
func TestProperty5_StableNodesNeverGoNotReady(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(5, 50).Draw(t, "nodeCount")
		durationMin := rapid.IntRange(1, 30).Draw(t, "durationMin")
		seed := rapid.Int64().Draw(t, "seed")

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  time.Duration(durationMin) * time.Minute,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"stable-ns": 1.0,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"stable-ns": ProfileStable,
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		// Every node should be stable and should have continuous leases (no large gaps)
		for _, node := range engine.nodes {
			if node.Profile != ProfileStable {
				t.Fatalf("node %q has profile %q, expected stable", node.Name, node.Profile)
			}
			if node.Status != "Ready" {
				t.Fatalf("stable node %q ended in status %q, expected Ready", node.Name, node.Status)
			}
			// Check that no lease gap exceeds 2x base interval (which would indicate NotReady)
			for i := 1; i < len(node.LeaseHistory); i++ {
				gap := node.LeaseHistory[i].Timestamp.Sub(node.LeaseHistory[i-1].Timestamp)
				maxGap := 2 * engine.config.BaseInterval
				if gap > maxGap {
					t.Fatalf("stable node %q has lease gap of %v (max allowed %v) between lease %d and %d",
						node.Name, gap, maxGap, i-1, i)
				}
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 3: NotReady duration bounds
// **Validates: Requirements 1.3**
func TestProperty3_NotReadyDurationBounds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(10, 50).Draw(t, "nodeCount")
		seed := rapid.Int64().Draw(t, "seed")

		// Use volatile profile to increase chance of NotReady transitions
		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  30 * time.Minute,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"volatile-ns": 1.0,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"volatile-ns": ProfileVolatile,
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		// For each node, find NotReady gaps (lease gaps > 2x base interval)
		// and verify they are between 5s and 120s
		for _, node := range engine.nodes {
			for i := 1; i < len(node.LeaseHistory); i++ {
				gap := node.LeaseHistory[i].Timestamp.Sub(node.LeaseHistory[i-1].Timestamp)
				// A gap significantly larger than the base interval indicates a NotReady period
				// The actual NotReady duration is approximately the gap minus one base interval
				if gap > 2*engine.config.BaseInterval {
					// The NotReady duration is the gap minus the normal interval
					notReadyDuration := gap - engine.config.BaseInterval
					// Allow some tolerance for jitter
					minBound := 4 * time.Second  // 5s minus 1s tolerance
					maxBound := 131 * time.Second // 120s + base interval + jitter tolerance
					if notReadyDuration < minBound {
						t.Fatalf("node %q: NotReady duration %v is below minimum bound %v (gap=%v)",
							node.Name, notReadyDuration, minBound, gap)
					}
					if notReadyDuration > maxBound {
						t.Fatalf("node %q: NotReady duration %v exceeds maximum bound %v (gap=%v)",
							node.Name, notReadyDuration, maxBound, gap)
					}
				}
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 9: Monotonically increasing lease timestamps
// **Validates: Requirements 2.5**
func TestProperty9_MonotonicallyIncreasingLeaseTimestamps(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(5, 30).Draw(t, "nodeCount")
		durationMin := rapid.IntRange(1, 20).Draw(t, "durationMin")
		seed := rapid.Int64().Draw(t, "seed")

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  time.Duration(durationMin) * time.Minute,
			Seed:      seed,
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		for _, node := range engine.nodes {
			for i := 1; i < len(node.LeaseHistory); i++ {
				if !node.LeaseHistory[i].Timestamp.After(node.LeaseHistory[i-1].Timestamp) {
					t.Fatalf("node %q: lease timestamp %v at index %d is not after %v at index %d",
						node.Name,
						node.LeaseHistory[i].Timestamp, i,
						node.LeaseHistory[i-1].Timestamp, i-1)
				}
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 7: No leases during NotReady
// **Validates: Requirements 2.3**
func TestProperty7_NoLeasesDuringNotReady(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(10, 50).Draw(t, "nodeCount")
		seed := rapid.Int64().Draw(t, "seed")

		// Use volatile profile to get NotReady transitions
		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  30 * time.Minute,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"volatile-ns": 1.0,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"volatile-ns": ProfileVolatile,
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		// Track NotReady periods during simulation
		type notReadyPeriod struct {
			start time.Time
			end   time.Time
		}
		notReadyPeriods := make(map[string][]notReadyPeriod)

		startTime := engine.clock
		totalSeconds := int(engine.config.Duration.Seconds())

		for i := 0; i <= totalSeconds; i++ {
			elapsed := time.Duration(i) * time.Second
			engine.clock = startTime.Add(elapsed)

			// Before tick, record current state
			for _, node := range engine.nodes {
				wasReady := node.Status == "Ready"
				// Run health transitions
				_ = wasReady // just tracking
			}

			engine.applyHealthTransitions()

			// After health transitions, record NotReady periods
			for _, node := range engine.nodes {
				if node.Status == "NotReady" {
					periods := notReadyPeriods[node.Name]
					if len(periods) == 0 || periods[len(periods)-1].end != (time.Time{}) {
						// Start a new period
						notReadyPeriods[node.Name] = append(notReadyPeriods[node.Name], notReadyPeriod{
							start: engine.clock,
						})
					}
				} else {
					periods := notReadyPeriods[node.Name]
					if len(periods) > 0 && periods[len(periods)-1].end == (time.Time{}) {
						// Close the period
						notReadyPeriods[node.Name][len(periods)-1].end = engine.clock
					}
				}
			}

			// Generate leases
			for _, node := range engine.nodes {
				engine.generateLeaseForNode(node, elapsed)
			}
		}

		// Close any open NotReady periods
		for name, periods := range notReadyPeriods {
			if len(periods) > 0 && periods[len(periods)-1].end == (time.Time{}) {
				notReadyPeriods[name][len(periods)-1].end = startTime.Add(time.Duration(totalSeconds) * time.Second)
			}
		}

		// Verify: no lease timestamps fall within any NotReady period
		for _, node := range engine.nodes {
			periods := notReadyPeriods[node.Name]
			for _, lease := range node.LeaseHistory {
				for _, period := range periods {
					// Lease should not be strictly within the NotReady period
					// (it can be at the boundary when the node transitions back to Ready)
					if lease.Timestamp.After(period.start) && lease.Timestamp.Before(period.end) {
						t.Fatalf("node %q: lease at %v falls within NotReady period [%v, %v)",
							node.Name, lease.Timestamp, period.start, period.end)
					}
				}
			}
		}
	})
}

// --- Property-Based Tests for Task 3: Rolling Deployment Scenarios ---

// Feature: realistic-data-and-visualizations, Property 10: Rolling deployment stagger
// **Validates: Requirements 3.1**
func TestProperty10_RollingDeploymentStagger(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(10, 30).Draw(t, "nodeCount")
		drainCount := rapid.IntRange(2, 5).Draw(t, "drainCount")
		staggerSec := rapid.IntRange(10, 60).Draw(t, "staggerSec")
		seed := rapid.Int64().Draw(t, "seed")

		stagger := time.Duration(staggerSec) * time.Second
		// Duration must be long enough for all drains + some buffer
		minDuration := time.Duration(drainCount+1) * stagger
		duration := minDuration + 2*time.Minute

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  duration,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"default": 1.0,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"default": ProfileStable, // stable so only scenario causes drains
			},
			Scenarios: []ScenarioConfig{
				{
					Type:             "rolling_deployment",
					TriggerAt:        30 * time.Second,
					NodeCount:        drainCount,
					StaggerInterval:  stagger,
					ReplacementDelay: 15 * time.Second,
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		// Find drain times from active scenarios
		if len(engine.activeScenarios) == 0 {
			t.Fatal("no active scenarios found")
		}
		as := engine.activeScenarios[0]
		if len(as.drainedNodes) < 2 {
			t.Skipf("only %d nodes drained, need at least 2 to check stagger", len(as.drainedNodes))
		}

		// Check stagger intervals between consecutive drains
		for i := 1; i < len(as.drainedNodes); i++ {
			gap := as.drainedNodes[i].drainTime.Sub(as.drainedNodes[i-1].drainTime)
			tolerance := 5 * time.Second
			if gap < stagger-tolerance || gap > stagger+tolerance {
				t.Fatalf("drain stagger between node %d and %d: got %v, expected ~%v (±%v)",
					i-1, i, gap, stagger, tolerance)
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 11: Drain stops leases and replacement appears
// **Validates: Requirements 3.2**
func TestProperty11_DrainStopsLeasesAndReplacementAppears(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(10, 30).Draw(t, "nodeCount")
		drainCount := rapid.IntRange(1, 4).Draw(t, "drainCount")
		replacementDelaySec := rapid.IntRange(5, 30).Draw(t, "replacementDelaySec")
		seed := rapid.Int64().Draw(t, "seed")

		replacementDelay := time.Duration(replacementDelaySec) * time.Second
		stagger := 30 * time.Second
		minDuration := time.Duration(drainCount+1)*stagger + replacementDelay + time.Minute
		duration := minDuration + 2*time.Minute

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  duration,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"default": 1.0,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"default": ProfileStable,
			},
			Scenarios: []ScenarioConfig{
				{
					Type:             "rolling_deployment",
					TriggerAt:        30 * time.Second,
					NodeCount:        drainCount,
					StaggerInterval:  stagger,
					ReplacementDelay: replacementDelay,
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		as := engine.activeScenarios[0]
		for _, di := range as.drainedNodes {
			// Find the drained node and verify no leases after drain time
			for _, node := range engine.nodes {
				if node.Name == di.nodeName {
					for _, lease := range node.LeaseHistory {
						if lease.Timestamp.After(di.drainTime) {
							t.Fatalf("drained node %q has lease at %v after drain time %v",
								node.Name, lease.Timestamp, di.drainTime)
						}
					}
					break
				}
			}

			// Verify replacement node exists with a different name
			if !di.replaced {
				t.Fatalf("drained node %q was not replaced", di.nodeName)
			}
			if di.replacementName == di.nodeName {
				t.Fatalf("replacement node has same name as drained node: %q", di.nodeName)
			}

			// Verify replacement node exists in engine.nodes
			found := false
			for _, node := range engine.nodes {
				if node.Name == di.replacementName {
					found = true
					if len(node.LeaseHistory) == 0 {
						t.Fatalf("replacement node %q has no leases", di.replacementName)
					}
					break
				}
			}
			if !found {
				t.Fatalf("replacement node %q not found in engine nodes", di.replacementName)
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 12: Node count invariant during rolling deployment
// **Validates: Requirements 3.3**
func TestProperty12_NodeCountInvariantDuringRollingDeployment(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(10, 30).Draw(t, "nodeCount")
		drainCount := rapid.IntRange(1, 4).Draw(t, "drainCount")
		seed := rapid.Int64().Draw(t, "seed")

		stagger := 30 * time.Second
		replacementDelay := 15 * time.Second
		minDuration := time.Duration(drainCount+1)*stagger + replacementDelay + time.Minute
		duration := minDuration + 2*time.Minute

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  duration,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"default": 1.0,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"default": ProfileStable,
			},
			Scenarios: []ScenarioConfig{
				{
					Type:             "rolling_deployment",
					TriggerAt:        30 * time.Second,
					NodeCount:        drainCount,
					StaggerInterval:  stagger,
					ReplacementDelay: replacementDelay,
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		// For each second of the simulation, count nodes that have at least one
		// lease within a ±15s window (half of the 30s window specified in the property).
		totalSeconds := int(duration.Seconds())
		windowHalf := 15 * time.Second

		for sec := 0; sec <= totalSeconds; sec++ {
			checkTime := engine.startTime.Add(time.Duration(sec) * time.Second)
			activeCount := 0

			for _, node := range engine.nodes {
				hasLease := false
				for _, lease := range node.LeaseHistory {
					if lease.Timestamp.After(checkTime.Add(-windowHalf)) && lease.Timestamp.Before(checkTime.Add(windowHalf)) {
						hasLease = true
						break
					}
				}
				if hasLease {
					activeCount++
				}
			}

			if activeCount < nodeCount-2 || activeCount > nodeCount+2 {
				t.Fatalf("at t=%ds, active node count = %d, expected within [%d, %d]",
					sec, activeCount, nodeCount-2, nodeCount+2)
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 13: Replacement node initial burst
// **Validates: Requirements 3.4**
func TestProperty13_ReplacementNodeInitialBurst(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(10, 30).Draw(t, "nodeCount")
		drainCount := rapid.IntRange(1, 4).Draw(t, "drainCount")
		seed := rapid.Int64().Draw(t, "seed")

		stagger := 30 * time.Second
		replacementDelay := 15 * time.Second
		minDuration := time.Duration(drainCount+1)*stagger + replacementDelay + time.Minute
		duration := minDuration + 2*time.Minute

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  duration,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"default": 1.0,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"default": ProfileStable,
			},
			Scenarios: []ScenarioConfig{
				{
					Type:             "rolling_deployment",
					TriggerAt:        30 * time.Second,
					NodeCount:        drainCount,
					StaggerInterval:  stagger,
					ReplacementDelay: replacementDelay,
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		// For each replacement node, verify the first 3 leases are within 5s of the first lease
		as := engine.activeScenarios[0]
		for _, di := range as.drainedNodes {
			if !di.replaced {
				t.Fatalf("drained node %q was not replaced", di.nodeName)
			}

			// Find the replacement node
			for _, node := range engine.nodes {
				if node.Name == di.replacementName {
					if len(node.LeaseHistory) < 3 {
						t.Fatalf("replacement node %q has only %d leases, expected at least 3",
							node.Name, len(node.LeaseHistory))
					}

					firstLease := node.LeaseHistory[0].Timestamp
					burstWindow := 5 * time.Second

					for i := 0; i < 3; i++ {
						elapsed := node.LeaseHistory[i].Timestamp.Sub(firstLease)
						if elapsed > burstWindow {
							t.Fatalf("replacement node %q: lease %d at %v is %v after first lease, exceeds 5s burst window",
								node.Name, i, node.LeaseHistory[i].Timestamp, elapsed)
						}
					}
					break
				}
			}
		}
	})
}

// --- Property-Based Tests for Task 4: Correlated eBPF Event Generation ---

// Feature: realistic-data-and-visualizations, Property 16: Correlated eBPF for kubelet_restart
// **Validates: Requirements 5.1**
func TestProperty16_CorrelatedEbpfForKubeletRestart(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(10, 50).Draw(t, "nodeCount")
		seed := rapid.Int64().Draw(t, "seed")

		// Use volatile profile to increase chance of NotReady transitions
		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  30 * time.Minute,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"volatile-ns": 1.0,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"volatile-ns": ProfileVolatile,
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		// For each node that had a kubelet_restart transition, verify there's
		// a correlated eBPF event with syscall "exit" and comm "kubelet"
		// timestamped 0–2 seconds before the transition time.
		for _, node := range engine.nodes {
			for _, tr := range node.TransitionHistory {
				if tr.Cause != CauseKubeletRestart {
					continue
				}

				foundExit := false
				for _, evt := range node.EbpfEvents {
					if evt.Syscall != "exit" || evt.Comm != "kubelet" {
						continue
					}
					// Event should be 0–2s before the transition
					diff := tr.Timestamp.Sub(evt.Timestamp)
					if diff >= 0 && diff <= 2*time.Second {
						foundExit = true
						// Verify event fields
						if evt.PID == 0 {
							t.Fatalf("node %q: kubelet_restart eBPF event has PID=0", node.Name)
						}
						if evt.PPID == 0 {
							t.Fatalf("node %q: kubelet_restart eBPF event has PPID=0", node.Name)
						}
						if evt.NodeName != node.Name {
							t.Fatalf("node %q: eBPF event NodeName=%q mismatch", node.Name, evt.NodeName)
						}
						break
					}
				}
				if !foundExit {
					t.Fatalf("node %q: kubelet_restart at %v has no correlated eBPF exit event within 0-2s before",
						node.Name, tr.Timestamp)
				}
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 17: Correlated eBPF for oom_kill
// **Validates: Requirements 5.2**
func TestProperty17_CorrelatedEbpfForOomKill(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(10, 50).Draw(t, "nodeCount")
		seed := rapid.Int64().Draw(t, "seed")

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  30 * time.Minute,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"volatile-ns": 1.0,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"volatile-ns": ProfileVolatile,
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		// For each node that had an oom_kill transition, verify there's
		// a correlated eBPF event with comm "oom_reaper" and syscall "kill"
		// within 1 second of the transition time.
		for _, node := range engine.nodes {
			for _, tr := range node.TransitionHistory {
				if tr.Cause != CauseOOMKill {
					continue
				}

				foundKill := false
				for _, evt := range node.EbpfEvents {
					if evt.Syscall != "kill" || evt.Comm != "oom_reaper" {
						continue
					}
					diff := evt.Timestamp.Sub(tr.Timestamp)
					if diff >= -1*time.Second && diff <= 1*time.Second {
						foundKill = true
						// Verify event fields
						if evt.PID == 0 {
							t.Fatalf("node %q: oom_kill eBPF event has PID=0", node.Name)
						}
						if evt.PPID == 0 {
							t.Fatalf("node %q: oom_kill eBPF event has PPID=0", node.Name)
						}
						if evt.NodeName != node.Name {
							t.Fatalf("node %q: eBPF event NodeName=%q mismatch", node.Name, evt.NodeName)
						}
						break
					}
				}
				if !foundKill {
					t.Fatalf("node %q: oom_kill at %v has no correlated eBPF kill event within ±1s",
						node.Name, tr.Timestamp)
				}
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 18: Correlated eBPF for replacement node fork
// **Validates: Requirements 5.3**
func TestProperty18_CorrelatedEbpfForReplacementFork(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(10, 30).Draw(t, "nodeCount")
		drainCount := rapid.IntRange(1, 4).Draw(t, "drainCount")
		seed := rapid.Int64().Draw(t, "seed")

		stagger := 30 * time.Second
		replacementDelay := 15 * time.Second
		minDuration := time.Duration(drainCount+1)*stagger + replacementDelay + time.Minute
		duration := minDuration + 2*time.Minute

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  duration,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"default": 1.0,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"default": ProfileStable,
			},
			Scenarios: []ScenarioConfig{
				{
					Type:             "rolling_deployment",
					TriggerAt:        30 * time.Second,
					NodeCount:        drainCount,
					StaggerInterval:  stagger,
					ReplacementDelay: replacementDelay,
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		// For each replacement node, verify there's an eBPF fork event
		// within 1 second of the node's first lease renewal.
		as := engine.activeScenarios[0]
		for _, di := range as.drainedNodes {
			if !di.replaced {
				t.Fatalf("drained node %q was not replaced", di.nodeName)
			}

			// Find the replacement node
			var replNode *SimNode
			for _, node := range engine.nodes {
				if node.Name == di.replacementName {
					replNode = node
					break
				}
			}
			if replNode == nil {
				t.Fatalf("replacement node %q not found", di.replacementName)
			}
			if len(replNode.LeaseHistory) == 0 {
				t.Fatalf("replacement node %q has no leases", replNode.Name)
			}

			firstLease := replNode.LeaseHistory[0].Timestamp

			// Find a fork eBPF event within 1 second of the first lease
			foundFork := false
			for _, evt := range replNode.EbpfEvents {
				if evt.Syscall == "fork" && evt.Comm == "kubelet" {
					diff := evt.Timestamp.Sub(firstLease)
					if diff >= 0 && diff <= 1*time.Second {
						foundFork = true
						// Verify event fields
						if evt.PID == 0 {
							t.Fatalf("replacement node %q: fork eBPF event has PID=0", replNode.Name)
						}
						if evt.PPID == 0 {
							t.Fatalf("replacement node %q: fork eBPF event has PPID=0", replNode.Name)
						}
						if evt.NodeName != replNode.Name {
							t.Fatalf("replacement node %q: fork eBPF event NodeName=%q mismatch",
								replNode.Name, evt.NodeName)
						}
						break
					}
				}
			}
			if !foundFork {
				t.Fatalf("replacement node %q: no fork eBPF event within 1s of first lease at %v",
					replNode.Name, firstLease)
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 19: Background eBPF event count
// **Validates: Requirements 5.4**
func TestProperty19_BackgroundEbpfCountEqualsLeaseCount(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(5, 30).Draw(t, "nodeCount")
		durationMin := rapid.IntRange(1, 20).Draw(t, "durationMin")
		seed := rapid.Int64().Draw(t, "seed")

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  time.Duration(durationMin) * time.Minute,
			Seed:      seed,
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		// Count total leases and total "write" eBPF events across all nodes
		totalLeases := 0
		totalWriteEvents := 0
		for _, node := range engine.nodes {
			totalLeases += len(node.LeaseHistory)
			for _, evt := range node.EbpfEvents {
				if evt.Syscall == "write" {
					totalWriteEvents++
				}
			}
		}

		if totalWriteEvents != totalLeases {
			t.Fatalf("background eBPF write events (%d) != total leases (%d)",
				totalWriteEvents, totalLeases)
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 20: eBPF event field validity
// **Validates: Requirements 5.5**
func TestProperty20_EbpfEventFieldValidity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(5, 30).Draw(t, "nodeCount")
		durationMin := rapid.IntRange(1, 20).Draw(t, "durationMin")
		seed := rapid.Int64().Draw(t, "seed")

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  time.Duration(durationMin) * time.Minute,
			Seed:      seed,
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		for _, node := range engine.nodes {
			for i, evt := range node.EbpfEvents {
				if evt.PID == 0 {
					t.Fatalf("node %q: eBPF event %d has PID=0", node.Name, i)
				}
				if evt.PPID == 0 {
					t.Fatalf("node %q: eBPF event %d has PPID=0", node.Name, i)
				}
				if evt.Comm == "" {
					t.Fatalf("node %q: eBPF event %d has empty comm", node.Name, i)
				}
				if evt.CgroupPath == "" {
					t.Fatalf("node %q: eBPF event %d has empty cgroup_path", node.Name, i)
				}
				if evt.Timestamp.IsZero() {
					t.Fatalf("node %q: eBPF event %d has zero timestamp", node.Name, i)
				}
				if evt.Syscall == "" {
					t.Fatalf("node %q: eBPF event %d has empty syscall", node.Name, i)
				}
			}
		}
	})
}

// --- Property-Based Tests for Task 5: Multi-Namespace Profiles and Cluster-Level Properties ---

// Feature: realistic-data-and-visualizations, Property 14: Namespace count and distribution
// **Validates: Requirements 4.1, 4.4**
func TestProperty14_NamespaceCountAndDistribution(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(20, 100).Draw(t, "nodeCount")
		seed := rapid.Int64().Draw(t, "seed")

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  10 * time.Minute,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"kube-system": 0.2,
				"production":  0.5,
				"staging":     0.3,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"kube-system": ProfileStable,
				"production":  ProfileNormal,
				"staging":     ProfileVolatile,
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		// Verify at least 3 namespaces
		nsCounts := make(map[string]int)
		for _, node := range engine.nodes {
			nsCounts[node.Namespace]++
		}
		if len(nsCounts) < 3 {
			t.Fatalf("expected at least 3 namespaces, got %d: %v", len(nsCounts), nsCounts)
		}

		// Verify distribution is within 5 percentage points of configured ratios
		for ns, ratio := range engine.config.NamespaceRatios {
			expectedPct := ratio * 100
			actualPct := float64(nsCounts[ns]) / float64(nodeCount) * 100
			if math.Abs(actualPct-expectedPct) > 5.0 {
				t.Fatalf("namespace %q: distribution %.1f%% deviates more than 5pp from expected %.1f%%",
					ns, actualPct, expectedPct)
			}
		}

		// Verify total node count
		total := 0
		for _, c := range nsCounts {
			total += c
		}
		if total != nodeCount {
			t.Fatalf("total nodes %d != configured %d", total, nodeCount)
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 15: Namespace NotReady rate matches profile
// **Validates: Requirements 4.2, 4.3**
func TestProperty15_NamespaceNotReadyRateMatchesProfile(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		seed := rapid.Int64().Draw(t, "seed")

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: 100,
			Duration:  1 * time.Hour,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"kube-system": 0.2,
				"production":  0.5,
				"staging":     0.3,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"kube-system": ProfileStable,
				"production":  ProfileNormal,
				"staging":     ProfileVolatile,
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		// Count NotReady nodes per namespace
		nsTotal := make(map[string]int)
		nsNotReady := make(map[string]int)
		for _, node := range engine.nodes {
			nsTotal[node.Namespace]++
			if len(node.TransitionHistory) > 0 {
				nsNotReady[node.Namespace]++
			}
		}

		// kube-system (stable): fewer than 2% NotReady
		kubeTotal := nsTotal["kube-system"]
		if kubeTotal > 0 {
			kubeNotReadyPct := float64(nsNotReady["kube-system"]) / float64(kubeTotal) * 100
			if kubeNotReadyPct >= 2.0 {
				t.Fatalf("kube-system NotReady rate %.1f%% >= 2%% (stable profile should have ~0%%)",
					kubeNotReadyPct)
			}
		}

		// staging (volatile): should have higher NotReady rate than kube-system
		stagingTotal := nsTotal["staging"]
		if stagingTotal > 0 {
			stagingNotReadyPct := float64(nsNotReady["staging"]) / float64(stagingTotal) * 100
			// The volatile profile has ~0.006% per-second probability
			// Over 3600 seconds: P(at least one) = 1 - (1-0.00006)^3600 ≈ 19.4%
			// With statistical variance and small sample sizes (30 nodes), allow 2-50% range
			if stagingNotReadyPct < 2.0 || stagingNotReadyPct > 50.0 {
				t.Fatalf("staging NotReady rate %.1f%% outside expected range [2%%, 50%%]",
					stagingNotReadyPct)
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 4: Cluster NotReady rate
// **Validates: Requirements 1.4**
func TestProperty4_ClusterNotReadyRate(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		seed := rapid.Int64().Draw(t, "seed")

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: 100,
			Duration:  1 * time.Hour,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"kube-system": 0.2,
				"production":  0.5,
				"staging":     0.3,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"kube-system": ProfileStable,
				"production":  ProfileNormal,
				"staging":     ProfileVolatile,
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		result, err := engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		// Count nodes that experienced NotReady using TransitionHistory
		notReadyCount := 0
		for _, node := range engine.nodes {
			if len(node.TransitionHistory) > 0 {
				notReadyCount++
			}
		}

		notReadyPct := float64(notReadyCount) / float64(result.Stats.TotalNodes) * 100

		// With default config: 20% stable (0%), 50% normal (~10%), 30% volatile (~19%)
		// Expected cluster rate: 0.2*0 + 0.5*10 + 0.3*19 ≈ 10.7%
		// Allow 3-25% range for statistical variance
		if notReadyPct < 3.0 || notReadyPct > 25.0 {
			t.Fatalf("cluster NotReady rate %.1f%% outside expected range [3%%, 25%%] (notReady=%d, total=%d)",
				notReadyPct, notReadyCount, result.Stats.TotalNodes)
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 1: Health profile assignment
// **Validates: Requirements 1.1**
func TestProperty1_HealthProfileAssignment(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(5, 100).Draw(t, "nodeCount")
		seed := rapid.Int64().Draw(t, "seed")

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  10 * time.Minute,
			Seed:      seed,
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		validProfiles := map[NodeHealthProfile]bool{
			ProfileStable:   true,
			ProfileNormal:   true,
			ProfileDrifting: true,
			ProfileVolatile: true,
		}

		for _, node := range engine.nodes {
			if !validProfiles[node.Profile] {
				t.Fatalf("node %q has invalid health profile %q", node.Name, node.Profile)
			}
			// Verify profile matches namespace config
			expectedProfile := engine.config.NamespaceProfiles[node.Namespace]
			if expectedProfile == "" {
				expectedProfile = ProfileNormal
			}
			if node.Profile != expectedProfile {
				t.Fatalf("node %q in namespace %q: profile=%q, expected=%q",
					node.Name, node.Namespace, node.Profile, expectedProfile)
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 2: NotReady cause validity
// **Validates: Requirements 1.2**
func TestProperty2_NotReadyCauseValidity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(10, 50).Draw(t, "nodeCount")
		seed := rapid.Int64().Draw(t, "seed")

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  30 * time.Minute,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"volatile-ns": 1.0,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"volatile-ns": ProfileVolatile,
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		validCauses := map[NotReadyCause]bool{
			CauseNetworkBlip:    true,
			CauseOOMKill:        true,
			CauseDiskPressure:   true,
			CauseKubeletRestart: true,
		}

		for _, node := range engine.nodes {
			for _, tr := range node.TransitionHistory {
				if !validCauses[tr.Cause] {
					t.Fatalf("node %q: NotReady transition has invalid cause %q", node.Name, tr.Cause)
				}
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 6: Lease interval base and jitter
// **Validates: Requirements 2.1, 2.2**
func TestProperty6_LeaseIntervalBaseAndJitter(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(5, 20).Draw(t, "nodeCount")
		durationMin := rapid.IntRange(5, 20).Draw(t, "durationMin")
		seed := rapid.Int64().Draw(t, "seed")

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  time.Duration(durationMin) * time.Minute,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"stable-ns": 1.0,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"stable-ns": ProfileStable,
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		baseInterval := engine.config.BaseInterval

		for _, node := range engine.nodes {
			if len(node.LeaseHistory) < 3 {
				continue
			}

			// Compute mean interval
			var totalInterval time.Duration
			intervals := make([]time.Duration, 0, len(node.LeaseHistory)-1)
			for i := 1; i < len(node.LeaseHistory); i++ {
				gap := node.LeaseHistory[i].Timestamp.Sub(node.LeaseHistory[i-1].Timestamp)
				totalInterval += gap
				intervals = append(intervals, gap)
			}
			meanInterval := totalInterval / time.Duration(len(intervals))

			// Mean should be within 1 second of base interval
			diff := meanInterval - baseInterval
			if diff < 0 {
				diff = -diff
			}
			if diff > 1*time.Second {
				t.Fatalf("node %q: mean interval %v deviates more than 1s from base %v",
					node.Name, meanInterval, baseInterval)
			}

			// Intervals should not all be identical (jitter is applied)
			allSame := true
			for i := 1; i < len(intervals); i++ {
				if intervals[i] != intervals[0] {
					allSame = false
					break
				}
			}
			if allSame && len(intervals) > 2 {
				t.Fatalf("node %q: all %d intervals are identical (%v), expected jitter",
					node.Name, len(intervals), intervals[0])
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 8: Drifting node interval increase
// **Validates: Requirements 2.4**
func TestProperty8_DriftingNodeIntervalIncrease(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(10, 20).Draw(t, "nodeCount")
		seed := rapid.Int64().Draw(t, "seed")

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  30 * time.Minute,
			Seed:      seed,
			NamespaceRatios: map[string]float64{
				"drifting-ns": 1.0,
			},
			NamespaceProfiles: map[string]NodeHealthProfile{
				"drifting-ns": ProfileDrifting,
			},
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		_, err = engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		baseInterval := engine.config.BaseInterval
		maxDrift := engine.config.MaxDriftIncrease

		// Aggregate intervals across all nodes for first and last quarter
		var firstSum, lastSum time.Duration
		var firstCount, lastCount int

		for _, node := range engine.nodes {
			if len(node.LeaseHistory) < 10 {
				continue
			}

			n := len(node.LeaseHistory)
			q1End := n / 4
			q4Start := 3 * n / 4

			for i := 1; i < q1End; i++ {
				gap := node.LeaseHistory[i].Timestamp.Sub(node.LeaseHistory[i-1].Timestamp)
				firstSum += gap
				firstCount++
			}

			for i := q4Start + 1; i < n; i++ {
				gap := node.LeaseHistory[i].Timestamp.Sub(node.LeaseHistory[i-1].Timestamp)
				lastSum += gap
				lastCount++
			}
		}

		if firstCount == 0 || lastCount == 0 {
			t.Skip("not enough lease data to compare quarters")
		}

		firstAvg := firstSum / time.Duration(firstCount)
		lastAvg := lastSum / time.Duration(lastCount)

		// Aggregate last quarter average should be greater than first quarter average
		if lastAvg <= firstAvg {
			t.Fatalf("aggregate last quarter avg interval %v <= first quarter avg %v (expected drift increase across %d nodes)",
				lastAvg, firstAvg, len(engine.nodes))
		}

		// Maximum drift should not exceed configured max (3s default) + tolerance for jitter
		maxObservedDrift := lastAvg - baseInterval
		if maxObservedDrift > maxDrift+2*time.Second {
			t.Fatalf("aggregate observed drift %v exceeds max drift %v + 2s jitter tolerance",
				maxObservedDrift, maxDrift)
		}
	})
}

// --- Property-Based Tests for Task 6: File Output and Manifest Generation ---

// Feature: realistic-data-and-visualizations, Property 21: Simulation duration and file segmentation
// **Validates: Requirements 6.1, 6.4, 6.5**
func TestProperty21_SimulationDurationAndFileSegmentation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(5, 20).Draw(t, "nodeCount")
		durationMin := rapid.IntRange(5, 60).Draw(t, "durationMin")
		segmentMin := rapid.IntRange(1, 10).Draw(t, "segmentMin")
		seed := rapid.Int64().Draw(t, "seed")

		duration := time.Duration(durationMin) * time.Minute
		segmentWindow := time.Duration(segmentMin) * time.Minute

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount:         nodeCount,
			Duration:          duration,
			FileSegmentWindow: segmentWindow,
			Seed:              seed,
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		result, err := engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		// Expected number of files = ceil(duration / segment_window)
		expectedFiles := int(math.Ceil(float64(duration) / float64(segmentWindow)))

		if len(result.LeaseFiles) != expectedFiles {
			t.Fatalf("expected %d lease files, got %d (duration=%v, window=%v)",
				expectedFiles, len(result.LeaseFiles), duration, segmentWindow)
		}

		if len(result.EbpfFiles) != expectedFiles {
			t.Fatalf("expected %d ebpf files, got %d (duration=%v, window=%v)",
				expectedFiles, len(result.EbpfFiles), duration, segmentWindow)
		}

		// Manifest should list files in chronological order matching actual filenames
		if len(result.Manifest) != expectedFiles {
			t.Fatalf("manifest has %d entries, expected %d", len(result.Manifest), expectedFiles)
		}
		for i, fname := range result.Manifest {
			if fname != result.LeaseFiles[i].Filename {
				t.Fatalf("manifest[%d]=%q != LeaseFiles[%d].Filename=%q",
					i, fname, i, result.LeaseFiles[i].Filename)
			}
		}

		if len(result.EbpfManifest) != expectedFiles {
			t.Fatalf("ebpf manifest has %d entries, expected %d", len(result.EbpfManifest), expectedFiles)
		}
		for i, fname := range result.EbpfManifest {
			if fname != result.EbpfFiles[i].Filename {
				t.Fatalf("ebpf manifest[%d]=%q != EbpfFiles[%d].Filename=%q",
					i, fname, i, result.EbpfFiles[i].Filename)
			}
		}

		// Verify chronological ordering of filenames
		for i := 1; i < len(result.Manifest); i++ {
			if result.Manifest[i] <= result.Manifest[i-1] {
				t.Fatalf("manifest not in chronological order: [%d]=%q >= [%d]=%q",
					i-1, result.Manifest[i-1], i, result.Manifest[i])
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 22: Output format round-trip
// **Validates: Requirements 6.2**
func TestProperty22_OutputFormatRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodeCount := rapid.IntRange(5, 20).Draw(t, "nodeCount")
		durationMin := rapid.IntRange(1, 15).Draw(t, "durationMin")
		seed := rapid.Int64().Draw(t, "seed")

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: nodeCount,
			Duration:  time.Duration(durationMin) * time.Minute,
			Seed:      seed,
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		result, err := engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		// For each lease file, serialize to JSON and deserialize back
		for _, lf := range result.LeaseFiles {
			// Serialize
			jsonBytes, err := json.Marshal(lf.Data)
			if err != nil {
				t.Fatalf("failed to marshal lease file %q: %v", lf.Filename, err)
			}

			// Deserialize
			var roundTripped map[string][]LeasePoint
			if err := json.Unmarshal(jsonBytes, &roundTripped); err != nil {
				t.Fatalf("failed to unmarshal lease file %q: %v", lf.Filename, err)
			}

			// Verify all namespace keys preserved
			if len(roundTripped) != len(lf.Data) {
				t.Fatalf("file %q: namespace count mismatch after round-trip: %d vs %d",
					lf.Filename, len(roundTripped), len(lf.Data))
			}

			for ns, origPoints := range lf.Data {
				rtPoints, ok := roundTripped[ns]
				if !ok {
					t.Fatalf("file %q: namespace %q missing after round-trip", lf.Filename, ns)
				}
				if len(rtPoints) != len(origPoints) {
					t.Fatalf("file %q, ns %q: point count mismatch: %d vs %d",
						lf.Filename, ns, len(rtPoints), len(origPoints))
				}
				for j, orig := range origPoints {
					if rtPoints[j].X != orig.X || rtPoints[j].Y != orig.Y {
						t.Fatalf("file %q, ns %q, point %d: mismatch (%d,%d) vs (%d,%d)",
							lf.Filename, ns, j, rtPoints[j].X, rtPoints[j].Y, orig.X, orig.Y)
					}
				}
			}
		}
	})
}

// Feature: realistic-data-and-visualizations, Property 23: Scenario injection rate
// **Validates: Requirements 6.3**
func TestProperty23_ScenarioInjectionRate(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Duration > 1 hour
		durationHours := rapid.IntRange(2, 5).Draw(t, "durationHours")
		seed := rapid.Int64().Draw(t, "seed")

		duration := time.Duration(durationHours) * time.Hour

		// Inject at least floor(duration_in_hours) scenarios
		numScenarios := durationHours
		scenarios := make([]ScenarioConfig, numScenarios)
		for i := 0; i < numScenarios; i++ {
			scenarios[i] = ScenarioConfig{
				Type:             "rolling_deployment",
				TriggerAt:        time.Duration(i)*time.Hour + 30*time.Minute,
				NodeCount:        2,
				StaggerInterval:  30 * time.Second,
				ReplacementDelay: 15 * time.Second,
			}
		}

		engine, err := NewSimulationEngine(SimulationConfig{
			NodeCount: 30,
			Duration:  duration,
			Seed:      seed,
			Scenarios: scenarios,
		})
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		result, err := engine.Run()
		if err != nil {
			t.Fatalf("simulation failed: %v", err)
		}

		expectedMinScenarios := int(math.Floor(float64(durationHours)))
		if result.Stats.ScenarioCount < expectedMinScenarios {
			t.Fatalf("scenario count %d < floor(duration_hours)=%d",
				result.Stats.ScenarioCount, expectedMinScenarios)
		}
	})
}

// --- Task 6.6: Unit tests for file output ---

func TestFileOutput_JSONSchema(t *testing.T) {
	engine, err := NewSimulationEngine(SimulationConfig{
		NodeCount:         5,
		Duration:          2 * time.Minute,
		FileSegmentWindow: 1 * time.Minute,
		Seed:              42,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	result, err := engine.Run()
	if err != nil {
		t.Fatalf("simulation failed: %v", err)
	}

	// Should have ceil(2min / 1min) = 2 files
	if len(result.LeaseFiles) != 2 {
		t.Fatalf("expected 2 lease files, got %d", len(result.LeaseFiles))
	}
	if len(result.EbpfFiles) != 2 {
		t.Fatalf("expected 2 ebpf files, got %d", len(result.EbpfFiles))
	}

	// Verify lease file data format
	for _, lf := range result.LeaseFiles {
		if lf.Filename == "" {
			t.Fatal("lease file has empty filename")
		}
		// Filename should match pattern leases{timestamp}.json
		if len(lf.Filename) < 10 {
			t.Fatalf("lease filename too short: %q", lf.Filename)
		}

		// Data should be map[string][]LeasePoint
		for ns, points := range lf.Data {
			if ns == "" {
				t.Fatal("empty namespace key in lease file data")
			}
			for i, p := range points {
				if p.X != i {
					t.Fatalf("file %q, ns %q: point %d has X=%d, expected %d",
						lf.Filename, ns, i, p.X, i)
				}
				if p.Y <= 0 {
					t.Fatalf("file %q, ns %q: point %d has Y=%d, expected > 0",
						lf.Filename, ns, i, p.Y)
				}
			}
		}
	}

	// Verify JSON serialization works
	for _, lf := range result.LeaseFiles {
		jsonBytes, err := json.Marshal(lf.Data)
		if err != nil {
			t.Fatalf("failed to marshal lease file %q: %v", lf.Filename, err)
		}
		if len(jsonBytes) == 0 {
			t.Fatalf("empty JSON for lease file %q", lf.Filename)
		}
	}
}

func TestFileOutput_ManifestOrdering(t *testing.T) {
	engine, err := NewSimulationEngine(SimulationConfig{
		NodeCount:         5,
		Duration:          15 * time.Minute,
		FileSegmentWindow: 5 * time.Minute,
		Seed:              42,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	result, err := engine.Run()
	if err != nil {
		t.Fatalf("simulation failed: %v", err)
	}

	// Should have ceil(15min / 5min) = 3 files
	if len(result.Manifest) != 3 {
		t.Fatalf("expected 3 manifest entries, got %d", len(result.Manifest))
	}
	if len(result.EbpfManifest) != 3 {
		t.Fatalf("expected 3 ebpf manifest entries, got %d", len(result.EbpfManifest))
	}

	// Verify chronological ordering
	for i := 1; i < len(result.Manifest); i++ {
		if result.Manifest[i] <= result.Manifest[i-1] {
			t.Fatalf("manifest not chronological: [%d]=%q, [%d]=%q",
				i-1, result.Manifest[i-1], i, result.Manifest[i])
		}
	}
	for i := 1; i < len(result.EbpfManifest); i++ {
		if result.EbpfManifest[i] <= result.EbpfManifest[i-1] {
			t.Fatalf("ebpf manifest not chronological: [%d]=%q, [%d]=%q",
				i-1, result.EbpfManifest[i-1], i, result.EbpfManifest[i])
		}
	}

	// Verify manifest entries match file outputs
	for i, fname := range result.Manifest {
		if fname != result.LeaseFiles[i].Filename {
			t.Fatalf("manifest[%d]=%q != LeaseFiles[%d].Filename=%q",
				i, fname, i, result.LeaseFiles[i].Filename)
		}
	}
}

func TestFileOutput_LeasesByNamespaceFormat(t *testing.T) {
	engine, err := NewSimulationEngine(SimulationConfig{
		NodeCount:         10,
		Duration:          5 * time.Minute,
		FileSegmentWindow: 5 * time.Minute,
		Seed:              42,
		NamespaceRatios: map[string]float64{
			"kube-system": 0.2,
			"production":  0.5,
			"staging":     0.3,
		},
		NamespaceProfiles: map[string]NodeHealthProfile{
			"kube-system": ProfileStable,
			"production":  ProfileNormal,
			"staging":     ProfileVolatile,
		},
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	result, err := engine.Run()
	if err != nil {
		t.Fatalf("simulation failed: %v", err)
	}

	// Should have 1 file (5min / 5min)
	if len(result.LeaseFiles) != 1 {
		t.Fatalf("expected 1 lease file, got %d", len(result.LeaseFiles))
	}

	lf := result.LeaseFiles[0]

	// Should have data for all 3 namespaces
	if len(lf.Data) < 3 {
		t.Fatalf("expected at least 3 namespaces in lease file, got %d", len(lf.Data))
	}

	// Verify each namespace has lease points
	for _, ns := range []string{"kube-system", "production", "staging"} {
		points, ok := lf.Data[ns]
		if !ok {
			t.Fatalf("namespace %q missing from lease file data", ns)
		}
		if len(points) == 0 {
			t.Fatalf("namespace %q has 0 lease points", ns)
		}

		// Verify X values are sequential indices
		for i, p := range points {
			if p.X != i {
				t.Fatalf("ns %q: point %d has X=%d, expected %d", ns, i, p.X, i)
			}
			if p.Y <= 0 {
				t.Fatalf("ns %q: point %d has Y=%d, expected > 0", ns, i, p.Y)
			}
		}
	}

	// Verify round-trip JSON serialization
	jsonBytes, err := json.Marshal(lf.Data)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var roundTripped map[string][]LeasePoint
	if err := json.Unmarshal(jsonBytes, &roundTripped); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(roundTripped) != len(lf.Data) {
		t.Fatalf("round-trip namespace count mismatch: %d vs %d", len(roundTripped), len(lf.Data))
	}
}
