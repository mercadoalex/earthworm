# Tasks

## Task 1: BPF C Programs and Shared Headers

- [x] 1.1 Create `src/ebpf/headers/common.h` with the shared `kernel_event` struct, event type constants, and BPF map definitions (ring buffer, per-CPU counter, inflight syscalls hash)
- [x] 1.2 Create `src/ebpf/syscall_tracer.c` attaching to `sys_enter_write`, `sys_enter_sendto`, `sys_exit_write`, `sys_exit_sendto` tracepoints; filter by comm "kubelet"/"containerd"/"cri-o"; compute latency; set `slow_syscall` flag when > 1s; emit to ring buffer
- [x] 1.3 Create `src/ebpf/process_monitor.c` attaching to `sched_process_fork`, `sched_process_exec`, `sched_process_exit` tracepoints; filter by comm; set `critical_exit` flag for kubelet non-zero exit; emit to ring buffer
- [x] 1.4 Create `src/ebpf/network_probe.c` attaching to `tcp_retransmit_skb` and `tcp_reset` kprobes; track per-connection RTT; emit `rtt_high` when RTT > 500ms; emit to ring buffer
- [x] 1.5 Refactor existing `src/ebpf/heartbeat.c` to use BPF ring buffer map instead of perf event array, and use CO-RE helpers

## Task 2: Agent Go Package — BPF Loader

- [x] 2.1 Create `src/agent/gen.go` with `//go:generate` directives for `bpf2go` to generate Go bindings from each BPF C program
- [x] 2.2 Create `src/agent/loader.go` implementing `BPFLoader` struct with `Load()`, `Close()`, and `Programs()` methods using cilium/ebpf; detect kernel version < 5.8 and missing capabilities; handle partial load on verifier rejection
- [x] 2.3 Create `src/agent/event.go` defining Go `KernelEvent` and `EnrichedEvent` structs with binary encoding/decoding methods matching the BPF `kernel_event` struct layout
- [x] 2.4 Write property test for `KernelEvent` binary round-trip (Property 15): `src/agent/event_codec_test.go`
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 15: KernelEvent binary round-trip — for any valid KernelEvent, encode then decode produces equivalent struct
- [x] 2.5 Write property test for BPF Loader cleanup invariant (Property 1): `src/agent/loader_test.go`
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 1: BPF Loader cleanup — for any loaded programs, Close() releases all resources

## Task 3: Agent Go Package — Probe Manager and Cgroup Resolver

- [x] 3.1 Create `src/agent/probe_manager.go` implementing `ProbeManager` with ring buffer polling (configurable interval, default 100ms), event forwarding to CgroupResolver, dropped event counter with rate-limited logging
- [x] 3.2 Create `src/agent/cgroup_resolver.go` implementing `CgroupResolver` with in-memory cache, kubelet API and /proc-based cgroup-to-pod mapping, configurable refresh interval (default 30s)
- [x] 3.3 Write property tests for event field completeness (Property 2) and conditional flag correctness (Property 3): `src/agent/probe_manager_test.go`
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 2: KernelEvent field completeness — for any event, all required fields for its type are present
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 3: Conditional flag correctness — slow_syscall, critical_exit, rtt_high flags match thresholds
- [x] 3.4 Write property tests for comm filter invariant (Property 4): `src/agent/loader_test.go`
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 4: Comm filter — emitted events only have allowed comm names
- [x] 3.5 Write property tests for cgroup enrichment (Properties 5, 6): `src/agent/cgroup_resolver_test.go`
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 5: Cgroup enrichment completeness — known cgroup IDs produce complete pod identity
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 6: Unknown cgroup host-level labeling — unknown cgroup IDs produce hostLevel=true

## Task 4: Agent Entrypoint

- [x] 4.1 Create `src/agent/main.go` as the agent binary entrypoint: initialize BPFLoader, ProbeManager, CgroupResolver; forward enriched events to server via HTTP POST (batched); handle graceful shutdown with 5-second timeout
- [x] 4.2 Add `--server-url`, `--poll-interval`, `--ring-buffer-size`, `--cgroup-refresh` CLI flags to agent

## Task 5: Server — Extended Store Interface

- [x] 5.1 Extend `Store` interface in `src/server/store.go` with `SaveKernelEvent`, `GetKernelEvents`, `GetKernelEventsByType`, `SaveCausalChain`, `GetCausalChains` methods
- [x] 5.2 Implement the new Store methods in `MemoryStore` (`src/server/store.go`)
- [x] 5.3 Implement the new Store methods in `RedisStore` (`src/server/redis_store.go`)
- [x] 5.4 Write property test for event persistence round-trip (Property 10): `src/server/replay_test.go`
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 10: KernelEvent persistence round-trip — store then query returns equivalent event

## Task 6: Server — Causal Chain Builder

- [x] 6.1 Create `src/server/causal_chain.go` implementing `CausalChainBuilder` with 120-second lookback window, chronological ordering, human-readable summary generation, and "unknown_cause" fallback
- [x] 6.2 Add `OnNotReady` trigger that queries kernel events from the store, builds the chain, stores it, and broadcasts via WebSocket Hub with "causal_chain" message type
- [x] 6.3 Write property test for causal chain construction invariants (Property 7): `src/server/causal_chain_test.go`
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 7: Causal chain invariants — events within 120s window, chronologically ordered, non-empty summary

## Task 7: Server — Prediction Engine

- [x] 7.1 Create `src/server/prediction.go` implementing `PredictionEngine` with sliding window analysis, behavioral pattern matching (syscall latency trends, retransmit spikes, memory pressure), confidence scoring, and accuracy tracking
- [x] 7.2 Add WebSocket broadcast of "prediction" message type and REST endpoint for accuracy metrics
- [x] 7.3 Write property test for prediction confidence bounds (Property 8): `src/server/prediction_test.go`
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 8: Prediction confidence bounds — confidence in [0.0, 1.0] and TTF positive
- [x] 7.4 Write property test for prediction accuracy computation (Property 9): `src/server/prediction_test.go`
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 9: Prediction accuracy computation — TPR and FPR computed correctly from outcomes

## Task 8: Server — Replay Store and API

- [x] 8.1 Create `src/server/replay.go` implementing `ReplayStore` with configurable retention, multi-filter query (node, time range, event type, pod name, min latency), and pagination (default page size 1000)
- [x] 8.2 Add REST API handlers: `GET /api/replay?node=&from=&to=&type=&pod=&minLatency=&page=&pageSize=`
- [x] 8.3 Write property test for replay query filter correctness (Property 11): `src/server/replay_test.go`
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 11: Replay query filter correctness — returned events match all filters and are chronologically ordered
- [x] 8.4 Write property test for replay pagination bounds (Property 12): `src/server/replay_test.go`
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 12: Replay pagination bounds — page size not exceeded, no duplicates or gaps across pages

## Task 9: Server — Integration and WebSocket Extensions

- [x] 9.1 Extend `src/server/ws.go` with new WSMessage types: "ebpf_event", "causal_chain", "prediction"
- [x] 9.2 Modify `src/server/main.go` to add `--ebpf` CLI flag, initialize BPF loader (when enabled), add HTTP endpoint for agent event ingestion (`POST /api/ebpf/events`), and fall back to mock mode when eBPF is unavailable
- [x] 9.3 Modify `src/server/anomaly.go` to include correlated kernel events in Alert payload when available
- [x] 9.4 Write property test for WSMessage envelope compliance (Property 13): `src/server/ws_test.go`
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 13: WSMessage envelope compliance — broadcast messages have correct type and payload structure
- [x] 9.5 Write property test for enhanced alert with kernel events (Property 14): `src/server/anomaly_test.go`
  - [x] 🧪 PBT: Feature: ebpf-kernel-observability, Property 14: Enhanced alert with kernel events — alerts include correlated events from 120s window

## Task 10: Visualizer — WebSocket and Type Extensions

- [x] 10.1 Add `EnrichedKernelEvent`, `CausalChainMessage`, and `PredictionMessage` types to `src/heartbeat-visualizer/src/types/heartbeat.ts`
- [x] 10.2 Extend `useWebSocket.ts` to handle "ebpf_event", "causal_chain", and "prediction" message types, routing parsed data to appropriate state/callbacks
- [x] 10.3 Extend `TimelineView.tsx` to render causal chain markers with directional arrows on node swimlanes, display causal chain summary in gap detail panel, and add "Replay" button that fetches from `/api/replay`
- [x] 10.4 Extend `HeatmapView.tsx` to highlight predicted node cells with a pulsing border animation when a prediction alert is received

## Task 11: Kubernetes Deployment — Helm Chart

- [x] 11.1 Create `deploy/helm/earthworm/Chart.yaml` and `deploy/helm/earthworm/values.yaml` with all configurable values (ebpf.enabled, agent image/resources/nodeSelector, server replicas/image/store/thresholds/prediction/replay, ui replicas/image/serviceType)
- [x] 11.2 Create `deploy/helm/earthworm/templates/agent-daemonset.yaml` with hostPID:true, CAP_BPF/CAP_SYS_ADMIN/CAP_PERFMON capabilities, conditional on `ebpf.enabled`
- [x] 11.3 Create `deploy/helm/earthworm/templates/server-deployment.yaml` and `deploy/helm/earthworm/templates/ui-deployment.yaml`
- [x] 11.4 Create `deploy/helm/earthworm/templates/rbac.yaml` with ServiceAccount, ClusterRole (leases, nodes, pods, namespaces read), and ClusterRoleBinding
- [x] 11.5 Create `deploy/helm/earthworm/templates/configmap.yaml` and `deploy/helm/earthworm/templates/services.yaml`
- [x] 11.6 Create `deploy/earthworm.yaml` standalone kubectl manifest
- [x] 11.7 Write property test for Helm template configuration propagation (Property 16): `deploy/helm/earthworm/tests/values_test.go`
  - [ ] 🧪 PBT: Feature: ebpf-kernel-observability, Property 16: Helm template configuration propagation — rendered templates reflect values.yaml (capabilities when enabled, no agent when disabled, ConfigMap values match)

## Task 12: Container Images and CI

- [x] 12.1 Create `deploy/docker/Dockerfile.agent` (multi-stage: clang-15 + bpf2go compile stage → Go build stage → distroless final)
- [x] 12.2 Create `deploy/docker/Dockerfile.server` (Go build → distroless final)
- [x] 12.3 Create `deploy/docker/Dockerfile.ui` (npm build → nginx:alpine final)
- [x] 12.4 Create `Makefile` with targets: `build-agent`, `build-server`, `build-ui`, `build-all`, `push-all`, `helm-package`, `deploy`
