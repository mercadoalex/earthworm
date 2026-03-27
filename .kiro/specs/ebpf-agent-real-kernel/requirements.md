# Requirements Document

## Introduction

The Earthworm project has scaffolding for eBPF-based kernel observability (C probe programs, a Go userspace agent, server-side event ingestion, and a Helm-based DaemonSet deployment) but currently runs entirely on mock data. This feature makes the eBPF agent operational on real Linux nodes (kernel 5.8+ with BTF and CAP_BPF), wiring up the full pipeline: kernel tracepoints/kprobes → BPF ring buffer → Go agent → HTTP → Go server → WebSocket → React visualizer. The implementation must preserve graceful degradation on non-Linux platforms and separate Mac-testable work (Go unit tests, Docker builds) from Linux-only work (eBPF compilation, probe loading, integration tests).

## Glossary

- **Agent**: The Go userspace binary (`src/agent/`) that loads eBPF programs, reads events from the BPF ring buffer, enriches them with Kubernetes pod identity, and forwards them to the Server.
- **BPF_Loader**: The platform-specific component (`loader_linux.go` / `loader_stub.go`) responsible for loading compiled eBPF object files into the kernel and attaching them to tracepoints or kprobes.
- **Probe_Manager**: The component (`probe_manager.go`) that polls the BPF ring buffer, decodes raw binary events, enriches them via the Cgroup_Resolver, and forwards EnrichedEvents to the event channel.
- **Cgroup_Resolver**: The component (`cgroup_resolver.go`) that maps Linux cgroup IDs to Kubernetes pod identities by querying the kubelet API and reading `/proc/<pid>/cgroup`.
- **Server**: The Go HTTP/WebSocket server (`src/server/`) that receives enriched events from the Agent and broadcasts them to connected clients.
- **BPF_Ring_Buffer**: A shared BPF map of type `BPF_MAP_TYPE_RINGBUF` used by all eBPF programs to emit `kernel_event` structs to userspace.
- **bpf2go**: The `cilium/ebpf/cmd/bpf2go` code generator that compiles eBPF C programs into `.o` files and generates Go loader code.
- **EnrichedEvent**: A JSON-serializable Go struct combining decoded kernel event fields with Kubernetes pod identity metadata.
- **KernelEvent**: The 120-byte binary C struct (`kernel_event` in `common.h`) shared between all eBPF programs and the Go decoder.
- **BTF**: BPF Type Format — kernel metadata enabling CO-RE (Compile Once, Run Everywhere) portability for eBPF programs.
- **DaemonSet**: A Kubernetes workload that runs one Agent pod per node.
- **Helm_Chart**: The Helm chart (`deploy/helm/earthworm/`) that templates the Agent DaemonSet, RBAC, and configuration.

## Requirements

### Requirement 1: eBPF Program Compilation via bpf2go

**User Story:** As a developer, I want the eBPF C programs to be compiled into Go-loadable objects via bpf2go, so that the Agent can load them at runtime without shipping a C compiler.

#### Acceptance Criteria

1. WHEN `go generate ./src/agent/...` is executed on a Linux host with clang installed, THE bpf2go tool SHALL produce Go source files and compiled `.o` objects for each eBPF program (syscall_tracer, process_monitor, network_probe, heartbeat).
2. THE bpf2go-generated Go files SHALL compile without errors as part of the Agent binary on Linux.
3. WHEN `go generate` is executed, THE bpf2go tool SHALL target `bpfel` (little-endian) and include the `vmlinux.h` and `common.h` headers from `src/ebpf/headers/`.
4. IF `clang` is not installed or the eBPF C source contains a syntax error, THEN THE bpf2go tool SHALL return a non-zero exit code with a descriptive error message.

### Requirement 2: BPF Program Loading and Attachment

**User Story:** As an operator, I want the Agent to load and attach eBPF programs to kernel tracepoints and kprobes at startup, so that kernel events are captured in real time.

#### Acceptance Criteria

1. WHEN the Agent starts on a Linux host with kernel 5.8+ and CAP_BPF capability, THE BPF_Loader SHALL load all four bpf2go-generated program objects (syscall_tracer, process_monitor, network_probe, heartbeat) into the kernel.
2. WHEN loading succeeds, THE BPF_Loader SHALL attach each program to its designated hook point: tracepoints for syscall_tracer (`sys_enter_write`, `sys_exit_write`, `sys_enter_sendto`, `sys_exit_sendto`), tracepoints for process_monitor (`sched_process_fork`, `sched_process_exec`, `sched_process_exit`), tracepoint for heartbeat (`sched_switch`), and kprobes for network_probe (`tcp_retransmit_skb`, `tcp_reset`).
3. IF the kernel version is below 5.8, THEN THE BPF_Loader SHALL return an error containing the detected kernel version and the minimum required version.
4. IF CAP_BPF or CAP_SYS_ADMIN capability is missing, THEN THE BPF_Loader SHALL return an error describing the missing capability.
5. WHEN the Agent receives SIGINT or SIGTERM, THE BPF_Loader SHALL detach all programs and close all map file descriptors within 5 seconds.

### Requirement 3: Ring Buffer Event Reading

**User Story:** As a developer, I want the Probe_Manager to read raw kernel events from the BPF ring buffer, so that events flow from kernel space to userspace.

#### Acceptance Criteria

1. WHEN eBPF programs emit events to the BPF_Ring_Buffer, THE Probe_Manager SHALL read each event using the cilium/ebpf ringbuf.Reader within the configured poll interval (default 100ms).
2. THE Probe_Manager SHALL decode each raw 120-byte record into a KernelEvent struct using the existing `UnmarshalBinary` method.
3. IF the ring buffer reader encounters a transient error, THEN THE Probe_Manager SHALL log the error and continue reading subsequent events.
4. IF the event channel is full, THEN THE Probe_Manager SHALL increment the dropped-event counter and log the drop count at most once per 10 seconds.
5. THE BPF_Ring_Buffer size SHALL be configurable via the `--ring-buffer-size` CLI flag (default 256 KB).

### Requirement 4: Cgroup-to-Pod Resolution

**User Story:** As an operator, I want the Agent to resolve cgroup IDs to Kubernetes pod identities, so that kernel events are attributed to the correct workloads.

#### Acceptance Criteria

1. THE Cgroup_Resolver SHALL query the kubelet read-only API (`/pods` endpoint) at the configured refresh interval (default 30 seconds) to obtain the list of running pods.
2. WHEN a pod list is received, THE Cgroup_Resolver SHALL read `/proc/<pid>/cgroup` for each container process to build a mapping from cgroup ID to PodIdentity (pod name, namespace, container name, node name).
3. WHEN a KernelEvent contains a cgroup ID present in the cache, THE Cgroup_Resolver SHALL return the matching PodIdentity with `hostLevel` set to false.
4. WHEN a KernelEvent contains a cgroup ID not present in the cache, THE Cgroup_Resolver SHALL return a PodIdentity with only the NodeName populated and `hostLevel` set to true.
5. IF the kubelet API is unreachable, THEN THE Cgroup_Resolver SHALL retain the stale cache and log a warning.

### Requirement 5: Event Enrichment and Forwarding

**User Story:** As a developer, I want decoded kernel events to be enriched with pod identity and forwarded to the Server, so that the full observability pipeline is connected.

#### Acceptance Criteria

1. THE Probe_Manager SHALL enrich each decoded KernelEvent into an EnrichedEvent using the Cgroup_Resolver before placing it on the event channel.
2. THE Agent SHALL batch EnrichedEvents (up to 100 per batch or 1-second flush interval, whichever comes first) and send them to the Server via HTTP POST to `/api/ebpf/events`.
3. IF the Server returns an HTTP status code >= 400, THEN THE Agent SHALL log the status code and discard the batch.
4. IF the Server is unreachable, THEN THE Agent SHALL log the connection error and retry on the next flush interval.
5. FOR ALL valid KernelEvent structs, encoding via `MarshalBinary` then decoding via `UnmarshalBinary` SHALL produce a KernelEvent equal to the original (round-trip property).
6. FOR ALL valid KernelEvent structs, enriching via `Enrich` SHALL produce an EnrichedEvent whose JSON serialization then deserialization yields an equivalent EnrichedEvent (round-trip property).

### Requirement 6: Graceful Degradation on Non-Linux Platforms

**User Story:** As a developer on macOS, I want the Agent to compile and run without eBPF support, so that I can develop and test the Go code locally.

#### Acceptance Criteria

1. WHEN the Agent is compiled on a non-Linux platform, THE BPF_Loader (loader_stub.go) SHALL be used via build tags, and the `Load()` method SHALL return an error indicating eBPF is unsupported.
2. WHEN `BPF_Loader.Load()` returns an error, THE Agent SHALL log the error and continue running without eBPF event collection.
3. THE Agent Go code (event.go, cgroup_resolver.go, probe_manager.go) SHALL compile and pass unit tests on macOS and Linux without requiring eBPF kernel support.
4. THE loader_stub.go SHALL implement the same public interface (NewBPFLoader, Load, Close, Programs) as loader_linux.go so that the rest of the Agent code is platform-agnostic.

### Requirement 7: Docker Multi-Stage Build for Agent

**User Story:** As a DevOps engineer, I want the Agent Docker image to compile eBPF programs and build the Go binary in a reproducible multi-stage build, so that the image is self-contained and deployable.

#### Acceptance Criteria

1. THE Dockerfile.agent SHALL use a first stage with clang and libbpf-dev to compile all four eBPF C programs into `.o` files.
2. THE Dockerfile.agent SHALL use a second stage with Go and bpf2go to generate Go bindings from the compiled `.o` files and build the Agent binary.
3. THE Dockerfile.agent SHALL use a final distroless stage containing only the Agent binary and compiled eBPF `.o` files.
4. WHEN `docker build -f deploy/docker/Dockerfile.agent .` is executed, THE build SHALL complete without errors and produce a runnable image.
5. THE final image SHALL be less than 50 MB in size.

### Requirement 8: Helm Chart DaemonSet Deployment

**User Story:** As a cluster operator, I want to deploy the Agent as a Kubernetes DaemonSet via Helm, so that every node in the cluster runs the eBPF agent.

#### Acceptance Criteria

1. WHEN `ebpf.enabled` is set to true in Helm values, THE Helm_Chart SHALL render an Agent DaemonSet with `hostPID: true` and security capabilities CAP_BPF, CAP_SYS_ADMIN, and CAP_PERFMON.
2. WHEN `ebpf.enabled` is set to false in Helm values, THE Helm_Chart SHALL not render the Agent DaemonSet.
3. THE DaemonSet template SHALL pass the Server URL, ring buffer size, and node name as environment variables or CLI arguments to the Agent container.
4. THE DaemonSet template SHALL support configurable `nodeSelector` and `tolerations` via Helm values for targeting specific node pools.
5. THE DaemonSet template SHALL mount `/sys/fs/cgroup` and `/proc` from the host as read-only volumes so the Cgroup_Resolver can read cgroup paths.
6. THE Helm_Chart SHALL include RBAC resources granting the Agent ServiceAccount permission to read pod metadata.

### Requirement 9: Server-Side Event Ingestion and Broadcasting

**User Story:** As a frontend developer, I want the Server to receive enriched kernel events and broadcast them over WebSocket, so that the React visualizer can display live kernel activity.

#### Acceptance Criteria

1. WHEN the Server receives a POST to `/api/ebpf/events` with a JSON array of EnrichedEvents, THE Server SHALL decode the payload, persist each event to the store, and broadcast each event to connected WebSocket clients with message type `ebpf_event`.
2. IF the POST body is not valid JSON, THEN THE Server SHALL return HTTP 400 with a descriptive error message.
3. WHEN a WebSocket client connects to `/ws/heartbeats`, THE Server SHALL send `ebpf_event` messages in real time as they arrive from the Agent.
4. THE Server SHALL pass each received EnrichedEvent to the PredictionEngine for anomaly analysis.

### Requirement 10: Event Binary Codec Correctness

**User Story:** As a developer, I want the KernelEvent binary codec to correctly handle all field types and padding, so that events decoded from the ring buffer match what the kernel wrote.

#### Acceptance Criteria

1. THE KernelEvent `MarshalBinary` method SHALL produce exactly 120 bytes matching the C struct layout defined in `common.h`, including all padding bytes.
2. THE KernelEvent `UnmarshalBinary` method SHALL correctly decode all fields from a 120-byte buffer, respecting the C struct alignment and padding at offsets 20, 49, 81, 93, 109, and 116.
3. IF `UnmarshalBinary` receives a buffer smaller than 120 bytes, THEN THE method SHALL return an error stating the actual and required sizes.
4. FOR ALL valid KernelEvent values, `MarshalBinary` followed by `UnmarshalBinary` SHALL produce a KernelEvent identical to the original (round-trip property).
5. THE `ValidateFlags` method SHALL return true when the `slow_syscall`, `critical_exit`, and `net_event_type` flags are consistent with the measured values, and false otherwise.

### Requirement 11: Kernel Prerequisite Validation

**User Story:** As an operator, I want the Agent to validate kernel prerequisites at startup, so that I get clear error messages instead of cryptic BPF verifier failures.

#### Acceptance Criteria

1. WHEN the Agent starts on Linux, THE BPF_Loader SHALL check the kernel version and return an error if the version is below 5.8.
2. WHEN the Agent starts on Linux, THE BPF_Loader SHALL check for CAP_BPF or root privileges and return an error if neither is available.
3. WHEN the Agent starts on Linux, THE BPF_Loader SHALL verify that BTF data is available (by checking for `/sys/kernel/btf/vmlinux`) and return a descriptive error if BTF is missing.
4. IF any prerequisite check fails, THEN THE Agent SHALL log the specific failure reason and exit with a non-zero exit code.

### Requirement 12: Agent Observability and Health

**User Story:** As an operator, I want the Agent to expose operational metrics, so that I can monitor its health and detect event loss.

#### Acceptance Criteria

1. THE Probe_Manager SHALL maintain a counter of dropped events (events that could not be placed on the event channel because it was full).
2. THE Agent SHALL log the total dropped event count at most once per 10 seconds when drops occur.
3. WHEN the Agent starts successfully with eBPF loaded, THE Agent SHALL log the number of attached programs and the ring buffer size.
4. WHEN the Agent shuts down, THE Agent SHALL log the total number of events processed and events dropped during the session.
