# Implementation Plan

- [x] 1. Write bug condition exploration test
  - **Property 1: Bug Condition** — UI Freeze Under Rapid WebSocket Messages
  - **CRITICAL**: This test MUST FAIL on unfixed code — failure confirms the bug exists
  - **DO NOT attempt to fix the test or the code when it fails**
  - **NOTE**: This test encodes the expected behavior — it will validate the fix when it passes after implementation
  - **GOAL**: Surface counterexamples that demonstrate excessive re-renders and recomputations when WebSocket messages arrive rapidly
  - **Scoped PBT Approach**: Simulate N rapid WebSocket messages (N ≥ 5) and measure render/recomputation counts
  - Test file: `src/heartbeat-visualizer/src/dataIngestionFreeze.property.test.tsx`
  - Test that when N WebSocket messages arrive with unchanged `leasesData`:
    - `transformLeasesForChart` is called no more than once (from Bug Condition in design: unmemoized chartData/namespaces recompute on every render)
    - `getEbpfMarkers` O(N×M×K) computation runs no more than once when ebpf inputs are unchanged (from Bug Condition: unmemoized ebpfMarkers)
    - `LineChart` does not re-render solely due to `liveEvents` state updates (from Bug Condition: no render isolation for LiveActivityPanel)
  - Run test on UNFIXED code — expect FAILURE (this confirms the bug exists)
  - **EXPECTED OUTCOME**: Test FAILS — counterexamples show `transformLeasesForChart` called N+ times, `getEbpfMarkers` called N+ times, and `LineChart` re-renders N+ times for N WebSocket messages with identical lease/ebpf data
  - Document counterexamples found to understand root cause
  - Mark task complete when test is written, run, and failure is documented
  - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [x] 2. Write preservation property tests (BEFORE implementing fix)
  - **Property 2: Preservation** — Functional Equivalence of Chart Data, eBPF Markers, and Live Events
  - **IMPORTANT**: Follow observation-first methodology
  - Test file: `src/heartbeat-visualizer/src/dataIngestionPreservation.property.test.ts`
  - **Observe on UNFIXED code**:
    - `transformLeasesForChart(leasesData, maxPoints)` produces correct `ChartDataPoint[]` with index, timestamp, and per-namespace values
    - `getEbpfMarkers(chartData, namespaces)` with `showEbpf=true` produces correct `EbpfMarker[]` with x, y, namespace, event, and offset
    - `liveEvents` array caps at 200 entries, discarding oldest, preserving order and content for heartbeat and alert messages
  - **Write property-based tests**:
    - For any valid `LeasesByNamespace` and `currentHeartbeat`, `transformLeasesForChart` output has length `currentHeartbeat + 1`, each point has correct index and namespace keys — output is identical whether called directly or through the hook
    - For any valid `chartData`, `namespaces`, and `ebpfData` arrays, `getEbpfMarkers` returns markers where each marker.namespace exists in namespaces, marker.x matches a chartData timestamp, and marker.event comes from ebpfData — output is identical between direct function call and memoized hook return
    - For any sequence of ≤ 300 WebSocket messages (mix of heartbeat/alert), the resulting `liveEvents` array has length ≤ 200, preserves insertion order, and contains the most recent 200 events
  - Run tests on UNFIXED code
  - **EXPECTED OUTCOME**: Tests PASS (this confirms baseline behavior to preserve)
  - Mark task complete when tests are written, run, and passing on unfixed code
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7_

- [x] 3. Fix for UI freeze during WebSocket data ingestion

  - [x] 3.1 Memoize `chartData` and `namespaces` in `useHeartbeatData`
    - File: `src/heartbeat-visualizer/src/hooks/useHeartbeatData.ts`
    - Import `useMemo` from React
    - Wrap `Object.keys(leasesData)` in `useMemo` with dependency `[leasesData]` so `namespaces` is a stable reference
    - Wrap `transformLeasesForChart(leasesData, currentHeartbeat + 1)` in `useMemo` with dependencies `[leasesData, currentHeartbeat]` so `chartData` is a stable reference
    - _Bug_Condition: isBugCondition(state) where NOT state.chartDataMemoized — chartData and namespaces recompute on every render even when leasesData/currentHeartbeat unchanged_
    - _Expected_Behavior: chartData and namespaces only recompute when leasesData or currentHeartbeat change_
    - _Preservation: transformLeasesForChart output must be identical for same inputs; namespaces must contain same keys_
    - _Requirements: 2.2_

  - [x] 3.2 Memoize `ebpfMarkers` in `useEbpfData` and return pre-computed result
    - File: `src/heartbeat-visualizer/src/hooks/useEbpfData.ts`
    - Import `useMemo` from React
    - Accept `chartData` and `namespaces` as parameters to `useEbpfData` hook
    - Replace the `getEbpfMarkers` `useCallback` with a `useMemo` that computes `ebpfMarkers` directly, with dependencies `[showEbpf, ebpfData, chartData, namespaces]`
    - Return `ebpfMarkers: EbpfMarker[]` instead of `getEbpfMarkers` function from the hook
    - Update `UseEbpfDataReturn` interface: replace `getEbpfMarkers` with `ebpfMarkers: EbpfMarker[]`
    - File: `src/heartbeat-visualizer/src/HeartbeatChart.tsx`
    - Update `useEbpfData` call to pass `chartData` and `namespaces` as arguments
    - Replace `const ebpfMarkers = getEbpfMarkers(chartData, namespaces)` with destructured `ebpfMarkers` from hook return
    - _Bug_Condition: isBugCondition(state) where NOT state.ebpfMarkersMemoized — O(N×M×K) getEbpfMarkers runs on every render because chartData is a new reference_
    - _Expected_Behavior: ebpfMarkers only recomputes when showEbpf, ebpfData, chartData, or namespaces actually change_
    - _Preservation: getEbpfMarkers output must be identical for same inputs; marker positions, shapes, and event data unchanged_
    - _Requirements: 2.3_

  - [x] 3.3 Isolate `LiveActivityPanel` rendering from chart rendering
    - File: `src/heartbeat-visualizer/src/LiveActivityPanel.tsx`
    - Wrap the default export with `React.memo` so `LiveActivityPanel` only re-renders when its props (`events`, `wsStatus`) change
    - File: `src/heartbeat-visualizer/src/HeartbeatChart.tsx`
    - Move `liveEvents` and `alerts` state, and the `useEffect` that processes `lastMessage`, out of `HeartbeatChart` into a new `useLiveEvents` hook or a sibling wrapper component
    - This ensures that `setLiveEvents` / `setAlerts` state updates from WebSocket messages do not trigger re-renders of the `LineChart`, `ReferenceDot` markers, or legend
    - _Bug_Condition: isBugCondition(state) where NOT state.liveActivityIsolated — every setLiveEvents call re-renders the entire HeartbeatChart tree_
    - _Expected_Behavior: liveEvents updates only re-render LiveActivityPanel; chart/legend/markers unaffected_
    - _Preservation: LiveActivityPanel continues to display events with correct timestamp, node name, namespace, status; alerts continue to trigger toasts with correct severity_
    - _Requirements: 2.1, 2.4_

  - [x] 3.4 Batch/throttle WebSocket state updates in HeartbeatChart
    - File: `src/heartbeat-visualizer/src/HeartbeatChart.tsx` (or the new `useLiveEvents` hook)
    - Accumulate incoming WebSocket messages in a ref buffer
    - Flush the buffer on a `requestAnimationFrame` or throttled interval (e.g., every 100–200ms) to batch multiple messages into a single state update
    - This prevents N rapid messages from causing N separate render cycles
    - _Bug_Condition: isBugCondition(state) where state.wsMessageRate > 1 — each message triggers individual setState calls and separate renders_
    - _Expected_Behavior: rapid messages are batched into fewer state updates, keeping the main thread unblocked_
    - _Preservation: all messages still appear in liveEvents in correct order; no messages are dropped; 200-cap behavior preserved_
    - _Requirements: 2.1_

  - [x] 3.5 Verify bug condition exploration test now passes
    - **Property 1: Expected Behavior** — UI Responsiveness Under Sustained WebSocket Load
    - **IMPORTANT**: Re-run the SAME test from task 1 — do NOT write a new test
    - The test from task 1 encodes the expected behavior: memoized computations and isolated rendering
    - When this test passes, it confirms the expected behavior is satisfied
    - Run bug condition exploration test from step 1: `src/heartbeat-visualizer/src/dataIngestionFreeze.property.test.tsx`
    - **EXPECTED OUTCOME**: Test PASSES (confirms bug is fixed)
    - _Requirements: 2.1, 2.2, 2.3, 2.4_

  - [x] 3.6 Verify preservation tests still pass
    - **Property 2: Preservation** — Functional Equivalence of Chart Data, eBPF Markers, and Live Events
    - **IMPORTANT**: Re-run the SAME tests from task 2 — do NOT write new tests
    - Run preservation property tests from step 2: `src/heartbeat-visualizer/src/dataIngestionPreservation.property.test.ts`
    - **EXPECTED OUTCOME**: Tests PASS (confirms no regressions)
    - Confirm all tests still pass after fix (no regressions)

- [x] 4. Checkpoint — Ensure all tests pass
  - Run full test suite to verify no regressions across the heartbeat-visualizer
  - Ensure exploration test (task 1) passes after fix
  - Ensure preservation tests (task 2) still pass after fix
  - Ensure existing tests (chartUtils, anomalyBadge, viewIntegration, heatmapView, histogramView) still pass
  - Ask the user if questions arise
