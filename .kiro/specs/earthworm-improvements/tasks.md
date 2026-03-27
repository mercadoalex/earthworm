# Implementation Plan: Earthworm Improvements

## Overview

Incremental implementation of refactoring, new features, visualization enhancements, and test coverage for the Earthworm Kubernetes heartbeat monitoring system. The Go server already has many components in place (store interface, WebSocket hub, anomaly detector, config loader, structured errors). The React visualizer has been partially migrated to TypeScript with hooks and services. Tasks focus on completing gaps, wiring remaining pieces, and adding comprehensive test coverage.

## Tasks

- [x] 1. Complete TypeScript migration and config extraction in the Visualizer
  - [x] 1.1 Audit and complete TypeScript type definitions
    - Ensure `src/heartbeat-visualizer/src/types/heartbeat.ts` includes all interfaces from the design: `HeartbeatEvent`, `EbpfMetadata`, `LeasePoint`, `LeasesByNamespace`, `Alert`, `ChartControlsProps`
    - Verify all component files (App.tsx, HeartbeatChart.tsx, ChartControls.tsx, Footer.tsx, ClusterSelector.tsx) have typed props and no `any` casts
    - _Requirements: 1.1, 1.2, 1.3_

  - [x] 1.2 Complete config module extraction
    - Verify `src/heartbeat-visualizer/src/config.ts` contains all thresholds, colors, intervals, endpoints, and reconnect settings from the design
    - Remove any remaining hardcoded color strings, interval values, or threshold constants from component files (HeartbeatChart.tsx, ChartControls.tsx, App.tsx)
    - Import from `config.ts` everywhere these values are used
    - _Requirements: 2.2, 2.3_

  - [x] 1.3 Verify separation of concerns in data layer
    - Ensure `src/heartbeat-visualizer/src/services/dataService.ts` exports `fetchHeartbeats`, `postHeartbeat`, and `transformLeasesForChart`
    - Ensure custom hooks (`useHeartbeatData`, `useEbpfData`, `useWebSocket`) handle all state management and data fetching
    - Ensure `HeartbeatChart.tsx` contains only rendering logic
    - Move `getSegmentColor`, `hasWarning`, `hasDeath`, `getAnomalies`, `formatFullDate` into `src/heartbeat-visualizer/src/utils/chartUtils.ts` if not already there
    - _Requirements: 3.1, 3.2, 3.3, 3.4_

  - [ ]* 1.4 Write property tests for chart utilities
    - **Property 16: Gap classification (hasWarning and hasDeath)**
    - **Validates: Requirements 12.1, 12.2**
    - **Property 17: Anomaly detection utility (getAnomalies)**
    - **Validates: Requirements 12.3**
    - **Property 18: Segment color mapping**
    - **Validates: Requirements 12.4**

- [x] 2. Complete Go server config, error handling, and middleware
  - [x] 2.1 Verify and complete server config loader
    - Ensure `src/server/config.go` reads all fields from the design (`Port`, `LogFilePath`, `CORSOrigins`, `StoreType`, `RedisAddr`, `WarningThresholdS`, `CriticalThresholdS`, `WebhookURL`) from environment variables with defaults
    - _Requirements: 2.1_

  - [ ]* 2.2 Write property test for config round-trip
    - **Property 1: Server config round-trip from environment**
    - **Validates: Requirements 2.1**

  - [x] 2.3 Verify structured error responses
    - Ensure `src/server/errors.go` defines `ErrorResponse` struct and `writeJSONError` helper
    - Verify all handler error paths in `main.go` use `writeJSONError` (405, 400, 500, 503 cases)
    - _Requirements: 4.1, 4.2, 4.4_

  - [ ]* 2.4 Write property tests for error handling
    - **Property 2: Invalid HTTP method returns structured 405**
    - **Validates: Requirements 4.1**
    - **Property 3: Malformed JSON returns structured 400**
    - **Validates: Requirements 4.2**

  - [x] 2.5 Verify logging middleware
    - Ensure `src/server/middleware.go` implements `LoggingMiddleware` that logs method, path, status code, and duration for every request
    - Verify it is wired into the HTTP handler chain in `main.go`
    - _Requirements: 4.3_

  - [ ]* 2.6 Write property test for logging middleware
    - **Property 4: Logging middleware captures request metadata**
    - **Validates: Requirements 4.3**

- [x] 3. Checkpoint — Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. WebSocket real-time streaming
  - [x] 4.1 Verify WebSocket hub and endpoint
    - Ensure `src/server/ws.go` implements `Hub` with `Run`, `BroadcastHeartbeat`, `BroadcastAlert` methods
    - Ensure `/ws/heartbeats` endpoint is registered in `main.go` and calls `ServeWS`
    - Verify heartbeat POST handler broadcasts to hub after saving
    - _Requirements: 5.1, 5.2_

  - [ ]* 4.2 Write property test for WebSocket broadcast
    - **Property 5: WebSocket broadcast of heartbeat events**
    - **Validates: Requirements 5.2, 13.2**

  - [x] 4.3 Complete Visualizer WebSocket integration
    - Ensure `useWebSocket` hook in `src/heartbeat-visualizer/src/hooks/useWebSocket.ts` connects on mount, parses message envelope (`{type, payload}`), and exposes connection status
    - Implement exponential backoff reconnection: delay = `min(2^n * 1000, 30000)` ms
    - Display "Disconnected" indicator when connection is lost
    - Append incoming heartbeat events to chart data without full reload
    - _Requirements: 5.3, 5.4, 5.5, 5.6_

  - [ ]* 4.4 Write property tests for WebSocket client behavior
    - **Property 6: Incoming WebSocket event appends to chart data**
    - **Validates: Requirements 5.4**
    - **Property 7: Exponential backoff on WebSocket reconnection**
    - **Validates: Requirements 5.5**

- [x] 5. Pluggable persistent storage
  - [x] 5.1 Verify Store interface and implementations
    - Ensure `src/server/store.go` defines `Store` interface with `Save`, `GetByTimeRange`, `GetLatestByNode`, `Ping`
    - Ensure `NewMemoryStore()` in `store.go` implements the interface with mutex-protected slice
    - Ensure `src/server/redis_store.go` implements the interface using sorted sets
    - Verify `main.go` selects store based on `EARTHWORM_STORE` env var and calls `Ping` on startup
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5_

- [x] 6. Anomaly detection and alerting
  - [x] 6.1 Verify anomaly detector and alert dispatcher
    - Ensure `src/server/anomaly.go` implements `AnomalyDetector` with `Evaluate` method that compares incoming event against latest stored event per node
    - Ensure `src/server/alert.go` implements `AlertDispatcher` with `Dispatch` method that sends to webhook URL and broadcasts to WebSocket clients
    - Verify `Alert` struct includes `NodeName`, `Namespace`, `Gap`, `Severity`, `Timestamp`
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6_

  - [ ]* 6.2 Write property tests for anomaly detection
    - **Property 8: Anomaly detection emits correct severity alerts**
    - **Validates: Requirements 7.2, 7.3, 7.5**
    - **Property 9: Alerts broadcast to WebSocket clients**
    - **Validates: Requirements 7.6**

  - [x] 6.3 Implement alert toast notifications in Visualizer
    - Add a toast notification component that displays when an alert message is received via WebSocket
    - Show severity color and node name, auto-dismiss after 10 seconds
    - _Requirements: 7.7_

- [x] 7. Checkpoint — Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 8. Visualization enhancements — zoom, pan, multi-cluster
  - [x] 8.1 Implement zoom/pan on HeartbeatChart
    - Add `xDomain` state to `HeartbeatChart.tsx`
    - Implement click-and-drag `ReferenceArea` for brush-select zoom that updates `xDomain` on mouse-up
    - Implement pan by shifting `xDomain` on drag
    - Add "Reset Zoom" button that clears `xDomain` to show full range
    - _Requirements: 8.1, 8.2, 8.3_

  - [x] 8.2 Complete multi-cluster support
    - Ensure `ClusterSelector.tsx` renders tabs or dropdown for switching between clusters
    - Fetch each cluster's data independently via `dataService`
    - Render chart data for the selected cluster
    - _Requirements: 8.4, 8.5_

  - [x] 8.3 Responsive layout
    - Ensure chart width adapts to viewport between 320px and 1920px
    - Single-column layout below 768px, wider layout above
    - _Requirements: 8.6, 9.5_

- [x] 9. Accessibility improvements
  - [x] 9.1 Implement semantic HTML and ARIA
    - Use `<header>`, `<main>`, `<footer>`, `<nav>` for page structure in `App.tsx`
    - Add ARIA labels to all interactive controls (sound toggle, language toggle, restart, eBPF correlation, zoom reset)
    - Ensure keyboard navigation with visible focus rings on all interactive elements
    - Verify color contrast ≥ 4.5:1 for all text
    - _Requirements: 9.1, 9.2, 9.3, 9.4_

- [x] 10. Lease parser round-trip serializer
  - [x] 10.1 Implement YAML serializer in parseLeases.ts
    - Add `serializeToYaml(data: LeasesByNamespace, originalYaml: any): string` function to `src/heartbeat-visualizer/src/parseLeases.ts`
    - Converts namespaced JSON structure back into a valid YAML Lease list
    - Verify output filename matches `leases{YYYYMMDDTHHmmss}.json` pattern
    - _Requirements: 10.1, 10.3, 10.4_

  - [ ]* 10.2 Write property tests for lease parser
    - **Property 10: Lease parser round-trip**
    - **Validates: Requirements 10.1, 10.4, 10.5**
    - **Property 11: Lease parser rejects invalid input**
    - **Validates: Requirements 10.2**
    - **Property 12: Lease parser output filename format**
    - **Validates: Requirements 10.3**

- [x] 11. Checkpoint — Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 12. Go server unit and integration tests
  - [x] 12.1 Write unit tests for heartbeat handlers
    - POST valid heartbeat → 201, verify stored
    - POST malformed JSON → 400 with JSON error body
    - POST wrong method (GET) → 405 with JSON error body
    - GET empty store → 200 with `[]`
    - GET populated store → 200 with correct data
    - Internal error → 500 with generic message
    - Store unreachable → 503
    - Add to `src/server/handler_unit_test.go`
    - _Requirements: 11.1, 11.2_

  - [x] 12.2 Write unit tests for eBPF correlation and mock generation
    - Test `CorrelateEBPFEvent` with matching and non-matching cgroup paths
    - Test `GenerateMockNodes` returns 50 nodes with valid fields
    - Test `GenerateMockEBPFEvents` returns correct count with populated fields
    - Add to `src/kubernetes/` test files
    - _Requirements: 11.3, 11.4, 11.5_

  - [ ]* 12.3 Write property tests for eBPF functions
    - **Property 13: eBPF event correlation**
    - **Validates: Requirements 11.3**
    - **Property 14: Mock eBPF event generation count and fields**
    - **Validates: Requirements 11.5**

  - [x] 12.4 Write integration test for POST-then-GET round-trip
    - POST a heartbeat, GET heartbeats, verify the posted event is in the response
    - Use `httptest.Server` for isolated testing
    - _Requirements: 11.6_

  - [ ]* 12.5 Write property test for POST-then-GET round-trip
    - **Property 15: Heartbeat POST-then-GET round-trip**
    - **Validates: Requirements 11.6, 13.1**

- [x] 13. Visualizer component and rendering tests
  - [x] 13.1 Write rendering tests for React components
    - HeartbeatChart renders chart elements with mock data
    - ChartControls renders all buttons and responds to clicks
    - App component renders header, chart, and footer
    - Verify semantic HTML elements present (header, main, footer)
    - Verify ARIA labels on interactive controls
    - Test connection status indicator shows "Disconnected" when WS is down
    - Test alert toast renders with severity and node name
    - Add to `src/heartbeat-visualizer/src/components.test.tsx`
    - _Requirements: 12.5, 12.6, 12.7, 9.1, 9.2_

  - [ ]* 13.2 Write unit tests for chart utility functions
    - Test `hasWarning` with gaps below, within, and above warning range
    - Test `hasDeath` with gaps below and above critical threshold
    - Test `getAnomalies` across multiple namespaces
    - Test `getSegmentColor` for green, yellow, and red intervals
    - Add to `src/heartbeat-visualizer/src/chartUtils.unit.test.ts`
    - _Requirements: 12.1, 12.2, 12.3, 12.4_

- [x] 14. End-to-end data flow tests
  - [x] 14.1 Write Go E2E test for POST → GET → WS flow
    - POST heartbeat → GET returns it (validates Property 15)
    - POST heartbeat → WS client receives it (validates Property 5)
    - Use `httptest.Server` + `gorilla/websocket` client
    - _Requirements: 13.1, 13.2_

  - [ ]* 14.2 Write Visualizer E2E test for WS data rendering
    - Mock WebSocket stream, verify Visualizer renders the received data point
    - Add to `src/heartbeat-visualizer/src/wsE2E.test.tsx`
    - _Requirements: 13.3_

- [x] 15. Final checkpoint — Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- Go tests use `testing` + `net/http/httptest` + `pgregory.net/rapid` for property tests
- Visualizer tests use Jest + React Testing Library + `fast-check` for property tests
