# Implementation Plan: Earthworm Improvements

## Overview

This plan covers four areas: refactoring (TypeScript migration, config extraction, separation of concerns, structured errors), new features (WebSocket streaming, pluggable storage, anomaly detection/alerting), visualization enhancements (zoom/pan, multi-cluster, accessibility), and comprehensive testing. Many components already exist — tasks focus on completing gaps, wiring remaining pieces, and adding test coverage.

## Tasks

- [x] 1. Complete TypeScript migration and remove legacy JS files
  - [x] 1.1 Remove legacy `.js`/`.jsx` duplicates and ensure all imports reference `.tsx`/`.ts` files
    - Delete `src/heartbeat-visualizer/src/HeartbeatChart.jsx`, `ChartControls.jsx`, `Footer.js`, `heartbeat.js`, `checkDates.js`, `App.test.js`
    - Update `src/heartbeat-visualizer/src/index.js` to import from `.tsx` modules (or convert to `index.tsx`)
    - Verify `src/heartbeat-visualizer/src/utils/getClusterName.js` is converted to TypeScript
    - _Requirements: 1.1, 1.2, 1.3_

  - [x] 1.2 Verify TypeScript strict mode compilation produces zero errors
    - Ensure `src/heartbeat-visualizer/tsconfig.json` has `"strict": true`
    - Run `tsc --noEmit` and fix any remaining type errors
    - _Requirements: 1.4_

  - [ ]* 1.3 Write property test for TypeScript compilation (zero errors)
    - **Property 6: Incoming WebSocket event appends to chart data**
    - **Validates: Requirements 5.4**

- [x] 2. Verify and complete Go server config, errors, and middleware
  - [x] 2.1 Verify config loader reads all parameters from environment variables
    - Confirm `src/server/config.go` `LoadConfig()` covers port, log file, CORS origins, store type, Redis addr, warning/critical thresholds, webhook URL
    - _Requirements: 2.1_

  - [ ]* 2.2 Write property test for config round-trip from environment
    - **Property 1: Server config round-trip from environment**
    - **Validates: Requirements 2.1**

  - [x] 2.3 Verify structured error responses for all error paths
    - Confirm `writeJSONError` is used for 400, 405, 500, 503 in `heartbeatHandler`, `getHeartbeatsHandler`, and `ebpfEventsHandler`
    - _Requirements: 4.1, 4.2, 4.4_

  - [ ]* 2.4 Write property test for invalid HTTP method returns 405
    - **Property 2: Invalid HTTP method returns structured 405**
    - **Validates: Requirements 4.1**

  - [ ]* 2.5 Write property test for malformed JSON returns 400
    - **Property 3: Malformed JSON returns structured 400**
    - **Validates: Requirements 4.2**

  - [x] 2.6 Verify logging middleware captures method, path, status, duration
    - Confirm `src/server/middleware.go` `LoggingMiddleware` logs all four fields
    - _Requirements: 4.3_

  - [ ]* 2.7 Write property test for logging middleware metadata
    - **Property 4: Logging middleware captures request metadata**
    - **Validates: Requirements 4.3**

- [x] 3. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. Verify and complete WebSocket real-time streaming
  - [x] 4.1 Verify WebSocket hub broadcasts heartbeat events to all clients
    - Confirm `src/server/ws.go` `Hub.BroadcastHeartbeat` sends to all registered clients
    - Confirm `heartbeatHandler` calls `hub.BroadcastHeartbeat` after saving
    - _Requirements: 5.1, 5.2_

  - [ ]* 4.2 Write property test for WebSocket broadcast of heartbeat events
    - **Property 5: WebSocket broadcast of heartbeat events**
    - **Validates: Requirements 5.2, 13.2**

  - [x] 4.3 Verify Visualizer WebSocket hook connects on mount with exponential backoff
    - Confirm `src/heartbeat-visualizer/src/hooks/useWebSocket.ts` connects on mount, reconnects with backoff, and exposes connection status
    - _Requirements: 5.3, 5.5, 5.6_

  - [ ]* 4.4 Write property test for exponential backoff on WebSocket reconnection
    - **Property 7: Exponential backoff on WebSocket reconnection**
    - **Validates: Requirements 5.5**

  - [x] 4.5 Verify Visualizer appends incoming WebSocket heartbeat events to chart data
    - Confirm `HeartbeatChart.tsx` handles `lastMessage` of type `heartbeat` and appends to live events
    - _Requirements: 5.4_

  - [ ]* 4.6 Write property test for incoming WebSocket event appends to chart data
    - **Property 6: Incoming WebSocket event appends to chart data**
    - **Validates: Requirements 5.4**

- [x] 5. Verify and complete pluggable persistent storage
  - [x] 5.1 Verify Store interface and MemoryStore implementation
    - Confirm `src/server/store.go` `Store` interface has `Save`, `GetByTimeRange`, `GetLatestByNode`, `Ping`
    - Confirm `MemoryStore` implements all methods correctly
    - _Requirements: 6.1, 6.2_

  - [x] 5.2 Verify Redis store implementation and config-based selection
    - Confirm `src/server/redis_store.go` implements `Store` interface
    - Confirm `main.go` selects store based on `cfg.StoreType`
    - _Requirements: 6.3_

  - [x] 5.3 Verify store connectivity check on startup and 503 on unreachable store
    - Confirm `main.go` calls `store.Ping()` before accepting requests
    - Confirm `getHeartbeatsHandler` returns 503 when store errors
    - _Requirements: 6.4, 6.5_

- [x] 6. Verify and complete anomaly detection and alerting
  - [x] 6.1 Verify anomaly detector evaluates gaps against configurable thresholds
    - Confirm `src/server/anomaly.go` `AnomalyDetector.Evaluate` compares gap to warning and critical thresholds
    - Confirm alert includes nodeName, namespace, gap, severity
    - _Requirements: 7.1, 7.2, 7.3, 7.5_

  - [ ]* 6.2 Write property test for anomaly detection severity
    - **Property 8: Anomaly detection emits correct severity alerts**
    - **Validates: Requirements 7.2, 7.3, 7.5**

  - [x] 6.3 Verify alert dispatcher sends to webhook and broadcasts to WebSocket
    - Confirm `src/server/alert.go` `AlertDispatcher.Dispatch` calls webhook and `wsBroadcast`
    - _Requirements: 7.4, 7.6_

  - [ ]* 6.4 Write property test for alerts broadcast to WebSocket clients
    - **Property 9: Alerts broadcast to WebSocket clients**
    - **Validates: Requirements 7.6**

  - [x] 6.5 Verify Visualizer displays toast notification on alert receipt
    - Confirm `HeartbeatChart.tsx` `Toast` component renders with severity and node name, auto-dismisses after 10s
    - _Requirements: 7.7_

- [x] 7. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 8. Verify and complete visualization enhancements
  - [x] 8.1 Verify zoom via click-and-drag brush selection on chart X axis
    - Confirm `HeartbeatChart.tsx` uses `ReferenceArea` for brush selection and updates `xDomain` on mouse-up
    - _Requirements: 8.1_

  - [x] 8.2 Verify pan via Shift+drag interaction on the chart
    - Confirm `HeartbeatChart.tsx` pan handlers shift `xDomain` by pixel-to-time delta
    - _Requirements: 8.2_

  - [x] 8.3 Verify "Reset Zoom" button restores full time range
    - Confirm `HeartbeatChart.tsx` renders "Reset Zoom" button when `xDomain` is set, and clicking it clears `xDomain`
    - _Requirements: 8.3_

  - [x] 8.4 Verify multi-cluster support via ClusterSelector
    - Confirm `App.tsx` renders `ClusterSelector` with tabs/dropdown
    - Confirm each cluster's data is fetched independently via `useHeartbeatData(cluster.manifestUrl, cluster.datasetPath)`
    - _Requirements: 8.4, 8.5_

  - [x] 8.5 Verify responsive chart width adapts to viewport
    - Confirm `HeartbeatChart.tsx` uses `ResizeObserver` to set `chartWidth` between 280px and 1920px
    - _Requirements: 8.6_

- [x] 9. Verify and complete accessibility and responsive UI
  - [x] 9.1 Verify semantic HTML elements in App, HeartbeatChart, Footer
    - Confirm `App.tsx` uses `<header>`, `<main>`, `<footer>` with appropriate roles
    - _Requirements: 9.1_

  - [x] 9.2 Verify ARIA labels on all interactive controls
    - Confirm `ChartControls.tsx` buttons have `aria-label` attributes (sound toggle, language toggle, restart, eBPF)
    - Confirm `ClusterSelector.tsx` uses `role="tablist"` and `role="tab"` with `aria-selected`
    - _Requirements: 9.2_

  - [x] 9.3 Verify keyboard navigation with visible focus indicators
    - Confirm `ClusterSelector.tsx` handles ArrowLeft/ArrowRight keyboard events
    - Add visible focus ring styles (`:focus-visible`) to interactive elements if missing
    - _Requirements: 9.3_

  - [x] 9.4 Verify color contrast ratio ≥ 4.5:1 for all text
    - Audit text colors against backgrounds: `#e0e0e0` on `#222` (Footer), `#ccc` on `#222` (controls), `#90caf9` on `#222` (links)
    - Adjust any colors that fail the 4.5:1 ratio
    - _Requirements: 9.4_

  - [x] 9.5 Verify responsive layout adapts below 768px
    - Confirm `App.css` or inline styles provide single-column layout on narrow viewports
    - Add media query or flex-wrap rules if missing
    - _Requirements: 9.5_

- [x] 10. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 11. Verify and complete Visualizer utility tests
  - [x] 11.1 Verify unit tests for `hasWarning` and `hasDeath` utility functions
    - Confirm `src/heartbeat-visualizer/src/chartUtils.property.test.ts` or equivalent covers gaps below, within, and above thresholds
    - _Requirements: 12.1, 12.2_

  - [ ]* 11.2 Write property test for gap classification (hasWarning and hasDeath)
    - **Property 16: Gap classification (hasWarning and hasDeath)**
    - **Validates: Requirements 12.1, 12.2**

  - [ ]* 11.3 Write property test for anomaly detection utility (getAnomalies)
    - **Property 17: Anomaly detection utility (getAnomalies)**
    - **Validates: Requirements 12.3**

  - [ ]* 11.4 Write property test for segment color mapping
    - **Property 18: Segment color mapping**
    - **Validates: Requirements 12.4**

  - [x] 11.5 Verify rendering tests for HeartbeatChart, ChartControls, App, and Footer
    - Confirm `src/heartbeat-visualizer/src/components.test.tsx` covers all four components
    - _Requirements: 12.5, 12.6, 12.7_

- [x] 12. Verify and complete lease parser tests
  - [x] 12.1 Verify unit tests for parseLeases (valid input, invalid YAML, missing file)
    - Confirm `src/heartbeat-visualizer/src/parseLeases.test.ts` covers valid parsing, malformed YAML, missing fields
    - _Requirements: 10.1, 10.2_

  - [ ]* 12.2 Write property test for lease parser round-trip
    - **Property 10: Lease parser round-trip**
    - **Validates: Requirements 10.1, 10.4, 10.5**

  - [ ]* 12.3 Write property test for lease parser rejects invalid input
    - **Property 11: Lease parser rejects invalid input**
    - **Validates: Requirements 10.2**

  - [ ]* 12.4 Write property test for lease parser output filename format
    - **Property 12: Lease parser output filename format**
    - **Validates: Requirements 10.3**

- [x] 13. Verify and complete Go server and K8s monitor tests
  - [x] 13.1 Verify unit tests for POST/GET heartbeat handlers
    - Add or confirm unit tests in `src/server/property_test.go` for: valid POST → 201, malformed POST → 400, wrong method → 405, GET empty → 200 with `[]`, GET populated → 200 with data
    - _Requirements: 11.1, 11.2_

  - [x] 13.2 Verify unit tests for CorrelateEBPFEvent (matching and non-matching cgroup paths)
    - Add or confirm tests in `src/agent/` or `src/kubernetes/` for `CorrelateEBPFEvent`
    - _Requirements: 11.3_

  - [ ]* 13.3 Write property test for eBPF event correlation
    - **Property 13: eBPF event correlation**
    - **Validates: Requirements 11.3**

  - [x] 13.4 Verify unit tests for GenerateMockNodes (50 nodes, valid fields)
    - Add or confirm tests for `GenerateMockNodes` returning 50 nodes with non-empty Name and Status
    - _Requirements: 11.4_

  - [x] 13.5 Verify unit tests for GenerateMockEBPFEvents (event count and field population)
    - Add or confirm tests for `GenerateMockEBPFEvents` returning correct count with non-zero PID, non-empty Comm, CgroupPath, Timestamp
    - _Requirements: 11.5_

  - [ ]* 13.6 Write property test for mock eBPF event generation count and fields
    - **Property 14: Mock eBPF event generation count and fields**
    - **Validates: Requirements 11.5**

  - [ ]* 13.7 Write property test for heartbeat POST-then-GET round-trip
    - **Property 15: Heartbeat POST-then-GET round-trip**
    - **Validates: Requirements 11.6, 13.1**

- [x] 14. Verify end-to-end data flow tests
  - [x] 14.1 Verify E2E test: POST heartbeat → GET returns it
    - Covered by Property 15 test; confirm it runs in CI
    - _Requirements: 13.1_

  - [x] 14.2 Verify E2E test: POST heartbeat → WS client receives it
    - Covered by Property 5 test; confirm it runs in CI
    - _Requirements: 13.2_

  - [x] 14.3 Verify E2E test: Visualizer renders data point from mocked WS stream
    - Add or confirm a Jest test that mocks WebSocket, sends a heartbeat message, and verifies the chart data updates
    - _Requirements: 13.3_

- [x] 15. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Many components already exist in the codebase — tasks focus on verification, gap-filling, and wiring
- Go server tests use `pgregory.net/rapid` for property-based testing
- Visualizer tests use `fast-check` for property-based testing and Jest + React Testing Library for unit/rendering tests
- Each property test references a specific correctness property from the design document
- Checkpoints ensure incremental validation across the four improvement areas
