# Requirements Document

## Introduction

Earthworm is a Kubernetes heartbeat monitoring tool that uses eBPF probes and a React-based visualizer. The current mock data generator produces flat, unrealistic data (50 nodes all "Ready" simultaneously) and the visualizer is limited to a single LineChart. This spec covers two immediate enhancements — a realistic mock data generator and richer visualization components — plus documents a roadmap of future capabilities (real eBPF integration, advanced alerting) that will be addressed in separate specs.

## Glossary

- **Mock_Data_Generator**: The Go module (`src/kubernetes/mock_ebpf.go`) responsible for producing simulated Kubernetes node, lease, and eBPF event data
- **Visualizer**: The React/TypeScript frontend application (`src/heartbeat-visualizer/`) that renders heartbeat data as interactive charts
- **Heatmap_View**: A grid visualization where rows represent nodes and columns represent time buckets, with cells color-coded by node health status
- **Timeline_View**: A swimlane-style visualization showing per-node health events as horizontal bars across a shared time axis
- **Sparkline**: A small, inline line chart embedded in a table cell showing a node's recent heartbeat interval trend
- **Heartbeat_Interval**: The time in milliseconds between consecutive lease renewals for a given node
- **Lease_Renewal**: A Kubernetes node lease update that signals the node is alive; default period is 10 seconds
- **Node_Health_Status**: One of Ready, NotReady, or Unknown, derived from lease renewal gaps and condition flags
- **Anomaly**: A heartbeat gap exceeding the configured warning threshold, indicating potential node trouble
- **eBPF_Event**: A simulated (or real) kernel-level event captured by eBPF probes, including syscall type, PID, comm, and timestamp
- **Cluster_Scenario**: A predefined simulation profile (e.g., rolling deployment, network partition) that drives the Mock_Data_Generator's behavior
- **Interval_Histogram**: A bar chart showing the frequency distribution of heartbeat intervals across all nodes

## Requirements

---

### Requirement 1: Realistic Node State Simulation

**User Story:** As a developer testing Earthworm, I want mock nodes that transition between Ready, NotReady, and Unknown states with realistic causes, so that I can validate the visualizer against production-like scenarios.

#### Acceptance Criteria

1. WHEN the Mock_Data_Generator initializes a cluster, THE Mock_Data_Generator SHALL assign each node a health profile that determines its probability of transitioning to NotReady within a given time window
2. WHEN a node transitions to NotReady, THE Mock_Data_Generator SHALL associate the transition with a cause (one of: network_blip, oom_kill, disk_pressure, kubelet_restart)
3. WHILE a node is in NotReady state, THE Mock_Data_Generator SHALL keep the node NotReady for a duration between 5 seconds and 120 seconds before returning to Ready, based on the cause type
4. THE Mock_Data_Generator SHALL produce a cluster where at least 5% and no more than 20% of nodes experience at least one NotReady transition over a 1-hour simulated window
5. IF a node's health profile specifies "stable", THEN THE Mock_Data_Generator SHALL keep that node in Ready state for the entire simulation duration

---

### Requirement 2: Variable Lease Renewal Intervals

**User Story:** As a developer, I want simulated lease renewals that vary per node rather than arriving at a fixed cadence, so that the visualizer can display realistic heartbeat drift patterns.

#### Acceptance Criteria

1. THE Mock_Data_Generator SHALL produce lease renewals with a base interval of 10 seconds per node
2. WHEN generating lease timestamps for a node, THE Mock_Data_Generator SHALL apply a per-node jitter drawn from a normal distribution with a configurable standard deviation (default: 500 milliseconds)
3. WHILE a node is in NotReady state, THE Mock_Data_Generator SHALL produce a gap in lease renewals matching the NotReady duration (no lease events during the outage)
4. WHEN a node has a "drifting" health profile, THE Mock_Data_Generator SHALL increase that node's base interval by up to 3 seconds over the simulation window to simulate gradual degradation
5. THE Mock_Data_Generator SHALL ensure all generated lease timestamps are monotonically increasing per node

---

### Requirement 3: Rolling Deployment Burst Patterns

**User Story:** As a developer, I want the mock data to simulate rolling deployment scenarios where nodes drain and new nodes join, so that I can test how the visualizer handles cluster topology changes.

#### Acceptance Criteria

1. WHEN a Cluster_Scenario of type "rolling_deployment" is active, THE Mock_Data_Generator SHALL simulate a configurable number of nodes (default: 5) transitioning to NotReady in sequence with a stagger interval of 30 seconds between each
2. WHEN a node is drained during a rolling deployment, THE Mock_Data_Generator SHALL stop generating lease renewals for that node and introduce a replacement node with a new name after a configurable delay (default: 15 seconds)
3. THE Mock_Data_Generator SHALL ensure the total node count remains within ±2 of the configured cluster size during a rolling deployment
4. WHEN a replacement node joins, THE Mock_Data_Generator SHALL generate an initial burst of 3 lease renewals within the first 5 seconds to simulate rapid initial heartbeating

---

### Requirement 4: Multi-Namespace Workload Profiles

**User Story:** As a developer, I want mock data spanning multiple namespaces with different workload characteristics, so that the visualizer can display namespace-level patterns.

#### Acceptance Criteria

1. THE Mock_Data_Generator SHALL produce data for at least 3 namespaces (e.g., "kube-system", "production", "staging") with distinct workload profiles
2. WHEN generating data for the "kube-system" namespace, THE Mock_Data_Generator SHALL use a stable profile where fewer than 2% of nodes experience NotReady transitions
3. WHEN generating data for the "staging" namespace, THE Mock_Data_Generator SHALL use a volatile profile where 10–20% of nodes experience NotReady transitions
4. THE Mock_Data_Generator SHALL distribute nodes across namespaces according to a configurable ratio (default: 20% kube-system, 50% production, 30% staging)

---

### Requirement 5: Correlated eBPF Events

**User Story:** As a developer, I want eBPF events that correlate with lease gaps so that the visualizer can demonstrate cause-and-effect relationships between kernel events and missed heartbeats.

#### Acceptance Criteria

1. WHEN a node transitions to NotReady due to kubelet_restart, THE Mock_Data_Generator SHALL generate an eBPF_Event with syscall "exit" for the kubelet process 0–2 seconds before the lease gap begins
2. WHEN a node transitions to NotReady due to oom_kill, THE Mock_Data_Generator SHALL generate an eBPF_Event with comm "oom_reaper" and syscall "kill" at the time of the transition
3. WHEN a replacement node starts during a rolling deployment, THE Mock_Data_Generator SHALL generate an eBPF_Event with syscall "fork" for the kubelet process within 1 second of the first lease renewal
4. THE Mock_Data_Generator SHALL generate background eBPF_Events (syscall "write" for lease renewals) at a rate proportional to the number of active nodes, with one event per lease renewal
5. FOR ALL generated eBPF_Events, THE Mock_Data_Generator SHALL include valid PID, PPID, comm, cgroup_path, and timestamp fields

---

### Requirement 6: Extended Time-Series Data

**User Story:** As a developer, I want mock data spanning hours or days rather than a few snapshots, so that the visualizer can be tested with realistic data volumes.

#### Acceptance Criteria

1. THE Mock_Data_Generator SHALL accept a configurable simulation duration (default: 4 hours, maximum: 7 days)
2. THE Mock_Data_Generator SHALL output data in the existing JSON format compatible with the Visualizer's LeasesByNamespace type
3. WHEN the simulation duration exceeds 1 hour, THE Mock_Data_Generator SHALL inject at least one Cluster_Scenario event (e.g., rolling_deployment, network_partition) per hour
4. THE Mock_Data_Generator SHALL produce a manifest file listing all generated data files in chronological order, compatible with the existing manifest format
5. THE Mock_Data_Generator SHALL generate data files segmented into configurable time windows (default: 5 minutes per file) to match the Visualizer's incremental loading pattern

---

### Requirement 7: Node Health Heatmap View

**User Story:** As an operator, I want a heatmap showing all nodes as rows with color-coded health over time, so that I can spot cluster-wide patterns at a glance.

#### Acceptance Criteria

1. THE Visualizer SHALL render a Heatmap_View where each row represents a node and each column represents a configurable time bucket (default: 30 seconds)
2. THE Visualizer SHALL color each heatmap cell using the existing color scheme: green for Ready, yellow for warning-level gaps, red for critical-level gaps
3. WHEN the user hovers over a heatmap cell, THE Visualizer SHALL display a tooltip showing the node name, time range, health status, and heartbeat count within that bucket
4. THE Visualizer SHALL sort heatmap rows by worst health status (most critical nodes at the top) by default, with an option to sort alphabetically
5. WHEN the dataset contains more than 30 nodes, THE Visualizer SHALL provide vertical scrolling for the heatmap while keeping the time axis header fixed

---

### Requirement 8: Swimlane Timeline View

**User Story:** As an operator, I want a timeline view with swimlanes per node so that I can spot temporal patterns and correlations across 50+ nodes.

#### Acceptance Criteria

1. THE Visualizer SHALL render a Timeline_View with one horizontal swimlane per node, sharing a common time axis
2. THE Visualizer SHALL render each swimlane as a continuous bar colored by Node_Health_Status (green for Ready, red for NotReady, gray for Unknown)
3. WHEN the user clicks on a gap segment in a swimlane, THE Visualizer SHALL display a detail panel showing the gap duration, cause (if available from eBPF correlation), and surrounding lease timestamps
4. THE Visualizer SHALL support horizontal zoom and pan on the shared time axis, consistent with the existing chart zoom/pan behavior
5. WHEN eBPF overlay is enabled, THE Visualizer SHALL render eBPF_Event markers as icons on the corresponding node's swimlane at the correct timestamp

---

### Requirement 9: Sparklines in Node Table

**User Story:** As an operator, I want sparklines in a node summary table so that I can quickly assess each node's recent heartbeat trend without switching views.

#### Acceptance Criteria

1. THE Visualizer SHALL render a node summary table with columns: Node Name, Namespace, Current Status, Last Heartbeat, and a Sparkline column
2. THE Visualizer SHALL render each Sparkline as a small line chart (approximately 120 pixels wide, 30 pixels tall) showing the last 20 heartbeat intervals for that node
3. WHEN a node's sparkline contains an interval exceeding the warning threshold, THE Visualizer SHALL render that segment of the sparkline in the warning color
4. WHEN a node's sparkline contains an interval exceeding the critical threshold, THE Visualizer SHALL render that segment of the sparkline in the critical/death color
5. THE Visualizer SHALL update sparklines in real-time as new WebSocket heartbeat events arrive

---

### Requirement 10: Automatic Anomaly Highlighting

**User Story:** As an operator, I want anomalies to be visually highlighted across all views so that gaps draw my attention without manual inspection.

#### Acceptance Criteria

1. WHEN an Anomaly is detected in the data, THE Visualizer SHALL highlight the corresponding region in the Heatmap_View with a pulsing border effect
2. WHEN an Anomaly is detected in the data, THE Visualizer SHALL highlight the corresponding gap segment in the Timeline_View with increased opacity and a contrasting border
3. THE Visualizer SHALL display an anomaly summary badge showing the total count of detected anomalies, positioned above the active chart view
4. WHEN the user clicks the anomaly summary badge, THE Visualizer SHALL scroll to and zoom into the most recent anomaly in the active view
5. THE Visualizer SHALL use the existing `getAnomalies()` function from chartUtils to detect anomalies, extended to support the new view types

---

### Requirement 11: Heartbeat Interval Histogram

**User Story:** As an operator, I want a histogram of heartbeat intervals so that I can see the distribution and identify outliers across the cluster.

#### Acceptance Criteria

1. THE Visualizer SHALL render an Interval_Histogram showing the frequency distribution of all Heartbeat_Intervals in the current dataset
2. THE Visualizer SHALL use configurable bin widths (default: 1 second) for the histogram
3. WHEN the user selects a specific namespace from the existing namespace filter, THE Visualizer SHALL filter the histogram to show only intervals from that namespace
4. THE Visualizer SHALL visually distinguish bins that fall within the warning range (yellow) and critical range (red) from normal bins (green)
5. WHEN the user hovers over a histogram bin, THE Visualizer SHALL display a tooltip showing the interval range, count, and percentage of total intervals

---

### Requirement 12: View Switching and Layout

**User Story:** As an operator, I want to switch between the existing line chart, heatmap, timeline, and histogram views so that I can choose the best visualization for my current task.

#### Acceptance Criteria

1. THE Visualizer SHALL provide a view selector control with options: Line Chart (existing), Heatmap, Timeline, Histogram, and Table (with sparklines)
2. WHEN the user selects a view, THE Visualizer SHALL render the selected view using the same underlying dataset without re-fetching data
3. THE Visualizer SHALL preserve the current zoom/pan state when switching between views that share a time axis (Line Chart, Heatmap, Timeline)
4. THE Visualizer SHALL default to the Line Chart view to maintain backward compatibility with the existing user experience
5. WHEN the Visualizer loads, THE Visualizer SHALL render the node summary table with sparklines below the active chart view as a persistent panel

---

## Roadmap (Future Specs — Not Implemented Here)

### Future Requirement A: Real eBPF Integration

**User Story:** As a platform engineer, I want Earthworm to capture actual kernel events via eBPF probes attached to kubelet, so that I can monitor real cluster heartbeat behavior.

#### Planned Capabilities

1. THE eBPF_Loader SHALL attach BPF programs to kubelet lease renewal syscalls using cilium/ebpf Go library
2. THE eBPF_Loader SHALL track process lifecycle events (fork, exec, exit) for kubelet and container runtime processes
3. THE eBPF_Loader SHALL capture network-level events (TCP retransmits, connection resets) relevant to API server communication
4. THE eBPF_Loader SHALL correlate cgroup paths to Kubernetes pods using the existing PodInfo correlation logic
5. THE eBPF program in `src/ebpf/heartbeat.c` SHALL be extended with actual BPF programs for lease write interception and a Go userspace loader

### Future Requirement B: Advanced Alerting and Dashboards

**User Story:** As an SRE, I want predictive anomaly detection and configurable alert routing so that I can proactively respond to node health degradation.

#### Planned Capabilities

1. THE Anomaly_Detector SHALL use historical heartbeat patterns to predict nodes likely to go NotReady within the next 5 minutes
2. THE Alert_System SHALL support custom alert rules and per-node/per-namespace thresholds configurable via a dashboard UI
3. THE Dashboard SHALL support persistence (save/load layouts) and sharing via URL
4. THE Alert_Dispatcher SHALL integrate with PagerDuty, Slack, and OpsGenie via configurable webhook endpoints
