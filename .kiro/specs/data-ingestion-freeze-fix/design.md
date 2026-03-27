# Data Ingestion Freeze Fix — Bugfix Design

## Overview

The Heartbeat Visualizer UI freezes during WebSocket data ingestion because every incoming message triggers cascading, unbatched state updates that re-render the entire `HeartbeatChart` component tree — including the expensive `LineChart`, all `ReferenceDot` eBPF markers, and the legend. The fix targets four root causes: unbatched per-message state updates, unmemoized `chartData`/`namespaces` derivation, unmemoized O(N×M×K) `ebpfMarkers` computation, and lack of render isolation between `LiveActivityPanel` and the chart. The approach is to batch WebSocket updates, memoize expensive derivations, and isolate the live-event rendering path from the chart rendering path.

## Glossary

- **Bug_Condition (C)**: The condition under which the UI freezes — rapid WebSocket messages causing per-message re-renders of the full component tree with unmemoized expensive computations.
- **Property (P)**: The UI remains responsive during sustained WebSocket message ingestion; expensive computations only run when their inputs change.
- **Preservation**: All existing behaviors — chart animation, eBPF marker rendering, legend interaction, zoom/pan, toast alerts, live event feed — must remain functionally identical.
- **HeartbeatChart**: The main component in `src/heartbeat-visualizer/src/HeartbeatChart.tsx` that renders the chart, controls, legend, eBPF markers, and live activity panel.
- **useHeartbeatData**: Hook in `src/heartbeat-visualizer/src/hooks/useHeartbeatData.ts` that fetches lease data and derives `chartData`/`namespaces` inline without memoization.
- **useEbpfData**: Hook in `src/heartbeat-visualizer/src/hooks/useEbpfData.ts` that provides `getEbpfMarkers`, a function computing O(N×M×K) markers.
- **LiveActivityPanel**: Component in `src/heartbeat-visualizer/src/LiveActivityPanel.tsx` that displays the live WebSocket event feed.
- **transformLeasesForChart**: Pure function in `src/heartbeat-visualizer/src/services/dataService.ts` that transforms `LeasesByNamespace` into `ChartDataPoint[]`.

## Bug Details

### Bug Condition

The bug manifests when WebSocket messages arrive at a sustained rate (multiple per second). Each message triggers `setLiveEvents` and/or `setAlerts` inside `HeartbeatChart`, causing a full re-render. During that render, `chartData` and `namespaces` are recomputed from scratch, `getEbpfMarkers` runs its O(N×M×K) nested loop, and `LiveActivityPanel` re-renders alongside the chart — all for a state change that only affects the live feed.

**Formal Specification:**
```
FUNCTION isBugCondition(state)
  INPUT: state of type { wsMessageRate: number, chartDataMemoized: boolean, ebpfMarkersMemoized: boolean, liveActivityIsolated: boolean }
  OUTPUT: boolean

  RETURN state.wsMessageRate > 1
         AND (
           NOT state.chartDataMemoized
           OR NOT state.ebpfMarkersMemoized
           OR NOT state.liveActivityIsolated
         )
END FUNCTION
```

### Examples

- **Example 1**: 5 heartbeat WebSocket messages arrive within 1 second. Each triggers `setLiveEvents`, causing 5 full re-renders of `HeartbeatChart`. Each render recomputes `transformLeasesForChart` (even though `leasesData` hasn't changed) and runs `getEbpfMarkers` over 300 chart points × 20 namespaces × 50 eBPF events = 300,000 iterations. **Expected**: UI stays responsive; chart only re-renders when lease data changes. **Actual**: UI freezes for several seconds.

- **Example 2**: An alert WebSocket message arrives. `setAlerts` triggers a re-render. The chart, legend, all eBPF markers, and `LiveActivityPanel` all re-render even though only a toast notification needs to appear. **Expected**: Only the toast and live feed update. **Actual**: Full component tree re-renders with expensive recomputations.

- **Example 3**: `currentHeartbeat` increments during animation. `chartData` is recomputed (correctly), but `ebpfMarkers` is also recomputed even though `ebpfData` and `showEbpf` haven't changed — the new `chartData` array reference defeats the `useCallback` dependency. **Expected**: `ebpfMarkers` recomputes only when eBPF-relevant inputs change. **Actual**: Recomputes on every heartbeat tick.

- **Edge case**: A single WebSocket message arrives while the chart is paused with no data (`step === 'nodata'`). Even in this state, `setLiveEvents` triggers a re-render of the full component, though the chart section is conditionally hidden. **Expected**: Minimal render cost. **Actual**: Still runs through all hook computations.

## Expected Behavior

### Preservation Requirements

**Unchanged Behaviors:**
- WebSocket heartbeat messages continue to appear in `LiveActivityPanel` with correct timestamp, node name, namespace, and status
- WebSocket alert messages continue to trigger toast notifications with correct severity, node name, namespace, and gap duration
- Chart animation progressively reveals data points at the configured `heartbeatInterval` rate with correct namespace lines, colors, and death/warning indicators
- eBPF markers render at correct chart positions with correct shapes (circle, exit ✖, fork ★) and respond to hover/click
- Legend click isolates a namespace line with the correct hover color
- Brush-zoom and shift-pan update the X-axis domain correctly and share zoom state via `ViewContext`
- The `liveEvents` array caps at 200 entries, discarding oldest events

**Scope:**
All functional behavior of the application must remain identical. The fix only changes _when_ computations run and _which_ components re-render — not _what_ they compute or display.

## Hypothesized Root Cause

Based on the bug description and code analysis, the four root causes are:

1. **Unbatched per-message WebSocket state updates**: In `HeartbeatChart`, the `useEffect` listening to `lastMessage` calls `setAlerts` and `setLiveEvents` individually for each message. React 18 batches state updates inside event handlers but the WebSocket `onmessage` callback in `useWebSocket` calls `setLastMessage` which triggers the effect — each message produces a separate render cycle.

2. **Unmemoized `chartData`/`namespaces` in `useHeartbeatData`**: Lines at the bottom of `useHeartbeatData.ts` compute `namespaces` and `chartData` as plain inline expressions:
   ```ts
   const namespaces = leasesData ? Object.keys(leasesData) : [];
   const chartData = leasesData ? transformLeasesForChart(leasesData, currentHeartbeat + 1) : [];
   ```
   These create new array references on every render, even when `leasesData` and `currentHeartbeat` haven't changed.

3. **Unmemoized O(N×M×K) `ebpfMarkers` calculation**: `getEbpfMarkers` is wrapped in `useCallback` in `useEbpfData`, but it's called inline in `HeartbeatChart`'s render body: `const ebpfMarkers = getEbpfMarkers(chartData, namespaces)`. Since `chartData` is a new reference every render (root cause 2), the callback runs every time. The nested loop iterates `chartData.length × namespaces.length × ebpfData.length`.

4. **No render isolation between `LiveActivityPanel` and the chart**: `LiveActivityPanel` receives `liveEvents` as a prop from `HeartbeatChart`. When `liveEvents` changes (every WebSocket message), the entire `HeartbeatChart` re-renders, including the `LineChart`, all `ReferenceDot` markers, and the legend. `LiveActivityPanel` is not wrapped in `React.memo`, and the live event state lives inside `HeartbeatChart`.

## Correctness Properties

Property 1: Bug Condition — UI Responsiveness Under Sustained WebSocket Load

_For any_ sequence of WebSocket messages arriving at a rate greater than 1 message per second, the fixed `HeartbeatChart` component SHALL NOT recompute `chartData`, `namespaces`, or `ebpfMarkers` unless their respective inputs (`leasesData`, `currentHeartbeat`, `ebpfData`, `showEbpf`) have actually changed, and SHALL NOT re-render the `LineChart` or `ReferenceDot` markers solely due to live event state updates.

**Validates: Requirements 2.1, 2.2, 2.3, 2.4**

Property 2: Preservation — Functional Equivalence of Rendered Output

_For any_ application state (lease data, eBPF data, animation step, user interactions), the fixed code SHALL produce the same rendered output (chart lines, eBPF markers, legend, toasts, live events, zoom state) as the original code, preserving all existing functionality for chart rendering, user interactions, and data display.

**Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7**

## Fix Implementation

### Changes Required

Assuming our root cause analysis is correct:

**File**: `src/heartbeat-visualizer/src/hooks/useHeartbeatData.ts`

**Function**: `useHeartbeatData`

**Specific Changes**:
1. **Memoize `namespaces`**: Wrap `Object.keys(leasesData)` in `useMemo` with `[leasesData]` as the dependency so it only recomputes when lease data changes.
2. **Memoize `chartData`**: Wrap `transformLeasesForChart(leasesData, currentHeartbeat + 1)` in `useMemo` with `[leasesData, currentHeartbeat]` as dependencies so the expensive transformation only runs when its inputs change.

---

**File**: `src/heartbeat-visualizer/src/hooks/useEbpfData.ts`

**Function**: `useEbpfData` / `getEbpfMarkers`

**Specific Changes**:
3. **Replace `getEbpfMarkers` callback with a memoized result**: Instead of exposing a `getEbpfMarkers` function that is called inline during render, compute the markers inside the hook using `useMemo` and return the memoized `ebpfMarkers` array directly. The dependencies should be `[showEbpf, ebpfData, chartData, namespaces]` — but since `chartData` and `namespaces` are now stable references (from fix 1–2), this will only recompute when eBPF or chart data actually changes. Alternatively, accept `chartData` and `namespaces` as hook parameters and memoize internally.

---

**File**: `src/heartbeat-visualizer/src/HeartbeatChart.tsx`

**Function**: `HeartbeatChart`

**Specific Changes**:
4. **Batch/throttle WebSocket state updates**: Refactor the `useEffect` that processes `lastMessage` to batch rapid updates. Options include: (a) accumulating messages in a ref and flushing on a `requestAnimationFrame` or throttled interval, or (b) moving live event state out of `HeartbeatChart` entirely into a dedicated context or the `useWebSocket` hook, so that live event updates don't trigger `HeartbeatChart` re-renders.
5. **Isolate `LiveActivityPanel` rendering**: Either (a) wrap `LiveActivityPanel` in `React.memo` so it only re-renders when its props change, combined with moving `liveEvents` state out of `HeartbeatChart`, or (b) extract `LiveActivityPanel` into a sibling component that manages its own WebSocket subscription independently. The goal is that a new live event does not cause the `LineChart`, `ReferenceDot` markers, or legend to re-render.
6. **Consume memoized `ebpfMarkers` from hook**: Replace the inline `const ebpfMarkers = getEbpfMarkers(chartData, namespaces)` call with the pre-computed memoized value returned from `useEbpfData`.

## Testing Strategy

### Validation Approach

The testing strategy follows a two-phase approach: first, surface counterexamples that demonstrate the bug on unfixed code, then verify the fix works correctly and preserves existing behavior.

### Exploratory Bug Condition Checking

**Goal**: Surface counterexamples that demonstrate the performance bug BEFORE implementing the fix. Confirm or refute the root cause analysis. If we refute, we will need to re-hypothesize.

**Test Plan**: Write tests that measure render counts and computation frequency when WebSocket messages arrive rapidly. Run these tests on the UNFIXED code to observe excessive re-renders and recomputations.

**Test Cases**:
1. **Render Count Under Load**: Simulate 10 rapid WebSocket messages and count how many times `HeartbeatChart` renders — expect excessive renders on unfixed code (will fail threshold on unfixed code)
2. **chartData Recomputation**: Verify that `transformLeasesForChart` is called on every render even when `leasesData` hasn't changed (will show unnecessary calls on unfixed code)
3. **ebpfMarkers Recomputation**: Verify that `getEbpfMarkers` runs on every render even when eBPF data hasn't changed (will show unnecessary calls on unfixed code)
4. **LiveActivityPanel Isolation**: Verify that updating `liveEvents` causes the `LineChart` to re-render (will show coupling on unfixed code)

**Expected Counterexamples**:
- 10 WebSocket messages produce 10+ full re-renders of the chart component
- `transformLeasesForChart` is called 10+ times with identical inputs
- `getEbpfMarkers` runs 10+ times with identical eBPF data
- `LineChart` re-renders when only `liveEvents` changed

### Fix Checking

**Goal**: Verify that for all inputs where the bug condition holds, the fixed code produces the expected responsive behavior.

**Pseudocode:**
```
FOR ALL input WHERE isBugCondition(input) DO
  result := HeartbeatChart_fixed(input)
  ASSERT chartDataRecomputeCount <= expectedRecomputeCount(input)
  ASSERT ebpfMarkersRecomputeCount <= expectedRecomputeCount(input)
  ASSERT lineChartRenderCount NOT increased by liveEvent updates
END FOR
```

### Preservation Checking

**Goal**: Verify that for all inputs where the bug condition does NOT hold, the fixed code produces the same result as the original code.

**Pseudocode:**
```
FOR ALL input WHERE NOT isBugCondition(input) DO
  ASSERT HeartbeatChart_original(input).renderedOutput = HeartbeatChart_fixed(input).renderedOutput
END FOR
```

**Testing Approach**: Property-based testing is recommended for preservation checking because:
- It generates many test cases automatically across the input domain
- It catches edge cases that manual unit tests might miss
- It provides strong guarantees that behavior is unchanged for all non-buggy inputs

**Test Plan**: Observe behavior on UNFIXED code first for chart rendering, eBPF markers, legend interactions, and zoom/pan, then write property-based tests capturing that behavior.

**Test Cases**:
1. **Chart Data Equivalence**: For any `leasesData` and `currentHeartbeat`, verify `transformLeasesForChart` output is identical before and after the fix
2. **eBPF Marker Equivalence**: For any combination of `chartData`, `namespaces`, and `ebpfData`, verify `getEbpfMarkers` output is identical before and after the fix
3. **Live Event Feed Equivalence**: For any sequence of WebSocket messages, verify the `liveEvents` array contents and ordering are identical
4. **Alert Toast Equivalence**: For any alert WebSocket message, verify the toast renders with the same severity, node name, namespace, and gap

### Unit Tests

- Test that `useMemo` in `useHeartbeatData` returns stable references when `leasesData` and `currentHeartbeat` are unchanged
- Test that memoized `ebpfMarkers` returns stable references when eBPF inputs are unchanged
- Test that `LiveActivityPanel` wrapped in `React.memo` does not re-render when unrelated props change
- Test that WebSocket message batching correctly accumulates and flushes events

### Property-Based Tests

- Generate random `LeasesByNamespace` objects and `currentHeartbeat` values; verify `transformLeasesForChart` output is identical between direct call and memoized hook return
- Generate random `ebpfData` arrays and `chartData` arrays; verify `getEbpfMarkers` output is identical between direct call and memoized hook return
- Generate random sequences of WebSocket messages; verify the final `liveEvents` array is identical (same contents, same 200-cap behavior) between original and fixed code

### Integration Tests

- Test full chart rendering with simulated WebSocket stream: verify chart lines, eBPF markers, legend, and live feed all display correctly
- Test that zoom/pan state via `ViewContext` continues to work correctly after the fix
- Test that switching between views (line, heatmap, timeline, histogram, table) works correctly with memoized data
