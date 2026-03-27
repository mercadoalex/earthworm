# Tasks: Realistic Data and Visualizations

## Task 1: Go Simulation Engine Core Types and Config
- [x] 1.1 Define `NodeHealthProfile`, `NotReadyCause`, `SimulationConfig`, `SimNode`, `LeaseEvent`, `SimEbpfEvent`, `SimulationResult`, and related types in a new file `src/kubernetes/simulation.go`
- [x] 1.2 Implement `SimulationConfig` validation (duration bounds, node count > 0, namespace ratio normalization, max 7 days)
- [x] 1.3 Implement `NewSimulationEngine()` constructor that initializes nodes with health profiles distributed across namespaces per configured ratios
- [x] 1.4 Write unit tests for config validation edge cases (zero nodes, duration > 7 days, ratios not summing to 1.0)

## Task 2: Node State Transitions and Lease Generation
- [x] 2.1 Implement `applyHealthTransitions()` — probabilistic NotReady transitions based on health profile, with cause assignment and duration bounds (5–120s)
- [x] 2.2 Implement `generateLeaseForNode()` — base interval 10s + normal-distribution jitter (configurable stddev), skip leases during NotReady, apply drift for "drifting" profile
- [x] 2.3 Implement the main `tick()` loop that advances the simulation clock and calls transition/lease logic
- [x] 2.4 Implement `Run()` method that executes the full simulation and returns `SimulationResult`
- [x] 2.5 Write property test: stable nodes never go NotReady (Property 5)
  - [x] 🧪 PBT: Property 5 — Stable nodes never go NotReady
- [x] 2.6 Write property test: NotReady duration bounds 5–120s (Property 3)
  - [x] 🧪 PBT: Property 3 — NotReady duration bounds
- [x] 2.7 Write property test: monotonically increasing lease timestamps (Property 9)
  - [x] 🧪 PBT: Property 9 — Monotonically increasing lease timestamps
- [x] 2.8 Write property test: no leases during NotReady (Property 7)
  - [x] 🧪 PBT: Property 7 — No leases during NotReady

## Task 3: Rolling Deployment Scenarios
- [x] 3.1 Implement `applyScenarios()` — trigger rolling deployment at configured time, drain nodes sequentially with stagger interval, introduce replacement nodes with burst leases
- [x] 3.2 Write property test: rolling deployment stagger interval (Property 10)
  - [x] 🧪 PBT: Property 10 — Rolling deployment stagger
- [x] 3.3 Write property test: drain stops leases and replacement appears (Property 11)
  - [x] 🧪 PBT: Property 11 — Drain stops leases and replacement appears
- [x] 3.4 Write property test: node count invariant ±2 during rolling deployment (Property 12)
  - [x] 🧪 PBT: Property 12 — Node count invariant during rolling deployment
- [x] 3.5 Write property test: replacement node initial burst of 3 leases in 5s (Property 13)
  - [x] 🧪 PBT: Property 13 — Replacement node initial burst

## Task 4: Correlated eBPF Event Generation
- [x] 4.1 Implement `generateCorrelatedEbpf()` — produce eBPF events correlated with NotReady causes (kubelet_restart→exit, oom_kill→kill, replacement→fork) and background write events per lease
- [x] 4.2 Write property test: correlated eBPF for kubelet_restart (Property 16)
  - [x] 🧪 PBT: Property 16 — Correlated eBPF for kubelet_restart
- [x] 4.3 Write property test: correlated eBPF for oom_kill (Property 17)
  - [x] 🧪 PBT: Property 17 — Correlated eBPF for oom_kill
- [x] 4.4 Write property test: correlated eBPF for replacement fork (Property 18)
  - [x] 🧪 PBT: Property 18 — Correlated eBPF for replacement node fork
- [x] 4.5 Write property test: background eBPF count equals lease count (Property 19)
  - [x] 🧪 PBT: Property 19 — Background eBPF event count
- [x] 4.6 Write property test: all eBPF event fields valid (Property 20)
  - [x] 🧪 PBT: Property 20 — eBPF event field validity

## Task 5: Multi-Namespace Profiles and Cluster-Level Properties
- [x] 5.1 Implement namespace-aware node distribution and per-namespace health profile assignment (kube-system=stable, production=normal, staging=volatile)
- [x] 5.2 Write property test: namespace count and distribution (Property 14)
  - [x] 🧪 PBT: Property 14 — Namespace count and distribution
- [x] 5.3 Write property test: namespace NotReady rate matches profile (Property 15)
  - [x] 🧪 PBT: Property 15 — Namespace NotReady rate matches profile
- [x] 5.4 Write property test: cluster NotReady rate 5–20% (Property 4)
  - [x] 🧪 PBT: Property 4 — Cluster NotReady rate
- [x] 5.5 Write property test: health profile assignment (Property 1) and NotReady cause validity (Property 2)
  - [x] 🧪 PBT: Property 1 — Health profile assignment
  - [x] 🧪 PBT: Property 2 — NotReady cause validity
- [x] 5.6 Write property test: lease interval base and jitter (Property 6)
  - [x] 🧪 PBT: Property 6 — Lease interval base and jitter
- [x] 5.7 Write property test: drifting node interval increase (Property 8)
  - [x] 🧪 PBT: Property 8 — Drifting node interval increase

## Task 6: File Output and Manifest Generation
- [x] 6.1 Implement file segmentation — split simulation output into time-windowed JSON files (default 5 min) in LeasesByNamespace format, plus eBPF event files
- [x] 6.2 Implement manifest generation — produce `leases.manifest.json` and `ebpf-leases.manifest.json` listing files in chronological order
- [x] 6.3 Write property test: simulation duration and file segmentation (Property 21)
  - [x] 🧪 PBT: Property 21 — Simulation duration and file segmentation
- [x] 6.4 Write property test: output format round-trip (Property 22)
  - [x] 🧪 PBT: Property 22 — Output format round-trip
- [x] 6.5 Write property test: scenario injection rate (Property 23)
  - [x] 🧪 PBT: Property 23 — Scenario injection rate
- [x] 6.6 Write unit tests for file output with known small simulation (verify JSON schema, manifest ordering)

## Task 7: TypeScript Types and ViewContext
- [x] 7.1 Add new types to `src/heartbeat-visualizer/src/types/heartbeat.ts`: `ViewType`, `HeatmapCell`, `SwimSegment`, `NodeSummary`, `HistogramBin`, `NodeAnomaly`
- [x] 7.2 Create `ViewContext` (React context) in `src/heartbeat-visualizer/src/contexts/ViewContext.tsx` with `activeView`, `setActiveView`, `xDomain`, `setXDomain`
- [x] 7.3 Create `ViewSelector` component in `src/heartbeat-visualizer/src/ViewSelector.tsx` with tabs for all 5 view types
- [x] 7.4 Integrate `ViewContext` into `App.tsx` — wrap main content, default to 'line' view
- [x] 7.5 Write unit test: ViewSelector renders all 5 options, ViewContext defaults to 'line'

## Task 8: Data Transformation Functions
- [x] 8.1 Implement `buildHeatmapData()` in `src/heartbeat-visualizer/src/utils/heatmapUtils.ts` — transform LeasesByNamespace into HeatmapCell grid
- [x] 8.2 Implement `buildSwimSegments()` in `src/heartbeat-visualizer/src/utils/timelineUtils.ts` — transform leases into contiguous SwimSegment arrays per node
- [x] 8.3 Implement `buildHistogramBins()` in `src/heartbeat-visualizer/src/utils/histogramUtils.ts` — compute interval frequency distribution with configurable bin width
- [x] 8.4 Implement `buildNodeSummaries()` in `src/heartbeat-visualizer/src/utils/nodeTableUtils.ts` — extract per-node summary with last 20 intervals
- [x] 8.5 Extend `getAnomalies()` in `chartUtils.ts` to return `NodeAnomaly[]` with per-node information
- [x] 8.6 Implement `getSparklineColor()` in `src/heartbeat-visualizer/src/utils/sparklineUtils.ts` — threshold-based color for each interval segment
- [x] 8.7 Implement `getStatusColor()` in `chartUtils.ts` — unified status-to-color mapping used by all views

## Task 9: Heatmap View Component
- [x] 9.1 Create `HeatmapView` component in `src/heartbeat-visualizer/src/views/HeatmapView.tsx` using Recharts ScatterChart with custom cell shapes
- [x] 9.2 Implement heatmap tooltip (node name, time range, status, heartbeat count)
- [x] 9.3 Implement default sort by worst health status, with alphabetical sort option
- [x] 9.4 Implement vertical scrolling for >30 nodes with fixed time axis header
- [x] 9.5 Implement anomaly highlighting with pulsing border CSS on anomaly cells
- [x] 9.6 Write property test: heatmap grid dimensions (Property 24)
  - [x] 🧪 PBT: Property 24 — Heatmap grid dimensions
- [x] 9.7 Write property test: heatmap tooltip data completeness (Property 26)
  - [x] 🧪 PBT: Property 26 — Heatmap tooltip data completeness
- [x] 9.8 Write property test: heatmap default sort order (Property 27)
  - [x] 🧪 PBT: Property 27 — Heatmap default sort order

## Task 10: Timeline View Component
- [x] 10.1 Create `TimelineView` component in `src/heartbeat-visualizer/src/views/TimelineView.tsx` with custom SVG swimlanes sharing the time axis from ViewContext
- [x] 10.2 Implement gap detail panel (click on gap segment → show duration, cause, surrounding timestamps)
- [x] 10.3 Implement eBPF event marker overlay on swimlanes
- [x] 10.4 Implement anomaly highlighting on gap segments (increased opacity, contrasting border)
- [x] 10.5 Write property test: swimlane segment coverage (Property 28)
  - [x] 🧪 PBT: Property 28 — Swimlane segment coverage
- [x] 10.6 Write property test: eBPF marker placement on swimlane (Property 29)
  - [x] 🧪 PBT: Property 29 — eBPF marker placement on swimlane

## Task 11: Node Table with Sparklines
- [x] 11.1 Create `Sparkline` component in `src/heartbeat-visualizer/src/components/Sparkline.tsx` — 120×30px Recharts LineChart with threshold-based segment coloring
- [x] 11.2 Create `NodeTable` component in `src/heartbeat-visualizer/src/views/NodeTable.tsx` — table with Node Name, Namespace, Status, Last Heartbeat, Sparkline columns
- [x] 11.3 Wire NodeTable to WebSocket updates via `useWebSocket` hook for real-time sparkline refresh
- [x] 11.4 Write property test: node summary table completeness (Property 30)
  - [x] 🧪 PBT: Property 30 — Node summary table completeness
- [x] 11.5 Write property test: sparkline threshold coloring (Property 31)
  - [x] 🧪 PBT: Property 31 — Sparkline threshold coloring

## Task 12: Histogram View Component
- [x] 12.1 Create `HistogramView` component in `src/heartbeat-visualizer/src/views/HistogramView.tsx` using Recharts BarChart with severity-colored bins
- [x] 12.2 Implement namespace filtering integration with existing namespace selector
- [x] 12.3 Implement histogram tooltip (interval range, count, percentage)
- [x] 12.4 Write property test: histogram bin count conservation (Property 33)
  - [x] 🧪 PBT: Property 33 — Histogram bin count conservation
- [x] 12.5 Write property test: histogram bin width consistency (Property 34)
  - [x] 🧪 PBT: Property 34 — Histogram bin width consistency
- [x] 12.6 Write property test: histogram namespace filtering (Property 35)
  - [x] 🧪 PBT: Property 35 — Histogram namespace filtering
- [x] 12.7 Write property test: histogram bin severity classification (Property 36)
  - [x] 🧪 PBT: Property 36 — Histogram bin severity classification
- [x] 12.8 Write property test: histogram tooltip data completeness (Property 37)
  - [x] 🧪 PBT: Property 37 — Histogram tooltip data completeness

## Task 13: Anomaly Badge and Cross-View Highlighting
- [x] 13.1 Create `AnomalyBadge` component in `src/heartbeat-visualizer/src/components/AnomalyBadge.tsx` — displays count, click zooms to most recent anomaly
- [x] 13.2 Integrate AnomalyBadge into all view layouts (positioned above active chart)
- [x] 13.3 Write property test: anomaly badge count (Property 32)
  - [x] 🧪 PBT: Property 32 — Anomaly badge count
- [x] 13.4 Write unit test: AnomalyBadge click selects most recent anomaly

## Task 14: View Switching and Layout Integration
- [x] 14.1 Update `HeartbeatChart.tsx` to use ViewContext for zoom/pan state instead of local state
- [x] 14.2 Integrate ViewSelector and all view components into the main chart area — render active view based on ViewContext
- [x] 14.3 Render NodeTable as persistent panel below the active chart view
- [x] 14.4 Write property test: view switch preserves zoom domain (Property 38)
  - [x] 🧪 PBT: Property 38 — View switch preserves zoom domain
- [x] 14.5 Write property test: status-to-color mapping consistency (Property 25)
  - [x] 🧪 PBT: Property 25 — Status-to-color mapping consistency
- [x] 14.6 Write unit test: default view is 'line', all 5 views render without crashing with sample data

## Task 15: Integration and CLI
- [x] 15.1 Add CLI flags to the Go server for simulation parameters (duration, node count, output directory, seed) and wire `SimulationEngine` into `main.go` as an alternative to the existing `GenerateMockNodes()`
- [x] 15.2 Generate a sample dataset using the new simulation engine and place output in `src/heartbeat-visualizer/public/mocking_data/`
- [x] 15.3 End-to-end manual verification: start Go server with new mock data, open visualizer, switch between all 5 views, verify anomaly highlighting and sparklines update
