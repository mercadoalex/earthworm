# Requirements Document

## Introduction

This feature extends the Earthworm eBPF observability platform with six advanced probe categories: file system I/O tracing, memory pressure signals, DNS resolution tracing, cgroup-level resource accounting, predictive failure detection enhancements, and network policy auditing. These probes generate new event types (event_type 3–8) that flow through the existing ring buffer → agent → server → WebSocket → React pipeline. The existing 120-byte `kernel_event` struct is preserved for backward compatibility; new probe data uses a separate `extended_event` struct with a shared header. All Go-level logic (codec, enrichment, prediction) is testable on macOS; only BPF compilation and loading require Linux 5.8+.

## Glossary

- **Agent**: The Go binary running as a DaemonSet on each Kubernetes node (`src/agent/`), responsible for loading BPF programs, reading the ring buffer, enriching events, and forwarding them to the Server.
- **Server**: The Go HTTP/WebSocket server (`src/server/`) that ingests enriched events, persists them, runs causal chain analysis and prediction, and broadcasts to the Visualizer.
- **Visualizer**: The React application (`src/heartbeat-visualizer/`) that renders live kernel activity, heartbeat charts, and diagnostic views.
- **KernelEvent**: The existing 120-byte binary struct defined in `src/ebpf/headers/common.h` and mirrored in `src/agent/event.go`.
- **ExtendedEvent**: A new variable-length binary struct sharing the same 48-byte common header as KernelEvent but carrying probe-specific payloads for the six new event types.
- **EnrichedEvent**: The JSON representation of a decoded kernel or extended event combined with pod identity from the CgroupResolver.
- **CgroupResolver**: The component in the Agent (`src/agent/cgroup_resolver.go`) that maps cgroup IDs to Kubernetes pod identities.
- **PredictionEngine**: The server-side component (`src/server/prediction.go`) that analyzes event patterns to forecast node failures.
- **CausalChainBuilder**: The server-side component (`src/server/causal_chain.go`) that correlates kernel events into causal chains explaining NotReady transitions.
- **ProbeManager**: The Agent component (`src/agent/probe_manager.go`) that reads events from the BPF ring buffer and dispatches them for enrichment.
- **Ring_Buffer**: The BPF_MAP_TYPE_RINGBUF shared map (`events` in `common.h`) used by all eBPF programs to emit events to userspace.
- **VFS_Probe**: The eBPF program hooking `vfs_read` and `vfs_write` kernel functions for file system I/O tracing.
- **OOM_Probe**: The eBPF program hooking `oom_kill_process` and `mm_page_alloc` failure paths for memory pressure detection.
- **DNS_Probe**: The eBPF program hooking `udp_sendmsg` filtered on destination port 53 for DNS resolution tracing.
- **Cgroup_Accounting_Probe**: The eBPF program using `bpf_get_current_cgroup_id()` with per-cgroup CPU and memory statistics.
- **Security_Socket_Probe**: The eBPF program hooking `security_socket_connect` for network policy auditing.
- **Slow_IO_Threshold**: A configurable duration (default 100ms) above which a file system I/O operation is flagged as slow.
- **DNS_Timeout_Threshold**: A configurable duration (default 5 seconds) above which a DNS lookup is flagged as timed out.
- **Network_Topology_Map**: A server-side data structure tracking pod-to-pod and pod-to-external connections derived from Security_Socket_Probe events.

## Requirements

### Requirement 1: Extended Event Binary Format

**User Story:** As a developer, I want a well-defined binary format for new event types, so that new probes can carry probe-specific payloads without breaking the existing 120-byte KernelEvent layout.

#### Acceptance Criteria

1. THE ExtendedEvent struct SHALL share the same 48-byte common header as KernelEvent (timestamp, pid, ppid, tgid, cgroup_id, comm, event_type).
2. THE ExtendedEvent struct SHALL include a 2-byte `payload_len` field at offset 48 indicating the length of the probe-specific payload that follows.
3. WHEN the Agent receives a ring buffer record with event_type >= 3, THE ProbeManager SHALL decode the record using the ExtendedEvent codec instead of the KernelEvent codec.
4. WHEN the Agent receives a ring buffer record with event_type 0, 1, or 2, THE ProbeManager SHALL decode the record using the existing KernelEvent codec.
5. THE ExtendedEvent codec SHALL support `MarshalBinary` and `UnmarshalBinary` methods mirroring the KernelEvent interface.
6. FOR ALL valid ExtendedEvent structs, encoding via `MarshalBinary` then decoding via `UnmarshalBinary` SHALL produce an ExtendedEvent identical to the original (round-trip property).
7. IF the ring buffer record is shorter than the common header plus `payload_len`, THEN THE ExtendedEvent codec SHALL return a descriptive error containing the actual and expected sizes.

### Requirement 2: File System I/O Probes

**User Story:** As an SRE, I want to trace VFS read and write operations, so that I can detect disk I/O bottlenecks affecting etcd latency and container performance.

#### Acceptance Criteria

1. THE VFS_Probe SHALL attach kprobes to `vfs_read` and `vfs_write` kernel functions on Linux 5.8+.
2. WHEN a `vfs_read` or `vfs_write` call completes, THE VFS_Probe SHALL emit an ExtendedEvent with event_type 3 to the Ring_Buffer.
3. THE VFS_Probe event payload SHALL include the file path (up to 256 bytes), operation latency in nanoseconds, bytes transferred, and cgroup ID.
4. WHEN the operation latency exceeds the Slow_IO_Threshold, THE VFS_Probe SHALL set a `slow_io` flag to 1 in the event payload.
5. THE Slow_IO_Threshold SHALL be configurable via a BPF map with a default value of 100 milliseconds (100,000,000 nanoseconds).
6. THE VFS_Probe SHALL use `bpf_d_path()` helper to resolve the file path from the `struct file` argument.
7. IF `bpf_d_path()` fails or the path exceeds 256 bytes, THEN THE VFS_Probe SHALL set the file path to an empty string and continue emitting the event.
8. THE VFS_Probe SHALL filter operations to monitored cgroups only, using the same `bpf_get_current_cgroup_id()` mechanism as existing probes.

### Requirement 3: Memory Pressure Signals

**User Story:** As an SRE, I want real-time OOM kill notifications and early memory pressure warnings, so that I can respond to memory exhaustion before pods are evicted.

#### Acceptance Criteria

1. THE OOM_Probe SHALL attach a kprobe to `oom_kill_process` on Linux 5.8+.
2. WHEN `oom_kill_process` is invoked, THE OOM_Probe SHALL emit an ExtendedEvent with event_type 4 to the Ring_Buffer.
3. THE OOM_Probe event payload for OOM kills SHALL include the killed process PID, comm (up to 16 bytes), cgroup ID, and OOM score adjustment value.
4. THE OOM_Probe SHALL attach a tracepoint to `mm_page_alloc` failure path (order > 0, gfp_flags indicating allocation failure).
5. WHEN a page allocation failure occurs, THE OOM_Probe SHALL emit an ExtendedEvent with event_type 4 and a `sub_type` field set to 1 (distinguishing allocation failure from OOM kill where sub_type is 0).
6. THE OOM_Probe allocation failure payload SHALL include the requested page order, GFP flags, and cgroup ID.
7. IF the OOM score adjustment value cannot be read from the task struct, THEN THE OOM_Probe SHALL set the OOM score field to -1 and continue emitting the event.

### Requirement 4: DNS Resolution Tracing

**User Story:** As an SRE, I want to trace DNS lookups from pods, so that I can identify slow DNS resolution that silently degrades Kubernetes service discovery.

#### Acceptance Criteria

1. THE DNS_Probe SHALL attach a kprobe to `udp_sendmsg` on Linux 5.8+.
2. WHEN `udp_sendmsg` is called with destination port 53, THE DNS_Probe SHALL record the query start timestamp in a BPF hash map keyed by (pid, transaction_id).
3. THE DNS_Probe SHALL attach a kprobe to `udp_recvmsg` to capture DNS responses.
4. WHEN a DNS response is received matching a tracked query, THE DNS_Probe SHALL emit an ExtendedEvent with event_type 5 to the Ring_Buffer.
5. THE DNS_Probe event payload SHALL include the query domain name (up to 253 bytes), response latency in nanoseconds, response code, and a `timed_out` flag.
6. WHEN a tracked DNS query has not received a response within the DNS_Timeout_Threshold, THE DNS_Probe SHALL emit an ExtendedEvent with the `timed_out` flag set to 1.
7. THE DNS_Timeout_Threshold SHALL be configurable via a BPF map with a default value of 5 seconds (5,000,000,000 nanoseconds).
8. THE DNS_Probe SHALL extract the query domain name by parsing the DNS wire format from the first 253 bytes of the UDP payload using `bpf_skb_load_bytes` or `bpf_probe_read_user`.
9. IF the DNS query domain cannot be parsed, THEN THE DNS_Probe SHALL set the domain field to an empty string and continue emitting the event.

### Requirement 5: Cgroup Resource Accounting

**User Story:** As an SRE, I want per-pod CPU and memory resource metrics derived from cgroup-level accounting, so that I can attribute resource pressure to specific pods.

#### Acceptance Criteria

1. THE Cgroup_Accounting_Probe SHALL read cgroup-level CPU usage and memory usage statistics using BPF cgroup helpers on Linux 5.8+.
2. THE Cgroup_Accounting_Probe SHALL emit an ExtendedEvent with event_type 6 to the Ring_Buffer at a configurable sampling interval (default 10 seconds).
3. THE Cgroup_Accounting_Probe event payload SHALL include cgroup ID, CPU usage in nanoseconds (cumulative), memory usage in bytes (current RSS), memory limit in bytes, and a `memory_pressure` flag.
4. WHEN the current RSS exceeds 90% of the memory limit, THE Cgroup_Accounting_Probe SHALL set the `memory_pressure` flag to 1.
5. THE CgroupResolver SHALL be extended with a `ResolveWithMetrics` method that returns PodIdentity combined with the latest cgroup resource metrics for a given cgroup ID.
6. THE Agent SHALL expose per-pod resource metrics via the existing EnrichedEvent JSON format with additional fields: `cpuUsageNs`, `memoryUsageBytes`, `memoryLimitBytes`, and `memoryPressure`.
7. WHEN the CgroupResolver cache does not contain a mapping for a cgroup ID in a resource accounting event, THE Agent SHALL label the event as host-level and include only the cgroup ID and node name.

### Requirement 6: Predictive Failure Detection Enhancements

**User Story:** As an SRE, I want the prediction engine to incorporate file system I/O latency, memory pressure, and DNS timeout signals, so that failure predictions are more accurate and provide earlier warnings.

#### Acceptance Criteria

1. THE PredictionEngine SHALL accept ExtendedEvent types (event_type 3 through 8) in addition to the existing KernelEvent types (0 through 2).
2. WHEN analyzing events for a node, THE PredictionEngine SHALL include a `filesystem_io_degradation` pattern that detects increasing VFS_Probe latencies over the analysis window.
3. WHEN analyzing events for a node, THE PredictionEngine SHALL include a `memory_pressure_escalation` pattern that detects OOM kills or sustained memory pressure flags from Cgroup_Accounting_Probe events.
4. WHEN analyzing events for a node, THE PredictionEngine SHALL include a `dns_resolution_degradation` pattern that detects increasing DNS response times or DNS timeouts from DNS_Probe events.
5. THE PredictionEngine SHALL weight the new signal patterns alongside existing patterns (syscall_latency_trend, retransmit_spike, critical_exit, high_rtt) using additive confidence scoring clamped to the range 0.0 to 1.0.
6. THE PredictionEngine SHALL include the names of all detected patterns (both existing and new) in the `patterns` field of the Prediction struct.
7. WHEN three or more distinct pattern types are detected simultaneously, THE PredictionEngine SHALL set a minimum confidence of 0.7 regardless of individual pattern scores.

### Requirement 7: Network Policy Auditing

**User Story:** As a security engineer, I want to see which pods are making outbound connections and to where, so that I can audit network policies and detect unexpected communication patterns.

#### Acceptance Criteria

1. THE Security_Socket_Probe SHALL attach an LSM hook or kprobe to `security_socket_connect` on Linux 5.8+.
2. WHEN `security_socket_connect` is called, THE Security_Socket_Probe SHALL emit an ExtendedEvent with event_type 7 to the Ring_Buffer.
3. THE Security_Socket_Probe event payload SHALL include the source cgroup ID, destination IP address (IPv4 as 4 bytes), destination port, socket protocol (TCP or UDP), and the comm of the connecting process.
4. THE Server SHALL maintain a Network_Topology_Map data structure that tracks unique (source_pod, destination_ip, destination_port, protocol) tuples observed within a configurable time window (default 5 minutes).
5. THE Server SHALL expose the Network_Topology_Map via a GET endpoint at `/api/network/topology` returning a JSON array of connection records.
6. WHEN a new connection tuple is observed that was not present in the Network_Topology_Map, THE Server SHALL broadcast a WebSocket message with type `network_topology_update` containing the new connection record.
7. THE Server SHALL expire connection records from the Network_Topology_Map that have not been observed within the configured time window.

### Requirement 8: Causal Chain Integration

**User Story:** As an SRE, I want causal chain analysis to incorporate the new event types, so that root cause summaries include file system, memory, DNS, and network policy signals.

#### Acceptance Criteria

1. WHEN building a causal chain for a NotReady transition, THE CausalChainBuilder SHALL query all event types (0 through 7) from the preceding 120-second window.
2. THE CausalChainBuilder root cause detection SHALL recognize `filesystem_io_bottleneck` when slow VFS_Probe events are present in the chain.
3. THE CausalChainBuilder root cause detection SHALL recognize `oom_kill` when OOM_Probe kill events (sub_type 0) are present in the chain.
4. THE CausalChainBuilder root cause detection SHALL recognize `dns_timeout` when DNS_Probe events with the `timed_out` flag are present in the chain.
5. THE CausalChainBuilder summary generation SHALL include counts of each new event type alongside existing event type counts.

### Requirement 9: Server Event Ingestion for Extended Events

**User Story:** As a developer, I want the server to accept and persist the new event types, so that all probe data flows through the existing pipeline.

#### Acceptance Criteria

1. THE Server `/api/ebpf/events` endpoint SHALL accept EnrichedEvent JSON payloads containing the new event types (event_type values "filesystem_io", "memory_pressure", "dns_resolution", "cgroup_resource", "network_audit").
2. WHEN the Server receives an EnrichedEvent with an unrecognized event type, THE Server SHALL log a warning and persist the event with the event type as-is.
3. THE Server SHALL broadcast all new event types to connected WebSocket clients using the existing `ebpf_event` message type.
4. FOR ALL valid EnrichedEvent JSON payloads containing new event type fields, serializing to JSON and deserializing back SHALL produce an equivalent EnrichedEvent (round-trip property).

### Requirement 10: Visualizer Extensions

**User Story:** As an SRE, I want the React visualizer to display the new signal types, so that I can see file system, memory, DNS, and network audit data alongside existing views.

#### Acceptance Criteria

1. THE Visualizer SHALL define TypeScript types for each new EnrichedEvent variant: `FilesystemIOEvent`, `MemoryPressureEvent`, `DNSResolutionEvent`, `CgroupResourceEvent`, and `NetworkAuditEvent`.
2. WHEN the Visualizer receives a WebSocket message with event type "filesystem_io", THE Visualizer SHALL render the event in the LiveActivityPanel with file path, latency, and slow_io indicator.
3. WHEN the Visualizer receives a WebSocket message with event type "memory_pressure", THE Visualizer SHALL render the event in the LiveActivityPanel with killed process name, OOM score, and sub_type indicator.
4. WHEN the Visualizer receives a WebSocket message with event type "dns_resolution", THE Visualizer SHALL render the event in the LiveActivityPanel with domain name, response time, and timed_out indicator.
5. THE Visualizer SHALL add a "Network Topology" view option to the ViewSelector component that renders the Network_Topology_Map as a connection list showing source pod, destination IP, destination port, and protocol.
6. THE Visualizer SHALL add a "Resource Pressure" view option to the ViewSelector component that renders per-pod CPU and memory usage from Cgroup_Accounting_Probe events.

### Requirement 11: Platform Separation

**User Story:** As a developer, I want all Go-level codec, enrichment, and prediction logic to be testable on macOS, so that I can develop and test without a Linux kernel.

#### Acceptance Criteria

1. THE ExtendedEvent `MarshalBinary` and `UnmarshalBinary` methods SHALL compile and run on macOS and Linux without build tags.
2. THE new prediction patterns (filesystem_io_degradation, memory_pressure_escalation, dns_resolution_degradation) SHALL be implemented as pure Go functions testable on macOS.
3. THE Network_Topology_Map data structure and its expiration logic SHALL be implemented as pure Go testable on macOS.
4. THE new eBPF C programs (VFS_Probe, OOM_Probe, DNS_Probe, Cgroup_Accounting_Probe, Security_Socket_Probe) SHALL compile only on Linux via the existing `bpf2go` build pipeline.
5. THE `loader_stub.go` SHALL be updated to include stub event type constants for the new event types (3 through 7) so that macOS tests can reference them.

### Requirement 12: Configuration and Thresholds

**User Story:** As an operator, I want configurable thresholds for the new probes, so that I can tune sensitivity to my cluster's characteristics.

#### Acceptance Criteria

1. THE Agent SHALL accept the following configuration parameters via environment variables or config file: `EARTHWORM_SLOW_IO_THRESHOLD_MS` (default 100), `EARTHWORM_DNS_TIMEOUT_MS` (default 5000), `EARTHWORM_CGROUP_SAMPLE_INTERVAL_S` (default 10), `EARTHWORM_TOPOLOGY_WINDOW_S` (default 300), `EARTHWORM_MEMORY_PRESSURE_PCT` (default 90).
2. WHEN a configuration parameter is set to a value outside its valid range, THE Agent SHALL log a warning and use the default value.
3. THE BPF-side thresholds (Slow_IO_Threshold, DNS_Timeout_Threshold) SHALL be updatable at runtime via BPF map writes from the Agent without reloading the BPF programs.
4. THE Server SHALL accept `EARTHWORM_TOPOLOGY_WINDOW_S` to configure the Network_Topology_Map expiration window.
