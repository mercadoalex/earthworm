# Implementation Plan: eBPF Agent Real Kernel Integration

## Overview

This plan wires the existing Earthworm eBPF scaffolding into a working pipeline: bpf2go code generation, real BPF program loading, ring buffer reading, cgroup-to-pod resolution, and Helm DaemonSet deployment. Tasks are ordered so macOS-safe work (Go unit tests, property tests, Helm template tests) comes first, and Linux-only work (eBPF compilation, loading, integration) comes last.

## Tasks

- [x] 1. Verify and harden the event binary codec (macOS-safe)
  - [x] 1.1 Verify KernelEvent MarshalBinary/UnmarshalBinary round-trip correctness
    - The existing `event_codec_test.go` has a round-trip property test from the prior spec. Add a new property test tagged for this feature that also asserts the encoded buffer is exactly 120 bytes and covers all padding offsets (20, 49, 81, 93, 109, 116).
    - File: `src/agent/event_codec_test.go`
    - _Requirements: 5.5, 10.1, 10.2, 10.4_

  - [ ]* 1.2 Write property test for short buffer rejection (Property 7)
    - **Property 7: Short buffer rejection**
    - For any byte buffer with length < 120, `UnmarshalBinary` must return a non-nil error containing both the actual and required sizes.
    - File: `src/agent/event_codec_test.go`
    - **Validates: Requirements 10.3**

  - [ ]* 1.3 Write property test for EnrichedEvent JSON round-trip (Property 2)
    - **Property 2: EnrichedEvent JSON round-trip**
    - For any KernelEvent and PodIdentity, `Enrich()` → JSON marshal → JSON unmarshal produces an equivalent EnrichedEvent.
    - File: `src/agent/event_codec_test.go`
    - **Validates: Requirements 5.6**

  - [ ]* 1.4 Write property test for conditional flag consistency (Property 4)
    - **Property 4: Conditional flag consistency**
    - For any KernelEvent with flags set consistently with measured values, `ValidateFlags()` returns true; for inconsistent flags, it returns false.
    - File: `src/agent/probe_manager_test.go`
    - **Validates: Requirements 10.5**

- [x] 2. Verify and complete cgroup resolution logic (macOS-safe)
  - [x] 2.1 Complete the `CgroupResolver.refresh()` method to build cgroup-ID-to-pod mapping
    - Read `/proc/<pid>/cgroup` for each container process from the kubelet pod list. Use `os.Stat` on the cgroup v2 path under `/sys/fs/cgroup` to get the inode number as the cgroup ID. Build the `map[uint64]PodIdentity` cache. Guard `/proc` reads behind a runtime.GOOS check so the method is safe to compile on macOS (returns early with no-op).
    - File: `src/agent/cgroup_resolver.go`
    - _Requirements: 4.1, 4.2_

  - [ ]* 2.2 Write property test for cgroup resolution correctness (Property 3)
    - **Property 3: Cgroup resolution correctness**
    - For any CgroupResolver with a populated cache: known cgroup IDs produce `hostLevel=false` with full pod identity; unknown cgroup IDs produce `hostLevel=true` with only nodeName.
    - File: `src/agent/cgroup_resolver_test.go`
    - **Validates: Requirements 4.3, 4.4, 5.1**

  - [x] 2.3 Add unit test for kubelet API unreachable scenario
    - Use `httptest.NewServer` that returns 503, verify `refresh()` returns error and stale cache is retained.
    - File: `src/agent/cgroup_resolver_test.go`
    - _Requirements: 4.5_

- [x] 3. Verify and harden ProbeManager and BPFLoader (macOS-safe)
  - [x] 3.1 Verify stub loader interface parity with linux loader
    - Add a compile-time interface check ensuring both `loader_stub.go` and `loader_linux.go` satisfy the same interface: `NewBPFLoader() → Load() → Close() → Programs()`.
    - File: `src/agent/loader_stub.go` and `src/agent/loader_linux.go`
    - _Requirements: 6.1, 6.4_

  - [ ]* 3.2 Write property test for BPF Loader cleanup invariant (Property 5)
    - **Property 5: BPF Loader cleanup invariant**
    - For any BPFLoader with loaded programs, `Close()` releases all resources so `Programs()` returns empty. Second `Close()` is idempotent.
    - File: `src/agent/loader_test.go`
    - **Validates: Requirements 2.5**

  - [ ]* 3.3 Write property test for drop counter accuracy (Property 6)
    - **Property 6: Drop counter accuracy**
    - For any ProbeManager with a full event channel, each dropped event increments the counter by exactly one.
    - File: `src/agent/probe_manager_test.go`
    - **Validates: Requirements 3.4, 12.1**

  - [x] 3.4 Add unit test verifying agent continues without eBPF when Load() fails
    - Verify that when `BPFLoader.Load()` returns an error (as on macOS), the agent logs the error and the main goroutines (forwarder, resolver) still start.
    - File: `src/agent/loader_test.go`
    - _Requirements: 6.2, 6.3_

- [x] 4. Checkpoint — macOS-safe tests
  - Ensure all tests pass with `go test ./src/agent/...`. Ask the user if questions arise.

- [x] 5. Server-side event ingestion verification (macOS-safe)
  - [x] 5.1 Add unit test for server rejecting invalid JSON on `/api/ebpf/events`
    - POST malformed JSON, verify HTTP 400 with descriptive error message.
    - File: `src/server/handler_unit_test.go`
    - _Requirements: 9.2_

  - [ ]* 5.2 Write property test for server batch ingestion completeness (Property 8)
    - **Property 8: Server batch ingestion completeness**
    - For any valid JSON array of EnrichedEvents POSTed to `/api/ebpf/events`, the server persists every event and broadcasts every event via WebSocket. Count of persisted and broadcast events equals input array length.
    - File: `src/server/property_test.go`
    - **Validates: Requirements 9.1**

  - [x] 5.3 Add unit test verifying WebSocket clients receive `ebpf_event` messages
    - Connect a WebSocket client, POST a batch of EnrichedEvents, verify each arrives as a `WSMessage` with `type: "ebpf_event"`.
    - File: `src/server/ws_test.go`
    - _Requirements: 9.3_

- [x] 6. Helm chart DaemonSet enhancements (macOS-safe)
  - [x] 6.1 Add volume mounts for `/sys/fs/cgroup` and `/proc` to agent DaemonSet template
    - Mount both as `readOnly: true` hostPath volumes so the CgroupResolver can read cgroup paths.
    - File: `deploy/helm/earthworm/templates/agent-daemonset.yaml`
    - _Requirements: 8.5_

  - [x] 6.2 Add `EARTHWORM_NODE_NAME` env var from `fieldRef: spec.nodeName` to agent container
    - Also wire `--ring-buffer-size` and `--server-url` from existing env vars into the container args.
    - File: `deploy/helm/earthworm/templates/agent-daemonset.yaml`
    - _Requirements: 8.3_

  - [x] 6.3 Add Helm template tests for volume mounts, node name env, nodeSelector, and tolerations
    - Verify the DaemonSet renders correctly when `ebpf.enabled=true` (with volumes, env, capabilities) and does not render when `ebpf.enabled=false`.
    - File: `deploy/helm/earthworm/tests/values_test.go`
    - _Requirements: 8.1, 8.2, 8.4, 8.5, 8.6_

- [x] 7. Checkpoint — all macOS-safe tests pass
  - Ensure all tests pass with `go test ./src/agent/... ./src/server/... ./deploy/helm/earthworm/tests/...`. Ask the user if questions arise.

- [ ] 8. bpf2go code generation setup (Linux-only)
  - [ ] 8.1 Update `gen.go` to include `vmlinux.h` in the bpf2go include path
    - Ensure the `//go:generate` directives reference both `vmlinux.h` and `common.h` from `src/ebpf/headers/`. Add a `vmlinux.h` header (generated via `bpftool btf dump file /sys/kernel/btf/vmlinux format c`) or document how to obtain it.
    - File: `src/agent/gen.go`, `src/ebpf/headers/`
    - _Requirements: 1.1, 1.3_

  - [ ] 8.2 Verify `go generate ./src/agent/...` produces Go source and `.o` files for all four programs
    - Run `go generate` on a Linux host with clang, confirm output files exist for syscallTracer, processMonitor, networkProbe, heartbeat.
    - _Requirements: 1.1, 1.2, 1.4_

- [ ] 9. Implement real BPF program loading and attachment (Linux-only)
  - [ ] 9.1 Complete `loader_linux.go` Load() to call bpf2go-generated loaders and attach to hook points
    - Call `LoadSyscallTracerObjects()`, `LoadProcessMonitorObjects()`, `LoadNetworkProbeObjects()`, `LoadHeartbeatObjects()`. Attach each program to its designated tracepoint or kprobe via `link.Tracepoint()` / `link.Kprobe()`. Store link handles for cleanup. Open the shared ring buffer map and create a `ringbuf.Reader`.
    - File: `src/agent/loader_linux.go`
    - _Requirements: 2.1, 2.2_

  - [ ] 9.2 Add BTF prerequisite check to `loader_linux.go`
    - Check for `/sys/kernel/btf/vmlinux` before loading. Return descriptive error if missing.
    - File: `src/agent/loader_linux.go`
    - _Requirements: 11.3_

  - [ ] 9.3 Update `ProbeManager.Start()` to use `ringbuf.Reader` instead of ticker-based polling
    - Accept a `*ringbuf.Reader` from the loader. Replace the ticker loop with blocking `reader.Read()` calls. Decode each `ringbuf.Record.RawSample` via `UnmarshalBinary`, enrich via `CgroupResolver`, send to event channel.
    - File: `src/agent/probe_manager.go`
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

  - [ ] 9.4 Wire the LoadResult into main.go startup flow
    - Pass the `ringbuf.Reader` from the loader to `ProbeManager.Start()`. Log the number of attached programs and ring buffer size on successful load. Log total events processed and dropped on shutdown.
    - File: `src/agent/main.go`
    - _Requirements: 12.2, 12.3, 12.4_

- [ ] 10. Docker multi-stage build verification (Linux-only)
  - [ ] 10.1 Update Dockerfile.agent to generate/download `vmlinux.h` in the BPF builder stage
    - Add `bpftool btf dump` or download a pre-built `vmlinux.h` so eBPF programs compile with CO-RE support.
    - File: `deploy/docker/Dockerfile.agent`
    - _Requirements: 7.1, 7.2, 7.3_

  - [ ] 10.2 Verify `docker build -f deploy/docker/Dockerfile.agent .` completes and image is < 50 MB
    - Build the image, check exit code and final image size.
    - _Requirements: 7.4, 7.5_

- [ ] 11. Final checkpoint — full integration
  - Ensure all tests pass on both macOS (`go test ./src/agent/... ./src/server/... ./deploy/helm/...`) and Linux (including `go generate` and BPF loading). Ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Tasks 1–7 are macOS-safe and can be developed/tested locally
- Tasks 8–10 require a Linux host with kernel 5.8+, clang, and CAP_BPF
- Property tests use `pgregory.net/rapid` (already in go.mod) with 100+ iterations
- Each property test references its design document property number
- Existing tests from the prior `ebpf-kernel-observability` spec provide a foundation; this plan fills gaps and adds the 8 new correctness properties
