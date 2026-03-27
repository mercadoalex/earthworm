# Requirements Document

## Introduction

Earthworm's core differentiator is real eBPF-based kernel observability for Kubernetes clusters. This feature replaces the current mock/simulated eBPF data with actual BPF programs that attach to the Linux kernel, enabling causal chain analysis (tracing from kernel syscall → process → container → pod → node health), predictive failure detection, zero-instrumentation observability, and live kernel-level replay. The system uses the cilium/ebpf Go library for userspace loading and BPF C programs compiled with clang/LLVM. It requires Linux kernel 5.8+ with CAP_BPF capability.

## Glossary

- **BPF_Loader**: The Go userspace component that compiles, loads, and manages eBPF programs using the cilium/ebpf library
- **Probe_Manager**: The component that attaches BPF programs to kernel tracepoints and kprobes, manages their lifecycle, and reads events from BPF maps
- **Syscall_Tracer**: The BPF program that traces syscalls (write, sendto, read, recvmsg) made by kubelet and container runtime processes
- **Process_Monitor**: The BPF program that captures process lifecycle events (fork, exec, exit) for kubelet and container runtimes
- **Network_Probe**: The BPF program that captures TCP-level events (retransmits, resets, latency) between nodes and the API server
- **Cgroup_Resolver**: The component that maps cgroup IDs from eBPF events to Kubernetes pod/container identities using the kubelet API or /proc filesystem
- **Causal_Chain_Builder**: The component that correlates eBPF syscall traces, process events, and network events with lease renewal timing to construct causal chains explaining node state transitions
- **Event_Ring_Buffer**: The BPF ring buffer map used to stream eBPF events from kernel space to userspace with minimal overhead
- **Prediction_Engine**: The component that analyzes patterns in eBPF event streams (syscall latency trends, retransmit spikes, memory pressure signals) to predict imminent node failures
- **Replay_Store**: The storage component that persists eBPF event streams with sufficient detail for post-mortem frame-by-frame replay of kernel activity during outages
- **Kernel_Event**: A structured event emitted by a BPF program containing timestamp, PID, comm, syscall, latency, cgroup ID, and node identity
- **Causal_Chain**: An ordered sequence of Kernel_Events that explains why a node transitioned to NotReady, linking syscall-level activity to pod/container identity and node health
- **WebSocket_Hub**: The existing Earthworm WebSocket hub (`src/server/ws.go`) that broadcasts events to connected visualizer clients
- **Store_Interface**: The existing Earthworm Store interface (`src/server/store.go`) for persisting heartbeat and event data
- **Anomaly_Detector**: The existing Earthworm anomaly detection component (`src/server/anomaly.go`) that evaluates heartbeat gaps against thresholds

## Requirements

### Requirement 1: BPF Program Compilation and Loading

**User Story:** As a cluster operator, I want Earthworm to compile and load eBPF programs into the kernel automatically, so that I can get kernel-level observability without manual BPF toolchain setup.

#### Acceptance Criteria

1. WHEN Earthworm starts on a Linux host with kernel 5.8+, THE BPF_Loader SHALL compile the BPF C programs in `src/ebpf/` using clang/LLVM and load the resulting bytecode into the kernel via the cilium/ebpf library
2. WHEN Earthworm starts on a host with a kernel version below 5.8, THE BPF_Loader SHALL log a descriptive error message identifying the minimum required kernel version and continue operating without eBPF functionality
3. WHEN Earthworm starts on a host without CAP_BPF or root capability, THE BPF_Loader SHALL log a descriptive error message identifying the missing capability and continue operating without eBPF functionality
4. WHEN Earthworm shuts down, THE BPF_Loader SHALL detach all loaded BPF programs and close all BPF map file descriptors within 5 seconds
5. IF a BPF program fails to load due to a verifier rejection, THEN THE BPF_Loader SHALL log the verifier output and continue loading remaining BPF programs
6. THE BPF_Loader SHALL use `bpf2go` code generation from the cilium/ebpf library to produce Go bindings for each BPF program at build time

### Requirement 2: Kubelet Syscall Tracing

**User Story:** As a cluster operator, I want to trace kubelet's lease renewal syscalls at the kernel level, so that I can see exactly when and why lease renewals succeed or fail.

#### Acceptance Criteria

1. WHILE the Syscall_Tracer is loaded, THE Syscall_Tracer SHALL capture write() and sendto() syscalls made by processes with comm name "kubelet" that target the API server socket
2. WHEN a traced syscall completes, THE Syscall_Tracer SHALL emit a Kernel_Event containing the syscall name, entry timestamp, exit timestamp, return value, PID, comm, and cgroup ID to the Event_Ring_Buffer
3. WHEN a traced write() or sendto() syscall takes longer than 1 second to complete, THE Syscall_Tracer SHALL set a "slow_syscall" flag on the emitted Kernel_Event
4. THE Syscall_Tracer SHALL filter traced syscalls to only kubelet and container runtime processes (containerd, cri-o) by matching the comm field, to minimize overhead on unrelated processes
5. WHILE the Syscall_Tracer is loaded, THE Syscall_Tracer SHALL maintain a per-CPU count of traced syscalls accessible via a BPF array map for monitoring overhead

### Requirement 3: Process Lifecycle Monitoring

**User Story:** As a cluster operator, I want to track when kubelet and container runtime processes start, fork, or exit, so that I can correlate process lifecycle events with node health changes.

#### Acceptance Criteria

1. WHILE the Process_Monitor is loaded, THE Process_Monitor SHALL capture fork, exec, and exit events for processes with comm name matching "kubelet", "containerd", or "cri-o"
2. WHEN a monitored process exits, THE Process_Monitor SHALL emit a Kernel_Event containing the exit code, PID, PPID, comm, and cgroup ID to the Event_Ring_Buffer
3. WHEN a monitored process forks a child, THE Process_Monitor SHALL emit a Kernel_Event containing the parent PID, child PID, and comm to the Event_Ring_Buffer
4. IF kubelet exits with a non-zero exit code, THEN THE Process_Monitor SHALL set a "critical_exit" flag on the emitted Kernel_Event

### Requirement 4: Network Event Capture

**User Story:** As a cluster operator, I want to capture TCP-level events between nodes and the API server, so that I can identify network issues that cause missed heartbeats.

#### Acceptance Criteria

1. WHILE the Network_Probe is loaded, THE Network_Probe SHALL capture TCP retransmit events on connections between the node and the API server IP address
2. WHILE the Network_Probe is loaded, THE Network_Probe SHALL capture TCP connection reset events on connections to the API server
3. WHEN a TCP retransmit or reset event occurs, THE Network_Probe SHALL emit a Kernel_Event containing the source IP, destination IP, source port, destination port, and event type to the Event_Ring_Buffer
4. WHILE the Network_Probe is loaded, THE Network_Probe SHALL track per-connection round-trip time estimates and emit a Kernel_Event when the RTT exceeds 500 milliseconds

### Requirement 5: Cgroup-to-Pod Correlation

**User Story:** As a cluster operator, I want eBPF events to be attributed to specific Kubernetes pods and containers, so that I can identify which workload caused resource contention.

#### Acceptance Criteria

1. THE Cgroup_Resolver SHALL build a mapping from cgroup IDs to Kubernetes pod name, namespace, container name, and node name by querying the kubelet API or reading /proc filesystem entries
2. WHEN a new pod is created or an existing pod is deleted, THE Cgroup_Resolver SHALL update the cgroup-to-pod mapping within 5 seconds
3. WHEN a Kernel_Event is received from the Event_Ring_Buffer, THE Cgroup_Resolver SHALL enrich the event with the corresponding pod name, namespace, and container name before forwarding the event to the Causal_Chain_Builder
4. IF a cgroup ID in a Kernel_Event does not match any known pod, THEN THE Cgroup_Resolver SHALL label the event as "host-level" and include the process comm name as the workload identifier
5. THE Cgroup_Resolver SHALL cache the cgroup-to-pod mapping in memory and refresh the cache at a configurable interval (default: 30 seconds)

### Requirement 6: Causal Chain Analysis

**User Story:** As a cluster operator, I want Earthworm to automatically trace the causal chain from kernel syscall to node NotReady, so that I can understand the root cause of node failures without manual investigation.

#### Acceptance Criteria

1. WHEN a node transitions to NotReady, THE Causal_Chain_Builder SHALL construct a Causal_Chain by correlating Kernel_Events from the preceding 120 seconds that share the same node identity
2. THE Causal_Chain_Builder SHALL order events in the Causal_Chain chronologically and link them by causal relationship: syscall blocked → process affected → container impacted → pod disrupted → lease renewal missed → node NotReady
3. WHEN a Causal_Chain is constructed, THE Causal_Chain_Builder SHALL generate a human-readable summary string (e.g., "node X went NotReady because kubelet's lease renewal write() syscall was blocked for 12s by a disk I/O stall caused by container Y doing heavy logging")
4. WHEN a Causal_Chain is constructed, THE Causal_Chain_Builder SHALL broadcast the chain to connected visualizer clients via the WebSocket_Hub using a new "causal_chain" message type
5. THE Causal_Chain_Builder SHALL store completed Causal_Chains via the Store_Interface for historical query
6. IF no correlated Kernel_Events are found for a NotReady transition, THEN THE Causal_Chain_Builder SHALL generate a Causal_Chain with a single "unknown_cause" entry and the available lease timing data

### Requirement 7: Predictive Failure Detection

**User Story:** As a cluster operator, I want Earthworm to predict node failures 30-60 seconds before they happen, so that I can take preventive action before workloads are disrupted.

#### Acceptance Criteria

1. WHILE eBPF probes are active, THE Prediction_Engine SHALL continuously analyze sliding windows of Kernel_Events per node for behavioral patterns: increasing write() syscall latencies, rising TCP retransmit counts, and growing memory pressure signals
2. WHEN the Prediction_Engine detects a pattern matching historical node failure signatures, THE Prediction_Engine SHALL emit a "prediction" alert with a confidence score between 0.0 and 1.0 and the predicted time-to-failure in seconds
3. THE Prediction_Engine SHALL emit prediction alerts with a target lead time of 30 to 60 seconds before the predicted NotReady transition
4. WHEN a prediction alert is emitted, THE Prediction_Engine SHALL broadcast the alert to connected visualizer clients via the WebSocket_Hub using a new "prediction" message type
5. THE Prediction_Engine SHALL track prediction accuracy by comparing emitted predictions against actual NotReady transitions and expose accuracy metrics (true positive rate, false positive rate) via an API endpoint
6. THE Prediction_Engine SHALL use behavioral pattern matching from kernel-level signals rather than static threshold comparisons for failure prediction

### Requirement 8: Event Ring Buffer and Streaming

**User Story:** As a cluster operator, I want eBPF events to stream in real time to the Earthworm visualizer with minimal kernel overhead, so that I can observe kernel activity as it happens.

#### Acceptance Criteria

1. THE Probe_Manager SHALL read Kernel_Events from the Event_Ring_Buffer using the cilium/ebpf library's ring buffer reader with a configurable poll interval (default: 100 milliseconds)
2. WHEN a Kernel_Event is read from the Event_Ring_Buffer, THE Probe_Manager SHALL forward the event to the Cgroup_Resolver for enrichment and then to the WebSocket_Hub for broadcast within 500 milliseconds of the kernel event timestamp
3. IF the Event_Ring_Buffer is full and events are being dropped, THEN THE Probe_Manager SHALL increment a "dropped_events" counter and log a warning at most once per 10 seconds
4. THE Event_Ring_Buffer SHALL be sized at 256 KB per CPU by default, configurable via an environment variable
5. WHEN a Kernel_Event is broadcast via the WebSocket_Hub, THE Probe_Manager SHALL use a new "ebpf_event" message type containing the enriched event data including pod/container attribution

### Requirement 9: Live Kernel-Level Replay

**User Story:** As a cluster operator, I want to click on a gap in the timeline view and see a frame-by-frame replay of every syscall, context switch, and network packet during the outage, so that I can perform detailed post-mortem analysis.

#### Acceptance Criteria

1. THE Replay_Store SHALL persist all Kernel_Events with full detail (timestamp, PID, comm, syscall, latency, cgroup ID, return value, network metadata) for a configurable retention period (default: 24 hours)
2. WHEN a user requests a replay for a time range on a specific node, THE Replay_Store SHALL return all Kernel_Events for that node within the requested time range, ordered chronologically
3. THE Replay_Store SHALL support querying events by node name, time range, event type (syscall, process, network), and pod name
4. WHEN a replay is requested via the API, THE Replay_Store SHALL return events in a paginated response with a default page size of 1000 events
5. THE Replay_Store SHALL store events via the existing Store_Interface, extending the interface with methods for eBPF event persistence and querying
6. WHEN a replay request covers a time range with more than 10,000 events, THE Replay_Store SHALL support server-side filtering by event type and minimum latency threshold to reduce response size

### Requirement 10: Zero-Instrumentation Operation

**User Story:** As a cluster operator, I want Earthworm's eBPF observability to work without sidecars, agents, or code changes in workloads, so that I can deploy it in air-gapped or locked-down clusters.

#### Acceptance Criteria

1. THE BPF_Loader SHALL operate by attaching BPF programs to kernel tracepoints and kprobes only, requiring no modifications to kubelet, container runtimes, or application workloads
2. THE BPF_Loader SHALL require only the Earthworm binary and the compiled BPF object files for deployment, with no additional sidecar containers or DaemonSet agents
3. WHILE eBPF probes are active, THE Probe_Manager SHALL consume less than 2% additional CPU overhead on the host, measured as the difference in CPU usage with and without probes loaded
4. WHILE eBPF probes are active, THE Probe_Manager SHALL consume less than 50 MB of additional memory on the host for BPF maps and userspace event processing

### Requirement 11: Integration with Existing Earthworm Architecture

**User Story:** As a developer, I want eBPF events to flow through the existing Earthworm server architecture, so that the visualizer and anomaly detector can use kernel-level data without architectural changes.

#### Acceptance Criteria

1. WHEN a Kernel_Event is enriched by the Cgroup_Resolver, THE Probe_Manager SHALL broadcast the event via the existing WebSocket_Hub using the same connection and message envelope format (WSMessage with type and payload fields)
2. THE Probe_Manager SHALL store enriched Kernel_Events via the existing Store_Interface, extending the Heartbeat struct or adding a new event type to the store
3. WHEN the Anomaly_Detector evaluates a heartbeat gap, THE Anomaly_Detector SHALL include correlated Kernel_Events from the preceding 120 seconds in the Alert payload sent to the WebSocket_Hub
4. THE BPF_Loader SHALL be initialized in `src/server/main.go` alongside the existing store, hub, and anomaly detector initialization, controlled by a `--ebpf` CLI flag
5. WHEN the `--ebpf` flag is not set or the host does not support eBPF, THE server SHALL fall back to the existing mock/simulated eBPF behavior without errors

### Requirement 12: Visualizer eBPF Causal Overlay

**User Story:** As a cluster operator, I want the timeline and heatmap views to show eBPF causal overlays, so that I can visually correlate kernel events with node health transitions.

#### Acceptance Criteria

1. WHEN a Causal_Chain is received via WebSocket, THE timeline view SHALL render causal chain markers on the corresponding node's swimlane, connecting related events with directional arrows
2. WHEN a user clicks on a NotReady segment in the timeline view, THE timeline view SHALL display the associated Causal_Chain summary and list of Kernel_Events in the gap detail panel
3. WHEN a prediction alert is received via WebSocket, THE heatmap view SHALL highlight the predicted node cell with a distinct visual indicator (pulsing border) before the node transitions to NotReady
4. THE visualizer SHALL add a new "ebpf_event" handler to the existing WebSocket message processing in `useWebSocket.ts` to parse and route enriched Kernel_Events to the appropriate view components
5. WHEN a user clicks on a gap in the timeline view, THE visualizer SHALL offer a "Replay" button that fetches the detailed Kernel_Event stream from the Replay_Store API and displays a frame-by-frame event list

### Requirement 13: BPF Program Extension for Real Probes

**User Story:** As a developer, I want the existing `src/ebpf/heartbeat.c` skeleton to be extended with real BPF programs for syscall tracing, process monitoring, and network capture, so that the eBPF integration produces actual kernel data.

#### Acceptance Criteria

1. THE Syscall_Tracer BPF program SHALL attach to the `sys_enter_write`, `sys_enter_sendto`, `sys_exit_write`, and `sys_exit_sendto` tracepoints to capture kubelet lease renewal syscalls with entry and exit timestamps
2. THE Process_Monitor BPF program SHALL attach to the `sched_process_fork`, `sched_process_exec`, and `sched_process_exit` tracepoints to capture process lifecycle events
3. THE Network_Probe BPF program SHALL attach to the `tcp_retransmit_skb` and `tcp_reset` kprobes to capture TCP retransmit and reset events
4. THE BPF programs SHALL use BPF ring buffer maps (BPF_MAP_TYPE_RINGBUF) instead of perf event arrays for event delivery to userspace, for lower overhead and simpler consumption
5. THE BPF programs SHALL use BPF CO-RE (Compile Once, Run Everywhere) via BTF type information to ensure portability across kernel versions 5.8+
6. FOR ALL Kernel_Events emitted by BPF programs, parsing the event in Go and formatting the event back to binary SHALL produce an equivalent byte sequence (round-trip property)

### Requirement 14: Kubernetes Deployment and Installation

**User Story:** As a cluster operator, I want to install Earthworm on my Kubernetes cluster with a single command (Helm or kubectl), so that I can get eBPF observability running without complex manual setup.

#### Acceptance Criteria

1. THE project SHALL provide a Helm chart (`deploy/helm/earthworm/`) that installs all Earthworm components into a configurable namespace (default: `earthworm-system`)
2. THE Helm chart SHALL create an `earthworm-agent` DaemonSet that runs the eBPF probe binary on every node (or a configurable subset via nodeSelector/tolerations), with `hostPID: true` and the minimum required capabilities (`CAP_BPF`, `CAP_SYS_ADMIN`, `CAP_PERFMON`)
3. THE Helm chart SHALL create an `earthworm-server` Deployment (1 replica by default, configurable) running the Go server with WebSocket hub, anomaly detector, prediction engine, and Store interface
4. THE Helm chart SHALL create an `earthworm-ui` Deployment serving the React visualizer as a static frontend behind an nginx container
5. THE Helm chart SHALL create a ServiceAccount with RBAC ClusterRole granting read access to `coordination.k8s.io/leases`, `v1/nodes`, `v1/pods`, and `v1/namespaces` resources
6. THE Helm chart SHALL create a ConfigMap with configurable values: API server address, warning/critical thresholds, store type (memory/redis), ring buffer size, prediction engine toggle, and retention period
7. THE Helm chart SHALL create Services for the server (ClusterIP) and UI (ClusterIP or LoadBalancer, configurable via `values.yaml`)
8. THE project SHALL provide a standalone manifest (`deploy/earthworm.yaml`) as an alternative to Helm, installable via `kubectl apply -f`
9. THE `earthworm-agent` container image SHALL include the compiled BPF object files and the Go binary, built from a multi-stage Dockerfile that compiles BPF programs with clang/LLVM and the Go binary with CGO_ENABLED=0
10. WHEN the `earthworm-agent` DaemonSet pod starts on a node, THE agent SHALL verify kernel version ≥ 5.8 and BTF availability, log a clear error and enter a degraded "mock-only" mode if requirements are not met, and report its status to the server via the heartbeat API
11. THE Helm chart SHALL support an `--ebpf.enabled=false` value that deploys only the server and UI without the agent DaemonSet, for environments where eBPF is not available

### Requirement 15: Container Image Build and CI

**User Story:** As a developer, I want automated container image builds for all Earthworm components, so that installation artifacts are always up to date and reproducible.

#### Acceptance Criteria

1. THE project SHALL provide a multi-stage Dockerfile (`deploy/docker/Dockerfile.agent`) that compiles BPF C programs with clang-15+, generates Go bindings via bpf2go, and builds the agent binary in a distroless/static final image
2. THE project SHALL provide a Dockerfile (`deploy/docker/Dockerfile.server`) that builds the Go server binary in a distroless/static final image
3. THE project SHALL provide a Dockerfile (`deploy/docker/Dockerfile.ui`) that builds the React visualizer with `npm run build` and serves it via nginx:alpine
4. THE project SHALL provide a `Makefile` with targets: `build-agent`, `build-server`, `build-ui`, `build-all`, `push-all`, `helm-package`, and `deploy`
5. ALL container images SHALL use a consistent tagging scheme: `earthworm/{component}:{git-sha-short}` and `earthworm/{component}:latest`
