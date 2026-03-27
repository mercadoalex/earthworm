# Implementation Plan: eBPF Advanced Probes

## Overview

Extends the Earthworm eBPF platform with five new probe types (event_type 3–7), a variable-length `ExtendedEvent` binary codec, three new prediction patterns, four new causal chain root causes, a `NetworkTopologyMap`, and two new visualizer views. Tasks are ordered: macOS-safe Go work first (codec, enrichment, prediction, topology, config), then server changes, then visualizer (TypeScript), then Linux-only BPF C programs last.

## Tasks

- [x] 1. Implement ExtendedEvent binary codec and payload structs (macOS-safe)
  - [x] 1.1 Create `src/agent/extended_event.go` with `ExtendedEvent` struct, `MarshalBinary`, `UnmarshalBinary`, and event type constants 3–7
    - Define `ExtendedEvent` struct with 52-byte header (timestamp, pid, ppid, tgid, cgroup_id, comm, event_type, payload_len) + variable payload
    - Implement `MarshalBinary` writing 52-byte header + payload bytes
    - Implement `UnmarshalBinary` with validation: reject buffers < 52 bytes, reject buffers < 52 + payload_len
    - Add constants: `EventTypeVFS = 3`, `EventTypeOOM = 4`, `EventTypeDNS = 5`, `EventTypeCgroup = 6`, `EventTypeSecurity = 7`
    - Add `CommString()` and `EventTypeString()` helpers
    - _Requirements: 1.1, 1.2, 1.5, 1.6, 1.7, 11.1_

  - [x] 1.2 Create payload structs and marshal/unmarshal functions in `src/agent/extended_event.go`
    - `VFSPayload` (280 bytes): FilePath [256]byte, LatencyNs uint64, BytesXfer uint64, SlowIO uint8, OpType uint8
    - `OOMPayload` (36 bytes): SubType uint8, KilledPID uint32, KilledComm [16]byte, OOMScoreAdj int32, PageOrder uint32, GFPFlags uint32
    - `DNSPayload` (268 bytes): Domain [253]byte, LatencyNs uint64, ResponseCode uint16, TimedOut uint8
    - `CgroupResourcePayload` (32 bytes): CPUUsageNs uint64, MemoryUsageBytes uint64, MemoryLimitBytes uint64, MemoryPressure uint8
    - `NetworkAuditPayload` (8 bytes): DstAddr uint32, DstPort uint16, Protocol uint8
    - Each payload struct gets `MarshalBinary` and `UnmarshalBinary` methods
    - _Requirements: 2.3, 3.3, 3.5, 3.6, 4.5, 5.3, 7.3_

  - [ ]* 1.3 Write property test: ExtendedEvent binary round-trip
    - **Property 1: ExtendedEvent binary round-trip**
    - Generate random ExtendedEvent with event_type in {3,4,5,6,7} and correct payload length
    - Assert MarshalBinary → UnmarshalBinary produces identical struct
    - Assert first 48 bytes match KernelEvent common header layout
    - Use `pgregory.net/rapid`, minimum 100 iterations
    - Test file: `src/agent/extended_event_test.go`
    - **Validates: Requirements 1.1, 1.2, 1.5, 1.6**

  - [ ]* 1.4 Write unit tests for ExtendedEvent error cases and payload marshaling
    - `TestExtendedEventShortBuffer`: reject buffers < 52 bytes
    - `TestExtendedEventPayloadTruncated`: reject buffer where len < 52 + payload_len
    - `TestVFSPayloadMarshal`, `TestOOMPayloadMarshal`, `TestDNSPayloadMarshal`, `TestCgroupResourcePayloadMarshal`, `TestNetworkAuditPayloadMarshal`
    - Test file: `src/agent/extended_event_test.go`
    - _Requirements: 1.7, 2.3, 3.3, 4.5, 5.3, 7.3_

- [x] 2. Extend ProbeManager dispatch and EnrichedEvent (macOS-safe)
  - [x] 2.1 Update `src/agent/probe_manager.go` to dispatch extended events
    - Modify `ProcessRawEvent`: if `data[48] <= 2` → existing `processKernelEvent`, if `data[48] >= 3` → new `processExtendedEvent`
    - Implement `processExtendedEvent`: decode ExtendedEvent, decode payload by event_type, enrich via CgroupResolver, forward to eventCh
    - Add `EnrichExtended` method to CgroupResolver that maps ExtendedEvent fields to EnrichedEvent
    - _Requirements: 1.3, 1.4_

  - [x] 2.2 Extend `EnrichedEvent` in `src/agent/event.go` with new probe fields
    - Add filesystem I/O fields: FilePath, IOLatencyNs, BytesXfer, SlowIO, IOOpType
    - Add memory pressure fields: OOMSubType, KilledPID, KilledComm, OOMScoreAdj, PageOrder, GFPFlags
    - Add DNS resolution fields: Domain, DNSLatencyNs, ResponseCode, TimedOut
    - Add cgroup resource fields: CPUUsageNs, MemoryUsageBytes, MemoryLimitBytes, MemoryPressure
    - Add network audit fields: AuditDstAddr, AuditDstPort, AuditProtocol
    - All fields use `json:",omitempty"` tags
    - _Requirements: 5.6, 9.4_

  - [x] 2.3 Mirror EnrichedEvent extensions in `src/server/kernel_event.go`
    - Add the same new optional fields to the server-side `EnrichedEvent` struct
    - _Requirements: 9.1, 9.4_

  - [ ]* 2.4 Write property test: ProbeManager dispatch correctness
    - **Property 2: ProbeManager dispatch correctness**
    - Generate random byte buffers with event_type 0–7 at offset 48
    - Assert event_type 0–2 routes to KernelEvent codec, 3–7 routes to ExtendedEvent codec
    - Use `pgregory.net/rapid`, minimum 100 iterations
    - Test file: `src/agent/probe_manager_test.go`
    - **Validates: Requirements 1.3, 1.4**

  - [ ]* 2.5 Write property test: EnrichedEvent JSON round-trip with extended fields
    - **Property 3: EnrichedEvent JSON round-trip with extended fields**
    - Generate random EnrichedEvent with fields from any new event type
    - Assert json.Marshal → json.Unmarshal produces equivalent struct
    - Test file: `src/agent/extended_event_test.go`
    - **Validates: Requirements 9.4, 5.6**

  - [ ]* 2.6 Write property test: Cgroup resolution for extended events
    - **Property 4: Cgroup resolution for extended events**
    - Generate random ExtendedEvent and random CgroupResolver cache state
    - Assert: cgroup ID in cache → hostLevel false + full PodIdentity; cgroup ID absent → hostLevel true + only nodeName
    - Test file: `src/agent/extended_event_test.go`
    - **Validates: Requirements 5.5, 5.7**

- [x] 3. Checkpoint — Ensure all agent-side tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. Implement new prediction patterns (macOS-safe)
  - [x] 4.1 Add three new pattern detectors to `src/server/prediction.go`
    - `detectFilesystemIODegradation`: detect increasing VFS latencies (≥3 filesystem_io events with monotonically increasing ioLatencyNs → positive score)
    - `detectMemoryPressureEscalation`: detect OOM kills (oomSubType "oom_kill") or sustained memoryPressure flags → positive score
    - `detectDNSResolutionDegradation`: detect increasing DNS latencies (≥3 dns_resolution events) or any timedOut → positive score
    - Update `Analyze` to call all 7 detectors
    - Enforce minimum confidence 0.7 when ≥3 distinct patterns fire
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7_

  - [ ]* 4.2 Write property test: Prediction confidence invariants
    - **Property 5: Prediction confidence invariants**
    - Generate random mix of EnrichedEvents (types 0–7)
    - Assert: confidence ∈ [0.0, 1.0], all firing patterns in patterns list, ≥3 patterns → confidence ≥ 0.7
    - Test file: `src/server/prediction_test.go`
    - **Validates: Requirements 6.5, 6.6, 6.7**

  - [ ]* 4.3 Write property test: Filesystem I/O degradation detection
    - **Property 6: Filesystem I/O degradation detection**
    - Generate sequences with monotonically increasing ioLatencyNs (≥3 events) → assert positive score
    - Generate sequences with no filesystem_io events → assert score 0
    - Test file: `src/server/prediction_test.go`
    - **Validates: Requirements 6.2**

  - [ ]* 4.4 Write property test: Memory pressure escalation detection
    - **Property 7: Memory pressure escalation detection**
    - Generate sequences with OOM kill events or sustained memoryPressure → assert positive score
    - Generate sequences with no memory events → assert score 0
    - Test file: `src/server/prediction_test.go`
    - **Validates: Requirements 6.3**

  - [ ]* 4.5 Write property test: DNS resolution degradation detection
    - **Property 8: DNS resolution degradation detection**
    - Generate sequences with increasing dnsLatencyNs (≥3 events) or timedOut → assert positive score
    - Generate sequences with no dns_resolution events → assert score 0
    - Test file: `src/server/prediction_test.go`
    - **Validates: Requirements 6.4**

- [x] 5. Implement NetworkTopologyMap (macOS-safe)
  - [x] 5.1 Create `src/server/network_topology.go` with `NetworkTopologyMap` and `ConnectionRecord`
    - `ConnectionRecord` struct: SourcePod, SourceNS, DstAddr, DstPort, Protocol, LastSeen, NodeName
    - `NetworkTopologyMap` struct with sync.RWMutex, connections map, configurable window
    - `Record(event EnrichedEvent)`: insert or update connection, return bool indicating new connection
    - `Expire()`: remove records older than window
    - `Connections() []ConnectionRecord`: return all active records
    - Broadcast `network_topology_update` WebSocket message on new connections via Hub
    - _Requirements: 7.4, 7.5, 7.6, 7.7_

  - [ ]* 5.2 Write property test: NetworkTopologyMap record and expire
    - **Property 9: NetworkTopologyMap record and expire**
    - Generate random ConnectionRecord insertions with varying lastSeen times
    - Assert: records within window present after Record, records outside window absent after Expire
    - Test file: `src/server/network_topology_test.go`
    - **Validates: Requirements 7.4, 7.7**

  - [ ]* 5.3 Write property test: NetworkTopologyMap broadcast on new connection
    - **Property 10: NetworkTopologyMap broadcast on new connection**
    - Assert: new tuple → broadcast triggered; duplicate tuple → no broadcast, only lastSeen updated
    - Test file: `src/server/network_topology_test.go`
    - **Validates: Requirements 7.6**

- [x] 6. Extend CausalChainBuilder with new root causes (macOS-safe)
  - [x] 6.1 Update `src/server/causal_chain.go` with new root cause categories and summary counts
    - Add root causes in priority order: critical_exit > oom_kill > filesystem_io_bottleneck > dns_timeout > slow_syscall > network_degradation > unknown_cause
    - `oom_kill`: OOM_Probe events with oomSubType "oom_kill"
    - `filesystem_io_bottleneck`: VFS_Probe events with slowIO true
    - `dns_timeout`: DNS_Probe events with timedOut true
    - `network_policy_violation`: Security_Socket_Probe events (informational, lowest priority)
    - Update `generateSummary` to count each new event type
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

  - [ ]* 6.2 Write property test: Causal chain completeness with new event types
    - **Property 11: Causal chain completeness with new event types**
    - Generate mixed events (types 0–7) within 120s window
    - Assert: all events included in chain, summary counts all present event types
    - Test file: `src/server/causal_chain_test.go`
    - **Validates: Requirements 8.1, 8.5**

  - [ ]* 6.3 Write property test: Causal chain root cause detection for new types
    - **Property 12: Causal chain root cause detection for new types**
    - Generate events with various combinations of OOM kills, slow VFS, DNS timeouts
    - Assert priority order: critical_exit > oom_kill > filesystem_io_bottleneck > dns_timeout > slow_syscall > network_degradation > unknown_cause
    - Test file: `src/server/causal_chain_test.go`
    - **Validates: Requirements 8.2, 8.3, 8.4**

- [x] 7. Implement configuration and validation (macOS-safe)
  - [x] 7.1 Create `src/agent/config.go` with new environment variable parsing and validation
    - Parse: EARTHWORM_SLOW_IO_THRESHOLD_MS (default 100, range 1–60000), EARTHWORM_DNS_TIMEOUT_MS (default 5000, range 100–60000), EARTHWORM_CGROUP_SAMPLE_INTERVAL_S (default 10, range 1–3600), EARTHWORM_TOPOLOGY_WINDOW_S (default 300, range 10–86400), EARTHWORM_MEMORY_PRESSURE_PCT (default 90, range 1–100)
    - Out-of-range values log warning and fall back to defaults
    - _Requirements: 12.1, 12.2_

  - [ ]* 7.2 Write property test: Configuration validation with defaults
    - **Property 14: Configuration validation with defaults**
    - Generate random values (in-range and out-of-range) for each parameter
    - Assert: in-range → used as-is; out-of-range → default used
    - Test file: `src/agent/config_test.go`
    - **Validates: Requirements 12.1, 12.2, 12.4**

- [x] 8. Checkpoint — Ensure all Go tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 9. Wire server-side components together
  - [x] 9.1 Update `src/server/main.go` to initialize NetworkTopologyMap and register `/api/network/topology` endpoint
    - Create `NetworkTopologyMap` with configured window from EARTHWORM_TOPOLOGY_WINDOW_S
    - Register `GET /api/network/topology` handler
    - Wire `ebpfEventsHandler` to call `topoMap.Record()` for network_audit events
    - Wire `ebpfEventsHandler` to call `chainBuilder` with new event types
    - Add `BroadcastTopologyUpdate` method to Hub in `src/server/ws.go`
    - _Requirements: 7.4, 7.5, 7.6, 9.1, 9.2, 9.3, 12.4_

  - [x] 9.2 Update `src/server/config.go` to parse EARTHWORM_TOPOLOGY_WINDOW_S for server-side use
    - _Requirements: 12.4_

  - [ ]* 9.3 Write property test: Server ingestion and broadcast for extended events
    - **Property 13: Server ingestion and broadcast for extended events**
    - POST valid EnrichedEvent JSON arrays with new event types
    - Assert: all events persisted to store, all events broadcast via WebSocket, counts match input
    - Test file: `src/server/handler_unit_test.go`
    - **Validates: Requirements 9.1, 9.3**

- [x] 10. Update loader_stub.go for macOS compatibility
  - [x] 10.1 Add extended event type constants to `src/agent/loader_stub.go`
    - Export constants: `EventTypeVFS`, `EventTypeOOM`, `EventTypeDNS`, `EventTypeCgroup`, `EventTypeSecurity` (values 3–7)
    - Ensure macOS tests can reference these constants
    - _Requirements: 11.5_

- [x] 11. Checkpoint — Ensure all Go tests pass on macOS
  - Ensure all tests pass, ask the user if questions arise.

- [x] 12. Extend React Visualizer with new event types and views
  - [x] 12.1 Add TypeScript types for new event variants in `src/heartbeat-visualizer/src/types/heartbeat.ts`
    - Add `FilesystemIOEvent`, `MemoryPressureEvent`, `DNSResolutionEvent`, `CgroupResourceEvent`, `NetworkAuditEvent` interfaces extending `EnrichedKernelEvent`
    - Extend `EnrichedKernelEvent.eventType` union with new string literals
    - Add `NetworkTopologyUpdate` WebSocket message type and `ConnectionRecord` type
    - Extend `ViewType` union with `'network-topology'` and `'resource-pressure'`
    - _Requirements: 10.1_

  - [x] 12.2 Add event renderers to `src/heartbeat-visualizer/src/LiveActivityPanel.tsx`
    - Add renderer for `filesystem_io`: show file path, latency, slow_io badge
    - Add renderer for `memory_pressure`: show killed process, OOM score, sub_type
    - Add renderer for `dns_resolution`: show domain, response time, timed_out badge
    - Add renderer for `cgroup_resource`: show pod, CPU/memory usage, pressure flag
    - Add renderer for `network_audit`: show source pod, destination, protocol
    - _Requirements: 10.2, 10.3, 10.4_

  - [x] 12.3 Create `src/heartbeat-visualizer/src/views/NetworkTopologyView.tsx`
    - Fetch connection records from `GET /api/network/topology`
    - Listen for `network_topology_update` WebSocket messages for live updates
    - Render connection list: source pod, destination IP, port, protocol, last seen
    - _Requirements: 10.5_

  - [x] 12.4 Create `src/heartbeat-visualizer/src/views/ResourcePressureView.tsx`
    - Render per-pod CPU and memory usage from cgroup_resource events
    - Show memory pressure indicators when RSS > 90% of limit
    - _Requirements: 10.6_

  - [x] 12.5 Update `src/heartbeat-visualizer/src/ViewSelector.tsx` to include new view options
    - Add `'network-topology'` and `'resource-pressure'` to VIEW_OPTIONS
    - Update ViewContext type if needed
    - _Requirements: 10.5, 10.6_

- [x] 13. Checkpoint — Ensure visualizer builds and all TypeScript tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 14. Implement BPF C programs (Linux-only)
  - [ ] 14.1 Create `src/ebpf/headers/extended_common.h` with `extended_event` C struct and payload structs
    - Define `struct extended_event` with flexible array member payload
    - Define `struct vfs_payload`, `struct oom_payload`, `struct dns_payload`, `struct cgroup_resource_payload`, `struct network_audit_payload`
    - _Requirements: 1.1, 1.2_

  - [ ] 14.2 Create `src/ebpf/vfs_probe.c`
    - Attach kprobes to `vfs_read` and `vfs_write`
    - Measure latency, resolve file path via `bpf_d_path()`, set slow_io flag based on threshold from BPF map
    - Emit extended_event with event_type 3 to ring buffer
    - Filter by cgroup using `bpf_get_current_cgroup_id()`
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8_

  - [ ] 14.3 Create `src/ebpf/oom_probe.c`
    - Attach kprobe to `oom_kill_process` (sub_type 0) and tracepoint to `mm_page_alloc` failure (sub_type 1)
    - Capture killed PID, comm, OOM score adj, page order, GFP flags
    - Emit extended_event with event_type 4 to ring buffer
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7_

  - [ ] 14.4 Create `src/ebpf/dns_probe.c`
    - Attach kprobes to `udp_sendmsg` (filter dst port 53) and `udp_recvmsg`
    - Track queries in BPF hash map keyed by (pid, transaction_id)
    - Parse domain from DNS wire format, measure latency, detect timeouts via BPF timer or agent-side check
    - Emit extended_event with event_type 5 to ring buffer
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 4.8, 4.9_

  - [ ] 14.5 Create `src/ebpf/cgroup_accounting.c`
    - Use BPF cgroup helpers to read CPU and memory stats per cgroup
    - Emit extended_event with event_type 6 at configurable interval
    - Set memory_pressure flag when RSS > configured percentage of limit
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

  - [ ] 14.6 Create `src/ebpf/security_socket.c`
    - Attach kprobe or LSM hook to `security_socket_connect`
    - Capture destination IP, port, protocol, source cgroup ID
    - Emit extended_event with event_type 7 to ring buffer
    - _Requirements: 7.1, 7.2, 7.3_

  - [ ] 14.7 Update `src/agent/loader_linux.go` to load new BPF programs and configure BPF maps
    - Load all 5 new programs via `bpf2go`
    - Push threshold values (Slow_IO_Threshold, DNS_Timeout_Threshold, memory_pressure_pct, cgroup_sample_interval) to BPF maps at startup
    - Support runtime threshold updates without program reload
    - _Requirements: 2.5, 4.7, 12.3_

  - [ ] 14.8 Update `src/agent/gen.go` with `go:generate` directives for new BPF programs
    - Add bpf2go generate lines for vfs_probe, oom_probe, dns_probe, cgroup_accounting, security_socket
    - _Requirements: 11.4_

- [ ] 15. Final checkpoint — Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Tasks 1–11 are macOS-safe (pure Go, no kernel dependencies)
- Task 12 is TypeScript/React (macOS-safe)
- Task 14 is Linux-only (BPF C compilation and loader changes)
- Property tests use `pgregory.net/rapid` (Go) and `fast-check` (TypeScript)
- Each property test references its design document property number and validated requirements
- Checkpoints ensure incremental validation at natural break points
