package kubernetes

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// NodeHealthProfile determines a node's behavior during simulation
type NodeHealthProfile string

const (
	ProfileStable   NodeHealthProfile = "stable"
	ProfileNormal   NodeHealthProfile = "normal"
	ProfileDrifting NodeHealthProfile = "drifting"
	ProfileVolatile NodeHealthProfile = "volatile"
)

// NotReadyCause describes why a node went NotReady
type NotReadyCause string

const (
	CauseNetworkBlip    NotReadyCause = "network_blip"
	CauseOOMKill        NotReadyCause = "oom_kill"
	CauseDiskPressure   NotReadyCause = "disk_pressure"
	CauseKubeletRestart NotReadyCause = "kubelet_restart"
)

// SimulationConfig holds all parameters for a simulation run
type SimulationConfig struct {
	NodeCount         int
	Duration          time.Duration
	BaseInterval      time.Duration            // default 10s
	JitterStdDev      time.Duration            // default 500ms
	FileSegmentWindow time.Duration            // default 5min
	NamespaceRatios   map[string]float64       // e.g. {"kube-system": 0.2, "production": 0.5, "staging": 0.3}
	NamespaceProfiles map[string]NodeHealthProfile
	Scenarios         []ScenarioConfig
	MaxDriftIncrease  time.Duration // default 3s
	Seed              int64         // 0 means use time-based seed
}

// ScenarioConfig defines a cluster scenario event
type ScenarioConfig struct {
	Type             string        // "rolling_deployment", "network_partition"
	TriggerAt        time.Duration // offset from simulation start
	NodeCount        int           // nodes affected
	StaggerInterval  time.Duration // default 30s
	ReplacementDelay time.Duration // default 15s
}

// NotReadyTransition records a single NotReady transition event
type NotReadyTransition struct {
	Timestamp time.Time
	Cause     NotReadyCause
	Duration  time.Duration
}

// SimNode extends MockNode with simulation state
type SimNode struct {
	Name               string
	Namespace          string
	Profile            NodeHealthProfile
	Status             string // "Ready", "NotReady", "Unknown"
	NotReadyCause      NotReadyCause
	NotReadyUntil      time.Time
	BaseInterval       time.Duration
	CurrentDrift       time.Duration
	LastLeaseTime      time.Time
	LeaseHistory       []LeaseEvent
	EbpfEvents         []SimEbpfEvent
	TransitionHistory  []NotReadyTransition
}

// LeaseEvent is a single lease renewal record
type LeaseEvent struct {
	Timestamp time.Time
	NodeName  string
	Namespace string
}

// SimEbpfEvent extends MockEBPFEvent with structured fields
type SimEbpfEvent struct {
	Timestamp  time.Time
	PID        uint32
	PPID       uint32
	Comm       string
	Syscall    string
	CgroupPath string
	NodeName   string
	Namespace  string
}

// LeasePoint matches the existing JSON format {x, y} used by the visualizer
type LeasePoint struct {
	X int   `json:"x"`
	Y int64 `json:"y"`
}

// LeaseFileOutput represents a segmented JSON file of lease data
type LeaseFileOutput struct {
	Filename string
	Data     map[string][]LeasePoint // LeasesByNamespace format
}

// EbpfFileOutput represents a segmented eBPF event file
type EbpfFileOutput struct {
	Filename string
	Events   []SimEbpfEvent
}

// SimulationStats holds summary statistics for a simulation run
type SimulationStats struct {
	TotalNodes      int
	TotalLeases     int
	TotalEbpfEvents int
	NotReadyCount   int
	ScenarioCount   int
}

// SimulationResult holds all output from a simulation run
type SimulationResult struct {
	LeaseFiles   []LeaseFileOutput
	EbpfFiles    []EbpfFileOutput
	Manifest     []string
	EbpfManifest []string
	Stats        SimulationStats
}

// drainedNodeInfo tracks a node that was drained during a rolling deployment
type drainedNodeInfo struct {
	nodeName       string
	drainTime      time.Time
	replacementAt  time.Time // drainTime + ReplacementDelay
	replaced       bool
	replacementName string
}

// activeScenario tracks the runtime state of a rolling deployment scenario
type activeScenario struct {
	config       ScenarioConfig
	started      bool
	drainIndex   int              // how many nodes have been drained so far
	drainedNodes []drainedNodeInfo
	nextDrainAt  time.Duration    // elapsed time for next drain
}

// SimulationEngine runs a tick-based simulation producing realistic data
type SimulationEngine struct {
	config          SimulationConfig
	nodes           []*SimNode
	clock           time.Time
	startTime       time.Time
	rng             *rand.Rand
	scenarios       []ScenarioConfig
	activeScenarios []*activeScenario
	nextNodeID      int // counter for generating unique node names
}

const (
	maxSimulationDuration = 7 * 24 * time.Hour // 7 days
	defaultBaseInterval   = 10 * time.Second
	defaultJitterStdDev   = 500 * time.Millisecond
	defaultSegmentWindow  = 5 * time.Minute
	defaultMaxDrift       = 3 * time.Second
)

// ValidateConfig checks and normalizes a SimulationConfig, returning an error
// if the configuration is invalid.
func ValidateConfig(cfg *SimulationConfig) error {
	if cfg.NodeCount <= 0 {
		return fmt.Errorf("node count must be greater than 0, got %d", cfg.NodeCount)
	}
	if cfg.Duration <= 0 {
		return fmt.Errorf("duration must be greater than 0, got %v", cfg.Duration)
	}
	if cfg.Duration > maxSimulationDuration {
		return fmt.Errorf("duration %v exceeds maximum of 7 days", cfg.Duration)
	}

	// Apply defaults
	if cfg.BaseInterval == 0 {
		cfg.BaseInterval = defaultBaseInterval
	}
	if cfg.JitterStdDev == 0 {
		cfg.JitterStdDev = defaultJitterStdDev
	}
	if cfg.FileSegmentWindow == 0 {
		cfg.FileSegmentWindow = defaultSegmentWindow
	}
	if cfg.MaxDriftIncrease == 0 {
		cfg.MaxDriftIncrease = defaultMaxDrift
	}

	// Normalize namespace ratios
	if len(cfg.NamespaceRatios) == 0 {
		cfg.NamespaceRatios = map[string]float64{
			"kube-system": 0.2,
			"production":  0.5,
			"staging":     0.3,
		}
	}
	normalizeRatios(cfg.NamespaceRatios)

	// Default namespace profiles if not set
	if len(cfg.NamespaceProfiles) == 0 {
		cfg.NamespaceProfiles = map[string]NodeHealthProfile{
			"kube-system": ProfileStable,
			"production":  ProfileNormal,
			"staging":     ProfileVolatile,
		}
	}

	return nil
}

// normalizeRatios adjusts namespace ratios so they sum to 1.0
func normalizeRatios(ratios map[string]float64) {
	sum := 0.0
	for _, v := range ratios {
		sum += v
	}
	if sum == 0 {
		return
	}
	for k, v := range ratios {
		ratios[k] = v / sum
	}
}

// NewSimulationEngine validates the config and initializes nodes distributed
// across namespaces per the configured ratios.
func NewSimulationEngine(config SimulationConfig) (*SimulationEngine, error) {
	if err := ValidateConfig(&config); err != nil {
		return nil, err
	}

	seed := config.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))

	nodes := distributeNodes(config, rng)

	// Build active scenario trackers
	var activeScens []*activeScenario
	for _, sc := range config.Scenarios {
		as := &activeScenario{
			config:      sc,
			nextDrainAt: sc.TriggerAt,
		}
		activeScens = append(activeScens, as)
	}

	now := time.Now()
	return &SimulationEngine{
		config:          config,
		nodes:           nodes,
		clock:           now,
		startTime:       now,
		rng:             rng,
		scenarios:       config.Scenarios,
		activeScenarios: activeScens,
		nextNodeID:      len(nodes),
	}, nil
}

// distributeNodes creates SimNodes distributed across namespaces per ratios,
// assigning health profiles from NamespaceProfiles.
func distributeNodes(config SimulationConfig, rng *rand.Rand) []*SimNode {
	// Sort namespaces deterministically for reproducible distribution
	namespaces := sortedKeys(config.NamespaceRatios)

	// Calculate node counts per namespace
	nsCounts := make(map[string]int)
	remaining := config.NodeCount
	for i, ns := range namespaces {
		if i == len(namespaces)-1 {
			// Last namespace gets the remainder to avoid rounding issues
			nsCounts[ns] = remaining
		} else {
			count := int(math.Round(config.NamespaceRatios[ns] * float64(config.NodeCount)))
			if count > remaining {
				count = remaining
			}
			nsCounts[ns] = count
			remaining -= count
		}
	}

	nodes := make([]*SimNode, 0, config.NodeCount)
	nodeIdx := 0
	for _, ns := range namespaces {
		profile := config.NamespaceProfiles[ns]
		if profile == "" {
			profile = ProfileNormal
		}
		for i := 0; i < nsCounts[ns]; i++ {
			nodes = append(nodes, &SimNode{
				Name:         fmt.Sprintf("node-%03d", nodeIdx),
				Namespace:    ns,
				Profile:      profile,
				Status:       "Ready",
				BaseInterval: config.BaseInterval,
			})
			nodeIdx++
		}
	}

	return nodes
}

// sortedKeys returns map keys in sorted order for deterministic iteration
func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Simple insertion sort for small maps
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// allCauses is the list of possible NotReady causes
var allCauses = []NotReadyCause{
	CauseNetworkBlip,
	CauseOOMKill,
	CauseDiskPressure,
	CauseKubeletRestart,
}

// notReadyProbabilityPerTick returns the per-second probability of transitioning
// to NotReady for a given health profile.
func notReadyProbabilityPerTick(profile NodeHealthProfile) float64 {
	switch profile {
	case ProfileStable:
		return 0.0 // stable nodes NEVER go NotReady
	case ProfileNormal:
		// Low probability: ~0.003% per second → ~10% of nodes over 1 hour
		return 0.000030
	case ProfileDrifting:
		// Same as normal for NotReady transitions
		return 0.000030
	case ProfileVolatile:
		// Higher probability: ~0.006% per second → ~20% of nodes over 1 hour
		return 0.000060
	default:
		return 0.000030
	}
}

// applyHealthTransitions checks each node and probabilistically transitions
// Ready nodes to NotReady based on their health profile. Nodes already in
// NotReady are transitioned back to Ready once their NotReadyUntil time passes.
func (se *SimulationEngine) applyHealthTransitions() {
	for _, node := range se.nodes {
		if node.Status == "NotReady" {
			// Don't recover nodes that were drained in a rolling deployment
			if node.NotReadyCause == "drain" {
				continue
			}
			// Check if NotReady duration has elapsed
			if !se.clock.Before(node.NotReadyUntil) {
				node.Status = "Ready"
				node.NotReadyCause = ""
			}
			continue
		}

		// Stable nodes never go NotReady
		if node.Profile == ProfileStable {
			continue
		}

		prob := notReadyProbabilityPerTick(node.Profile)
		if se.rng.Float64() < prob {
			// Transition to NotReady
			cause := allCauses[se.rng.Intn(len(allCauses))]
			// Duration between 5 and 120 seconds
			durationSec := 5 + se.rng.Intn(116) // 5..120 inclusive
			duration := time.Duration(durationSec) * time.Second
			node.Status = "NotReady"
			node.NotReadyCause = cause
			node.NotReadyUntil = se.clock.Add(duration)

			// Record the transition for test observability
			node.TransitionHistory = append(node.TransitionHistory, NotReadyTransition{
				Timestamp: se.clock,
				Cause:     cause,
				Duration:  duration,
			})

			// Generate correlated eBPF event for this NotReady transition
			se.generateCorrelatedEbpf(node, cause)
		}
	}
}

// generateLeaseForNode generates a lease event for a node if it is Ready and
// enough time has passed since its last lease (baseInterval + jitter).
// For drifting profiles, the base interval gradually increases over the simulation.
func (se *SimulationEngine) generateLeaseForNode(node *SimNode, elapsed time.Duration) {
	// Skip lease generation if node is NotReady
	if node.Status == "NotReady" {
		return
	}

	// Apply drift for drifting profile
	if node.Profile == ProfileDrifting && se.config.Duration > 0 {
		progress := float64(elapsed) / float64(se.config.Duration)
		if progress > 1.0 {
			progress = 1.0
		}
		node.CurrentDrift = time.Duration(float64(se.config.MaxDriftIncrease) * progress)
	}

	// Calculate effective interval: base + drift + jitter
	effectiveBase := node.BaseInterval + node.CurrentDrift
	jitter := time.Duration(se.rng.NormFloat64() * float64(se.config.JitterStdDev))
	nextInterval := effectiveBase + jitter

	// Ensure minimum interval of 1 second
	if nextInterval < time.Second {
		nextInterval = time.Second
	}

	// Check if enough time has passed since last lease
	if node.LastLeaseTime.IsZero() {
		// First lease — generate immediately
		node.LastLeaseTime = se.clock
		lease := LeaseEvent{
			Timestamp: se.clock,
			NodeName:  node.Name,
			Namespace: node.Namespace,
		}
		node.LeaseHistory = append(node.LeaseHistory, lease)
		// Generate background eBPF write event for this lease
		se.generateBackgroundEbpf(node, se.clock)
		return
	}

	timeSinceLast := se.clock.Sub(node.LastLeaseTime)
	if timeSinceLast >= nextInterval {
		node.LastLeaseTime = se.clock
		lease := LeaseEvent{
			Timestamp: se.clock,
			NodeName:  node.Name,
			Namespace: node.Namespace,
		}
		node.LeaseHistory = append(node.LeaseHistory, lease)
		// Generate background eBPF write event for this lease
		se.generateBackgroundEbpf(node, se.clock)
	}
}

// applyScenarios processes rolling deployment scenarios at the current tick.
// It drains nodes sequentially with the configured stagger interval and
// introduces replacement nodes with burst leases after the replacement delay.
func (se *SimulationEngine) applyScenarios(elapsed time.Duration) {
	for _, as := range se.activeScenarios {
		if as.config.Type != "rolling_deployment" {
			continue
		}

		// Drain nodes sequentially according to stagger interval
		for as.drainIndex < as.config.NodeCount && elapsed >= as.nextDrainAt {
			// Find a Ready node to drain (skip already-drained nodes)
			drained := false
			for _, node := range se.nodes {
				if node.Status == "Ready" && !se.isNodeDrained(node.Name) {
					// Drain this node
					node.Status = "NotReady"
					node.NotReadyCause = "drain"
					// Set NotReadyUntil far in the future so it stays drained
					node.NotReadyUntil = se.clock.Add(se.config.Duration)

					replacementTime := se.clock.Add(as.config.ReplacementDelay)
					as.drainedNodes = append(as.drainedNodes, drainedNodeInfo{
						nodeName:      node.Name,
						drainTime:     se.clock,
						replacementAt: replacementTime,
					})
					as.drainIndex++
					as.nextDrainAt = as.config.TriggerAt + time.Duration(as.drainIndex)*as.config.StaggerInterval
					drained = true
					break
				}
			}
			if !drained {
				// No more Ready nodes available to drain
				break
			}
		}

		// Check for replacement nodes that should be introduced
		for i := range as.drainedNodes {
			di := &as.drainedNodes[i]
			if di.replaced {
				continue
			}
			if !se.clock.Before(di.replacementAt) {
				// Create replacement node
				replacementName := fmt.Sprintf("node-repl-%03d", se.nextNodeID)
				se.nextNodeID++

				// Find the drained node to get its namespace
				ns := "production" // default
				for _, node := range se.nodes {
					if node.Name == di.nodeName {
						ns = node.Namespace
						break
					}
				}

				newNode := &SimNode{
					Name:         replacementName,
					Namespace:    ns,
					Profile:      ProfileNormal,
					Status:       "Ready",
					BaseInterval: se.config.BaseInterval,
				}

				// Generate initial burst of 3 leases within 5 seconds
				se.generateBurstLeases(newNode, se.clock, 3, 5*time.Second)

				// Generate correlated eBPF fork event for replacement node
				se.generateReplacementForkEbpf(newNode, se.clock)

				se.nodes = append(se.nodes, newNode)
				di.replaced = true
				di.replacementName = replacementName
			}
		}
	}
}

// isNodeDrained checks if a node name is in any active scenario's drained list
func (se *SimulationEngine) isNodeDrained(name string) bool {
	for _, as := range se.activeScenarios {
		for _, di := range as.drainedNodes {
			if di.nodeName == name {
				return true
			}
		}
	}
	return false
}

// generateBurstLeases creates count lease events spread within the given window
// starting from startTime. Used for replacement node initial burst.
func (se *SimulationEngine) generateBurstLeases(node *SimNode, startTime time.Time, count int, window time.Duration) {
	// Generate `count` leases spread within `window`
	// First lease at startTime, subsequent ones evenly spaced within window
	for i := 0; i < count; i++ {
		var ts time.Time
		if i == 0 {
			ts = startTime
		} else {
			// Spread remaining leases within the window
			offset := time.Duration(float64(window) * float64(i) / float64(count))
			ts = startTime.Add(offset)
		}
		lease := LeaseEvent{
			Timestamp: ts,
			NodeName:  node.Name,
			Namespace: node.Namespace,
		}
		node.LeaseHistory = append(node.LeaseHistory, lease)
		node.LastLeaseTime = ts
		// Generate background eBPF write event for each burst lease
		se.generateBackgroundEbpf(node, ts)
	}
}

// tick advances the simulation by one second, applying health transitions,
// scenarios, and generating leases for all nodes.
func (se *SimulationEngine) tick(elapsed time.Duration) {
	se.applyHealthTransitions()
	se.applyScenarios(elapsed)
	for _, node := range se.nodes {
		se.generateLeaseForNode(node, elapsed)
	}
}

// Run executes the full simulation from start to start+duration in 1-second
// ticks and returns the SimulationResult with all lease and eBPF data.
func (se *SimulationEngine) Run() (*SimulationResult, error) {
	se.startTime = se.clock
	startTime := se.startTime
	totalSeconds := int(se.config.Duration.Seconds())

	for i := 0; i <= totalSeconds; i++ {
		elapsed := time.Duration(i) * time.Second
		se.clock = startTime.Add(elapsed)
		se.tick(elapsed)
	}

	// Collect results
	result := &SimulationResult{
		Stats: SimulationStats{
			TotalNodes: len(se.nodes),
		},
	}

	// Gather all leases and eBPF events
	notReadyNodes := make(map[string]bool)
	for _, node := range se.nodes {
		result.Stats.TotalLeases += len(node.LeaseHistory)
		result.Stats.TotalEbpfEvents += len(node.EbpfEvents)
		// Track nodes that experienced NotReady (check if they have a non-zero NotReadyUntil
		// or if they are currently NotReady — but we need to track during simulation)
	}

	// Count nodes that went NotReady at any point by checking lease gaps
	for _, node := range se.nodes {
		if hadNotReadyTransition(node, se.config.BaseInterval) {
			notReadyNodes[node.Name] = true
		}
	}
	result.Stats.NotReadyCount = len(notReadyNodes)
	result.Stats.ScenarioCount = len(se.config.Scenarios)

	// Segment output into time-windowed files
	result.LeaseFiles = se.SegmentLeaseFiles()
	result.EbpfFiles = se.SegmentEbpfFiles()
	result.Manifest = GenerateLeaseManifest(result.LeaseFiles)
	result.EbpfManifest = GenerateEbpfManifest(result.EbpfFiles)

	return result, nil
}

// hadNotReadyTransition checks if a node experienced any NotReady transition
// by looking for gaps in lease history that exceed 2x the base interval.
func hadNotReadyTransition(node *SimNode, baseInterval time.Duration) bool {
	for i := 1; i < len(node.LeaseHistory); i++ {
		gap := node.LeaseHistory[i].Timestamp.Sub(node.LeaseHistory[i-1].Timestamp)
		if gap > 2*baseInterval {
			return true
		}
	}
	return false
}

// generateCorrelatedEbpf produces an eBPF event correlated with a NotReady cause.
// - kubelet_restart → syscall "exit", comm "kubelet", 0–2s before the lease gap
// - oom_kill → syscall "kill", comm "oom_reaper", within 1s of transition
// - Other causes do not produce correlated eBPF events.
func (se *SimulationEngine) generateCorrelatedEbpf(node *SimNode, cause NotReadyCause) {
	switch cause {
	case CauseKubeletRestart:
		// eBPF event 0–2 seconds before the lease gap begins (i.e., before current clock)
		offsetMs := se.rng.Intn(2001) // 0..2000 ms
		ts := se.clock.Add(-time.Duration(offsetMs) * time.Millisecond)
		node.EbpfEvents = append(node.EbpfEvents, SimEbpfEvent{
			Timestamp:  ts,
			PID:        uint32(se.rng.Intn(32000) + 1),
			PPID:       uint32(se.rng.Intn(32000) + 1),
			Comm:       "kubelet",
			Syscall:    "exit",
			CgroupPath: fmt.Sprintf("/sys/fs/cgroup/kubepods/%s", node.Name),
			NodeName:   node.Name,
			Namespace:  node.Namespace,
		})
	case CauseOOMKill:
		// eBPF event within 1 second of the transition time
		offsetMs := se.rng.Intn(1001) // 0..1000 ms
		// Randomly before or after
		if se.rng.Intn(2) == 0 {
			offsetMs = -offsetMs
		}
		ts := se.clock.Add(time.Duration(offsetMs) * time.Millisecond)
		node.EbpfEvents = append(node.EbpfEvents, SimEbpfEvent{
			Timestamp:  ts,
			PID:        uint32(se.rng.Intn(32000) + 1),
			PPID:       uint32(se.rng.Intn(32000) + 1),
			Comm:       "oom_reaper",
			Syscall:    "kill",
			CgroupPath: fmt.Sprintf("/sys/fs/cgroup/kubepods/%s", node.Name),
			NodeName:   node.Name,
			Namespace:  node.Namespace,
		})
	default:
		// network_blip and disk_pressure don't produce correlated eBPF events
	}
}

// generateBackgroundEbpf produces a background eBPF "write" event for a lease renewal.
func (se *SimulationEngine) generateBackgroundEbpf(node *SimNode, ts time.Time) {
	node.EbpfEvents = append(node.EbpfEvents, SimEbpfEvent{
		Timestamp:  ts,
		PID:        uint32(se.rng.Intn(32000) + 1),
		PPID:       uint32(se.rng.Intn(32000) + 1),
		Comm:       "kubelet",
		Syscall:    "write",
		CgroupPath: fmt.Sprintf("/sys/fs/cgroup/kubepods/%s", node.Name),
		NodeName:   node.Name,
		Namespace:  node.Namespace,
	})
}

// generateReplacementForkEbpf produces an eBPF "fork" event for a replacement node
// within 1 second of the node's first lease renewal.
func (se *SimulationEngine) generateReplacementForkEbpf(node *SimNode, firstLeaseTime time.Time) {
	offsetMs := se.rng.Intn(1001) // 0..1000 ms
	ts := firstLeaseTime.Add(time.Duration(offsetMs) * time.Millisecond)
	node.EbpfEvents = append(node.EbpfEvents, SimEbpfEvent{
		Timestamp:  ts,
		PID:        uint32(se.rng.Intn(32000) + 1),
		PPID:       uint32(se.rng.Intn(32000) + 1),
		Comm:       "kubelet",
		Syscall:    "fork",
		CgroupPath: fmt.Sprintf("/sys/fs/cgroup/kubepods/%s", node.Name),
		NodeName:   node.Name,
		Namespace:  node.Namespace,
	})
}

// Feature: realistic-data-and-visualizations
// Task 6.1: File segmentation — split simulation output into time-windowed JSON files

// SegmentLeaseFiles splits all node lease histories into time-windowed files
// in LeasesByNamespace format (map[string][]LeasePoint).
func (se *SimulationEngine) SegmentLeaseFiles() []LeaseFileOutput {
	window := se.config.FileSegmentWindow
	numFiles := int(math.Ceil(float64(se.config.Duration) / float64(window)))
	if numFiles < 1 {
		numFiles = 1
	}

	files := make([]LeaseFileOutput, numFiles)
	for i := 0; i < numFiles; i++ {
		windowStart := se.startTime.Add(time.Duration(i) * window)
		filename := fmt.Sprintf("leases%s.json", windowStart.Format("20060102T150405"))
		files[i] = LeaseFileOutput{
			Filename: filename,
			Data:     make(map[string][]LeasePoint),
		}
	}

	// Distribute leases into files based on their timestamp
	for _, node := range se.nodes {
		for _, lease := range node.LeaseHistory {
			elapsed := lease.Timestamp.Sub(se.startTime)
			idx := int(elapsed / window)
			if idx >= numFiles {
				idx = numFiles - 1
			}
			if idx < 0 {
				idx = 0
			}
			// LeasePoint: X is the index within the namespace array, Y is timestamp in ms
			ns := lease.Namespace
			x := len(files[idx].Data[ns])
			files[idx].Data[ns] = append(files[idx].Data[ns], LeasePoint{
				X: x,
				Y: lease.Timestamp.UnixNano() / int64(time.Millisecond),
			})
		}
	}

	return files
}

// SegmentEbpfFiles splits all node eBPF events into time-windowed files.
func (se *SimulationEngine) SegmentEbpfFiles() []EbpfFileOutput {
	window := se.config.FileSegmentWindow
	numFiles := int(math.Ceil(float64(se.config.Duration) / float64(window)))
	if numFiles < 1 {
		numFiles = 1
	}

	files := make([]EbpfFileOutput, numFiles)
	for i := 0; i < numFiles; i++ {
		windowStart := se.startTime.Add(time.Duration(i) * window)
		filename := fmt.Sprintf("ebpf-leases%s.json", windowStart.Format("20060102T150405"))
		files[i] = EbpfFileOutput{
			Filename: filename,
		}
	}

	for _, node := range se.nodes {
		for _, evt := range node.EbpfEvents {
			elapsed := evt.Timestamp.Sub(se.startTime)
			idx := int(elapsed / window)
			if idx >= numFiles {
				idx = numFiles - 1
			}
			if idx < 0 {
				idx = 0
			}
			files[idx].Events = append(files[idx].Events, evt)
		}
	}

	return files
}

// Task 6.2: Manifest generation

// GenerateManifest produces a chronologically ordered list of filenames from
// the given file outputs.
func GenerateLeaseManifest(files []LeaseFileOutput) []string {
	manifest := make([]string, len(files))
	for i, f := range files {
		manifest[i] = f.Filename
	}
	return manifest
}

// GenerateEbpfManifest produces a chronologically ordered list of eBPF filenames.
func GenerateEbpfManifest(files []EbpfFileOutput) []string {
	manifest := make([]string, len(files))
	for i, f := range files {
		manifest[i] = f.Filename
	}
	return manifest
}
