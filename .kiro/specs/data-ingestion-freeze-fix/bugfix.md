# Bugfix Requirements Document

## Introduction

The Heartbeat Visualizer frontend freezes during data ingestion. When WebSocket messages arrive at a moderate-to-high rate, the UI becomes unresponsive because each incoming message triggers cascading state updates and expensive re-renders of the entire HeartbeatChart component tree. The root cause is a combination of unbatched per-message state updates, unmemoized expensive computations (chart data transformation, eBPF marker calculation), and the lack of component isolation — meaning every WebSocket event re-renders the full chart, all reference dots, the legend, and the live activity panel simultaneously.

## Bug Analysis

### Current Behavior (Defect)

1.1 WHEN WebSocket messages arrive at a sustained rate (e.g., multiple messages per second) THEN the system freezes the UI because each message triggers individual `setLiveEvents` and `setAlerts` state updates in HeartbeatChart, causing a full re-render of the entire component tree including the expensive LineChart, all ReferenceDot markers, and the legend on every single message.

1.2 WHEN the HeartbeatChart component re-renders due to any state change (including WebSocket messages) THEN the system recomputes `chartData` and `namespaces` from scratch via `transformLeasesForChart` because these values are derived inline without memoization in `useHeartbeatData`, even when the underlying `leasesData` and `currentHeartbeat` have not changed.

1.3 WHEN the HeartbeatChart component re-renders THEN the system recomputes `ebpfMarkers` via the O(N×M×K) `getEbpfMarkers` nested loop (chartData points × namespaces × ebpfData events) inline in the render path, because the result is not memoized and `chartData` is a new array reference on every render.

1.4 WHEN the `liveEvents` array is updated via `setLiveEvents((prev) => [...prev.slice(-199), newEvent])` THEN the system creates a new 200-element array and re-renders the entire HeartbeatChart component (including the chart, markers, legend, and controls) rather than only the LiveActivityPanel, because LiveActivityPanel is not isolated from the parent component's render cycle.

### Expected Behavior (Correct)

2.1 WHEN WebSocket messages arrive at a sustained rate THEN the system SHALL batch or throttle incoming message processing so that state updates from rapid WebSocket messages do not trigger individual re-renders for each message, keeping the UI responsive and the main thread unblocked.

2.2 WHEN the HeartbeatChart component re-renders due to state changes unrelated to lease data THEN the system SHALL return memoized `chartData` and `namespaces` values from `useHeartbeatData` (e.g., via `useMemo`) so that `transformLeasesForChart` is only recomputed when `leasesData` or `currentHeartbeat` actually change.

2.3 WHEN the HeartbeatChart component re-renders THEN the system SHALL use a memoized `ebpfMarkers` result (e.g., via `useMemo`) so that the O(N×M×K) `getEbpfMarkers` computation only runs when its inputs (`chartData`, `namespaces`, `showEbpf`, `ebpfData`) actually change, not on every render.

2.4 WHEN `liveEvents` state is updated from a WebSocket message THEN the system SHALL isolate the LiveActivityPanel rendering from the chart rendering so that live event updates do not cause the expensive LineChart, ReferenceDot markers, and legend to re-render. This can be achieved through component memoization (e.g., `React.memo`) or by lifting live event state out of HeartbeatChart.

### Unchanged Behavior (Regression Prevention)

3.1 WHEN a WebSocket message of type 'heartbeat' is received THEN the system SHALL CONTINUE TO append it to the live events feed and display it in the LiveActivityPanel with the correct timestamp, node name, namespace, and status.

3.2 WHEN a WebSocket message of type 'alert' is received THEN the system SHALL CONTINUE TO display a toast notification with the correct severity, node name, namespace, and gap duration, and append it to the live events feed.

3.3 WHEN the animation step transitions from 'sync' to 'animate' THEN the system SHALL CONTINUE TO progressively reveal chart data points at the configured `heartbeatInterval` rate with correct namespace lines, colors, and death/warning indicators.

3.4 WHEN eBPF data is loaded and `showEbpf` is enabled THEN the system SHALL CONTINUE TO render eBPF markers at the correct chart positions with the correct shapes (circle, exit ✖, fork ★) and respond to hover and click interactions.

3.5 WHEN the user clicks a namespace in the legend THEN the system SHALL CONTINUE TO isolate that namespace's line in the chart and highlight it with the correct hover color.

3.6 WHEN the user performs brush-zoom or shift-pan on the chart THEN the system SHALL CONTINUE TO update the X-axis domain correctly and share the zoom state across views via ViewContext.

3.7 WHEN the `liveEvents` array reaches 200 entries THEN the system SHALL CONTINUE TO cap it at 200 by discarding the oldest events, preventing unbounded memory growth.
