package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// YAML schema types
// ---------------------------------------------------------------------------

// Scenario represents a single Earthworm evaluation scenario loaded from YAML.
type Scenario struct {
	Name     string   `yaml:"name"`
	Inputs   Inputs   `yaml:"inputs"`
	Expected Expected `yaml:"expected"`
}

// Inputs holds the input data for a scenario evaluation.
type Inputs struct {
	Heartbeat HeartbeatInput `yaml:"heartbeat"`
	Config    ConfigInput    `yaml:"config"`
	AIScore   *float64       `yaml:"ai_score"` // nil means AI unavailable
}

// HeartbeatInput mirrors the heartbeat telemetry fields.
type HeartbeatInput struct {
	NodeID      string  `yaml:"node_id"`
	CPUUsage    float64 `yaml:"cpu_usage"`
	MemoryUsage float64 `yaml:"memory_usage"`
	DiskIO      float64 `yaml:"disk_io"`
	NetworkIO   float64 `yaml:"network_io"`
	Latency     float64 `yaml:"latency"`
}

// ConfigInput holds decision thresholds.
type ConfigInput struct {
	Threshold      float64        `yaml:"threshold"`
	RuleThresholds RuleThresholds `yaml:"rule_thresholds"`
}

// RuleThresholds defines per-metric thresholds for rule-based fallback.
type RuleThresholds struct {
	CPUThreshold     float64 `yaml:"cpu_threshold"`
	MemoryThreshold  float64 `yaml:"memory_threshold"`
	LatencyThreshold float64 `yaml:"latency_threshold"`
}

// Expected holds the expected outcome of the scenario.
type Expected struct {
	ActionTaken   bool    `yaml:"action_taken"`
	ActionType    string  `yaml:"action_type"`
	WasRuleBased  bool    `yaml:"was_rule_based"`
	MinScore      *float64 `yaml:"min_score"`
	MaxScore      *float64 `yaml:"max_score"`
	TriggeredRule string  `yaml:"triggered_rule"`
}

// ---------------------------------------------------------------------------
// Decision engine
// ---------------------------------------------------------------------------

// Decision is the output of the Earthworm decision engine for a scenario.
type Decision struct {
	ActionTaken   bool
	ActionType    string
	WasRuleBased  bool
	Score         float64
	TriggeredRule string
}

// Evaluate runs the Earthworm decision logic against the given inputs.
// When ai_score is nil (AI unavailable), it falls back to rule-based evaluation.
func Evaluate(inputs Inputs) Decision {
	// If AI score is available, use it as primary decision source.
	if inputs.AIScore != nil {
		score := *inputs.AIScore
		if score >= inputs.Config.Threshold {
			actionType := inferActionType(inputs.Heartbeat, score)
			return Decision{
				ActionTaken:  true,
				ActionType:   actionType,
				WasRuleBased: false,
				Score:        score,
			}
		}
		// AI score below threshold — no action.
		return Decision{
			ActionTaken:  false,
			ActionType:   "",
			WasRuleBased: false,
			Score:        score,
		}
	}

	// AI unavailable — rule-based fallback.
	return evaluateRuleBased(inputs)
}

// evaluateRuleBased applies threshold rules to decide on an action.
func evaluateRuleBased(inputs Inputs) Decision {
	hb := inputs.Heartbeat
	rt := inputs.Config.RuleThresholds

	// Evaluate rules in priority order.
	// 1. Latency rule — triggers workload_reschedule
	if rt.LatencyThreshold > 0 && hb.Latency >= rt.LatencyThreshold {
		score := computeRuleScore(hb.Latency, rt.LatencyThreshold)
		return Decision{
			ActionTaken:   true,
			ActionType:    "workload_reschedule",
			WasRuleBased:  true,
			Score:         score,
			TriggeredRule: "latency",
		}
	}

	// 2. CPU rule — triggers node_cordon
	if rt.CPUThreshold > 0 && hb.CPUUsage >= rt.CPUThreshold {
		score := computeRuleScore(hb.CPUUsage, rt.CPUThreshold)
		return Decision{
			ActionTaken:   true,
			ActionType:    "node_cordon",
			WasRuleBased:  true,
			Score:         score,
			TriggeredRule: "cpu",
		}
	}

	// 3. Memory rule — triggers pod_restart
	if rt.MemoryThreshold > 0 && hb.MemoryUsage >= rt.MemoryThreshold {
		score := computeRuleScore(hb.MemoryUsage, rt.MemoryThreshold)
		return Decision{
			ActionTaken:   true,
			ActionType:    "pod_restart",
			WasRuleBased:  true,
			Score:         score,
			TriggeredRule: "memory",
		}
	}

	// No rule triggered.
	return Decision{
		ActionTaken:  false,
		ActionType:   "",
		WasRuleBased: true,
		Score:        0,
	}
}

// computeRuleScore calculates a normalized score based on how far a metric
// exceeds its threshold (clamped to [0,1]).
func computeRuleScore(value, threshold float64) float64 {
	if threshold <= 0 {
		return 0
	}
	// Score is how much the value exceeds the threshold, normalized.
	score := value / threshold
	return math.Min(score, 1.0)
}

// inferActionType determines the remediation action based on heartbeat metrics.
func inferActionType(hb HeartbeatInput, score float64) string {
	// High latency + high score → reschedule workloads away from node.
	if hb.Latency > 500 {
		return "workload_reschedule"
	}
	// High CPU → cordon the node to prevent new scheduling.
	if hb.CPUUsage > 90 {
		return "node_cordon"
	}
	// High memory → restart the offending pod.
	if hb.MemoryUsage > 85 {
		return "pod_restart"
	}
	// Default for high combined score.
	if score >= 0.9 {
		return "node_cordon"
	}
	return "pod_restart"
}

// ---------------------------------------------------------------------------
// Scenario evaluation & reporting
// ---------------------------------------------------------------------------

// Result captures the pass/fail status for a single scenario.
type Result struct {
	ScenarioName string
	Passed       bool
	Failures     []string
}

// Check evaluates the decision against expected outcomes.
func Check(scenario Scenario, decision Decision) Result {
	r := Result{ScenarioName: scenario.Name, Passed: true}
	exp := scenario.Expected

	if decision.ActionTaken != exp.ActionTaken {
		r.fail(fmt.Sprintf("action_taken: got %v, want %v", decision.ActionTaken, exp.ActionTaken))
	}

	if exp.ActionType != "" && decision.ActionType != exp.ActionType {
		r.fail(fmt.Sprintf("action_type: got %q, want %q", decision.ActionType, exp.ActionType))
	}

	if decision.WasRuleBased != exp.WasRuleBased {
		r.fail(fmt.Sprintf("was_rule_based: got %v, want %v", decision.WasRuleBased, exp.WasRuleBased))
	}

	if exp.MinScore != nil && decision.Score < *exp.MinScore {
		r.fail(fmt.Sprintf("score %.4f below min_score %.4f", decision.Score, *exp.MinScore))
	}

	if exp.MaxScore != nil && decision.Score > *exp.MaxScore {
		r.fail(fmt.Sprintf("score %.4f above max_score %.4f", decision.Score, *exp.MaxScore))
	}

	if exp.TriggeredRule != "" && decision.TriggeredRule != exp.TriggeredRule {
		r.fail(fmt.Sprintf("triggered_rule: got %q, want %q", decision.TriggeredRule, exp.TriggeredRule))
	}

	return r
}

func (r *Result) fail(msg string) {
	r.Passed = false
	r.Failures = append(r.Failures, msg)
}

// ---------------------------------------------------------------------------
// File loading
// ---------------------------------------------------------------------------

// LoadScenarios reads all YAML files in a directory and parses them as scenarios.
func LoadScenarios(dir string) ([]Scenario, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading scenario directory %s: %w", dir, err)
	}

	var scenarios []Scenario
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}

		var s Scenario
		if err := yaml.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		if s.Name == "" {
			s.Name = entry.Name()
		}
		scenarios = append(scenarios, s)
	}

	return scenarios, nil
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	scenarioDir := flag.String("scenarios", "eval/scenarios/earthworm", "Path to Earthworm scenario YAML directory")
	verbose := flag.Bool("verbose", false, "Print details for each scenario")
	flag.Parse()

	scenarios, err := LoadScenarios(*scenarioDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading scenarios: %v\n", err)
		os.Exit(1)
	}

	if len(scenarios) == 0 {
		fmt.Fprintf(os.Stderr, "No scenarios found in %s\n", *scenarioDir)
		os.Exit(1)
	}

	var results []Result
	passed, failed := 0, 0

	for _, sc := range scenarios {
		decision := Evaluate(sc.Inputs)
		result := Check(sc, decision)
		results = append(results, result)

		if result.Passed {
			passed++
			if *verbose {
				fmt.Printf("  PASS  %s\n", result.ScenarioName)
			}
		} else {
			failed++
			fmt.Printf("  FAIL  %s\n", result.ScenarioName)
			for _, f := range result.Failures {
				fmt.Printf("        - %s\n", f)
			}
		}
	}

	fmt.Println()
	fmt.Printf("Earthworm Eval: %d scenarios, %d passed, %d failed\n", len(scenarios), passed, failed)

	if failed > 0 {
		os.Exit(1)
	}
}
