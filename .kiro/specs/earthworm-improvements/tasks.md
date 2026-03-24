# Implementation Plan: Earthworm Improvements

## Overview

This plan implements four improvement areas for the Earthworm Kubernetes heartbeat monitoring system: Go server refactoring with structured errors, storage interface, WebSocket streaming, and anomaly detection; React Visualizer migration to TypeScript with separation of concerns, zoom/pan, multi-cluster, and accessibility; lease parser round-trip verification; and comprehensive test coverage across all layers. Tasks are ordered so each builds on the previous, with no orphaned code.

## Tasks

- [x] 1. Go server refactoring: config, structured errors, storage interface, and logging middleware
  - [x] 1.1 Create `src/server/config.go` with `Config` struct and loader from environment variables
    - Define `Config` struct with fields: Port, LogFilePath, CORSOrigins, StoreType, RedisAddr, WarningThresholdS, CriticalThresholdS, WebhookURL
    - Implement `LoadConfig()` that reads from env vars with defaults as specified in the design
    - _Requirements: 2.1_

  - [x] 1.2 Create `src/server/errors.go` with `ErrorResponse` struct and `writeJSONError` helper
    - Define `ErrorResponse` struct with `Error` field
    - Implement `writeJSONError(w, msg, status)` that writes JSON error responses with correct Content-Type
    - _Requirements: 4.1, 4.2, 4.4_

  - [x] 1.3 Create `src/server/store.go` with `Store` interface and in-memory implementation
    - Define `Store` interface with `Save`, `GetByTimeRange`, `GetLatestByNode`, `Ping` methods
    - Define `Heartbeat` struct with NodeName, Namespace, Timestamp, Status, EbpfPID, EbpfComm fields
    - Implement `MemoryStore` with mutex-protected slice
    - _Requirements: 6.1, 6.2_

  - [x] 1.4 Create `src/server/middleware.go` with logging middleware
    - Implement `LoggingMiddleware` that wraps handlers and logs method, path, status code, and duration
    - Use a `responseWriter` wrapper to capture status code
    - _Requirements: 4.3_

  - [x] 1.5 Refactor `src/server/main.go` to use Config, Store, structured errors, and middleware
    - Load config via `LoadConfig()` on startup
    - Initialize `MemoryStore` (or Redis based on config) and verify connectivity via `Ping`
    - Update `heartbeatHandler` to use `writeJSONError` for 405 (invalid method) and 400 (malformed JSON) responses
    - Update `getHeartbeatsHandler` to use Store interface and return 503 on store errors
    - Wrap handlers with `LoggingMiddleware`
    - Set up CORS from config
    - _Requirements: 2.1, 4.1, 4.2, 4.3, 4.4, 6.4, 6.5_

  - [x]* 1.6 Write property test: Server config round-trip from environment (Property 1)
    - **Property 1: Server config round-trip from environment**
    - Use `pgregory.net/rapid` to generate random valid config values, set as env vars, call `LoadConfig()`, assert fields match
    - **Validates: Requirements 2.1**

  - [x]* 1.7 Write property test: Invalid HTTP method returns structured 405 (Property 2)
    - **Property 2: Invalid HTTP method returns structured 405**
    - Use `rapid` to generate HTTP methods other than POST, send to `/api/heartbeat`, assert 405 with JSON `error` field
    - **Validates: Requirements 4.1**

  - [x]* 1.8 Write property test: Malformed JSON returns structured 400 (Property 3)
    - **Property 3: Malformed JSON returns structured 400**
    - Use `rapid` to generate non-JSON strings, POST to `/api/heartbeat`, assert 400 with JSON `error` field
    - **Validates: Requirements 4.2**

  - [x]* 1.9 Write property test: Logging middleware captures request metadata (Property 4)
    - **Property 4: Logging middleware captures request metadata**
    - Use `rapid` to generate method/path combinations, send through middleware, assert log contains method, path, status, non-negative duration
    - **Validates: Requirements 4.3**

- [x] 2. Checkpoint — Ensure all Go server refactoring tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 3. Go server new features: WebSocket hub, anomaly detection, and alert dispatcher
  - [x] 3.1 Create `src/server/ws.go` with WebSocket Hub using `gorilla/websocket`
    - Implement `Hub` struct with clients map, broadcast/register/unregister channels
    - Implement `Hub.Run()`, `Hub.BroadcastHeartbeat(event)`, `Hub.BroadcastAlert(alert)`
    - Implement `/ws/heartbeats` upgrade handler
    - Define WebSocket message envelope: `{"type": "heartbeat"|"alert"|"status", "payload": {...}}`
    - _Requirements: 5.1, 5.2_

  - [x] 3.2 Create `src/server/anomaly.go` with `AnomalyDetector` and `Alert` types
    - Define `Alert` struct with NodeName, Namespace, Gap, Severity, Timestamp
    - Implement `AnomalyDetector` with configurable warning/critical thresholds
    - Implement `Evaluate(event)` that compares against latest stored event for the same node
    - Emit warning alert if gap > warningThreshold and ≤ criticalThreshold, critical if > criticalThreshold
    - _Requirements: 7.1, 7.2, 7.3, 7.5_

  - [x] 3.3 Create `src/server/alert.go` with `AlertDispatcher`
    - Implement `AlertDispatcher` with webhook URL and WS broadcast function
    - Implement `Dispatch(alert)` that sends to webhook (if configured) and broadcasts to WS clients
    - _Requirements: 7.4, 7.6_

  - [x] 3.4 Wire WebSocket, anomaly detection, and alerting into `src/server/main.go`
    - Start Hub goroutine on server startup
    - On POST heartbeat: save to store, broadcast to WS clients, evaluate with AnomalyDetector, dispatch alerts
    - Ensure broadcast happens within 500ms of POST
    - _Requirements: 5.2, 7.6, 13.2_

  - [x]* 3.5 Write property test: WebSocket broadcast of heartbeat events (Property 5)
    - **Property 5: WebSocket broadcast of heartbeat events**
    - Use `rapid` to generate valid heartbeats, POST them, verify connected WS client receives within 500ms
    - **Validates: Requirements 5.2, 13.2**

  - [x]* 3.6 Write property test: Anomaly detection emits correct severity alerts (Property 8)
    - **Property 8: Anomaly detection emits correct severity alerts**
    - Use `rapid` to generate pairs of timestamps with varying gaps, assert correct alert severity or no alert
    - **Validates: Requirements 7.2, 7.3, 7.5**

  - [x]* 3.7 Write property test: Alerts broadcast to WebSocket clients (Property 9)
    - **Property 9: Alerts broadcast to WebSocket clients**
    - Use `rapid` to generate alerts, dispatch them, verify WS clients receive message of type `"alert"`
    - **Validates: Requirements 7.6**

  - [x]* 3.8 Write property test: Heartbeat POST-then-GET round-trip (Property 15)
    - **Property 15: Heartbeat POST-then-GET round-trip**
    - Use `rapid` to generate valid heartbeats, POST then GET, assert response contains matching event
    - **Validates: Requirements 11.6, 13.1**

- [x] 4. Checkpoint — Ensure all Go server feature tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Go Kubernetes package tests: eBPF correlation and mock event generation
  - [x]* 5.1 Write property test: eBPF event correlation (Property 13)
    - **Property 13: eBPF event correlation**
    - Use `rapid` to generate PodInfo slices and cgroup paths, assert `CorrelateEBPFEvent` returns correct match or nil
    - **Validates: Requirements 11.3**

  - [x]* 5.2 Write property test: Mock eBPF event generation count and fields (Property 14)
    - **Property 14: Mock eBPF event generation count and fields**
    - Use `rapid` to generate positive `n` and node lists, assert `GenerateMockEBPFEvents` returns `n` events with non-zero PID, non-empty Comm, non-empty CgroupPath, non-zero Timestamp
    - **Validates: Requirements 11.5**

- [x] 6. Visualizer TypeScript migration and config extraction
  - [x] 6.1 Set up TypeScript in `src/heartbeat-visualizer/`
    - Add `typescript`, `@types/react`, `@types/react-dom` to devDependencies
    - Create `tsconfig.json` in `src/heartbeat-visualizer/` with strict mode, JSX react-jsx
    - _Requirements: 1.1, 1.4_

  - [x] 6.2 Create type definitions in `src/heartbeat-visualizer/src/types/heartbeat.ts`
    - Define `HeartbeatEvent`, `EbpfMetadata`, `LeasePoint`, `LeasesByNamespace`, `Alert`, `ChartControlsProps` interfaces
    - _Requirements: 1.2, 1.3_

  - [x] 6.3 Create config module at `src/heartbeat-visualizer/src/config.ts`
    - Export `config` object with heartbeatInterval, warningGapThreshold, criticalGapThreshold, colors, clusterName, wsEndpoint, apiBaseUrl, reconnect settings
    - _Requirements: 2.2, 2.3_

  - [x] 6.4 Create shared utilities at `src/heartbeat-visualizer/src/utils/chartUtils.ts`
    - Extract and type `getSegmentColor`, `hasWarning`, `hasDeath`, `getAnomalies`, `formatFullDate` from HeartbeatChart.jsx and ChartControls.jsx
    - Import thresholds and colors from config module
    - _Requirements: 3.4_

  - [x] 6.5 Create data service module at `src/heartbeat-visualizer/src/services/dataService.ts`
    - Implement `fetchHeartbeats(from?, to?)`, `postHeartbeat(event)`, `transformLeasesForChart(data, maxPoints)`
    - Import types from `types/heartbeat.ts` and config from `config.ts`
    - _Requirements: 3.1_

  - [x] 6.6 Create custom hooks in `src/heartbeat-visualizer/src/hooks/`
    - Implement `useHeartbeatData` hook for lease data loading, animation state, current heartbeat index
    - Implement `useEbpfData` hook for eBPF manifest loading and marker computation
    - Implement `useWebSocket` hook with exponential backoff reconnection (1s initial, 30s cap), connection status, incoming messages
    - _Requirements: 3.2, 5.3, 5.4, 5.5, 5.6_

  - [x] 6.7 Migrate components to TypeScript: `App.tsx`, `HeartbeatChart.tsx`, `ChartControls.tsx`, `Footer.tsx`
    - Convert `.js`/`.jsx` → `.tsx` with typed props and state
    - HeartbeatChart: use custom hooks and data service, contain only rendering logic
    - ChartControls: use typed props interface, import utilities from chartUtils
    - Add connection status indicator for WebSocket disconnection
    - Add toast notification component for alerts received via WebSocket
    - _Requirements: 1.1, 1.2, 3.3, 5.6, 7.7_

- [x] 7. Checkpoint — Ensure TypeScript compilation produces zero errors
  - Ensure all tests pass, ask the user if questions arise.

- [x] 8. Visualizer enhancements: zoom/pan, multi-cluster, accessibility
  - [x] 8.1 Add zoom/pan support to HeartbeatChart
    - Manage `xDomain` state for controlled chart domain
    - Implement click-and-drag `ReferenceArea` for brush-select zoom
    - Implement pan via drag that shifts `xDomain` by delta
    - Add "Reset Zoom" button that clears `xDomain` to full range
    - _Requirements: 8.1, 8.2, 8.3_

  - [x] 8.2 Add multi-cluster support
    - Create `ClusterSelector` component with tabs or dropdown for cluster switching
    - Fetch data independently per cluster
    - Render chart for selected cluster
    - _Requirements: 8.4, 8.5_

  - [x] 8.3 Implement accessibility and responsive layout
    - Use semantic HTML: `<header>`, `<main>`, `<footer>`, `<nav>`
    - Add ARIA labels to all interactive controls (sound toggle, language toggle, restart, eBPF button)
    - Add keyboard navigation with visible focus rings
    - Ensure color contrast ≥ 4.5:1 for all text
    - Implement responsive layout: single-column below 768px, wider above
    - Make chart width responsive to viewport (320px–1920px)
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 8.6_

- [x] 9. Visualizer tests: utility property tests and component rendering tests
  - [x]* 9.1 Write property test: Gap classification — hasWarning and hasDeath (Property 16)
    - **Property 16: Gap classification (hasWarning and hasDeath)**
    - Use `fast-check` to generate arrays of lease points, assert `hasWarning` returns true iff gap in (10000, 40000), `hasDeath` returns true iff gap > 40000
    - **Validates: Requirements 12.1, 12.2**

  - [x]* 9.2 Write property test: Anomaly detection utility — getAnomalies (Property 17)
    - **Property 17: Anomaly detection utility (getAnomalies)**
    - Use `fast-check` to generate `LeasesByNamespace` objects, assert one anomaly per gap in (10000, 40000) with correct namespace and gap
    - **Validates: Requirements 12.3**

  - [x]* 9.3 Write property test: Segment color mapping (Property 18)
    - **Property 18: Segment color mapping**
    - Use `fast-check` to generate pairs of timestamps, assert `getSegmentColor` returns correct color per gap range
    - **Validates: Requirements 12.4**

  - [x]* 9.4 Write property test: Incoming WebSocket event appends to chart data (Property 6)
    - **Property 6: Incoming WebSocket event appends to chart data**
    - Use `fast-check` to generate heartbeat events, simulate WS message via mock, assert chart data length increases by 1 and last element matches
    - **Validates: Requirements 5.4**

  - [x]* 9.5 Write property test: Exponential backoff on WebSocket reconnection (Property 7)
    - **Property 7: Exponential backoff on WebSocket reconnection**
    - Use `fast-check` to generate consecutive failure counts `n`, assert delay equals `min(2^n * 1000, 30000)`
    - **Validates: Requirements 5.5**

  - [x]* 9.6 Write rendering tests for HeartbeatChart, ChartControls, App, and Footer
    - HeartbeatChart: renders chart elements with mock data
    - ChartControls: renders all buttons, responds to click events
    - App: renders header, chart, footer
    - Verify semantic HTML elements (header, main, footer) present
    - Verify ARIA labels on interactive controls
    - _Requirements: 12.5, 12.6, 12.7, 9.1, 9.2_

- [x] 10. Checkpoint — Ensure all Visualizer tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 11. Lease parser: TypeScript migration and round-trip serializer
  - [x] 11.1 Migrate `parseLeases.js` to `parseLeases.ts` with typed interfaces
    - Add TypeScript types for parsed lease structures
    - Add input validation: exit with descriptive error for missing file, malformed YAML, missing required fields
    - _Requirements: 10.1, 10.2, 10.3_

  - [x] 11.2 Add `serializeToYaml` function to `parseLeases.ts`
    - Convert namespaced JSON structure back into a valid YAML Lease list
    - Enable round-trip: `parse(yaml) → json → serialize(json) → yaml2 → parse(yaml2)` produces equivalent JSON
    - _Requirements: 10.4, 10.5_

  - [x]* 11.3 Write property test: Lease parser round-trip (Property 10)
    - **Property 10: Lease parser round-trip**
    - Use `fast-check` to generate valid lease YAML inputs, parse → serialize → parse, assert equivalence
    - **Validates: Requirements 10.1, 10.4, 10.5**

  - [x]* 11.4 Write property test: Lease parser rejects invalid input (Property 11)
    - **Property 11: Lease parser rejects invalid input**
    - Use `fast-check` to generate invalid YAML strings, assert parser throws descriptive error
    - **Validates: Requirements 10.2**

  - [x]* 11.5 Write property test: Lease parser output filename format (Property 12)
    - **Property 12: Lease parser output filename format**
    - Use `fast-check` to generate execution timestamps, assert filename matches `leases{YYYYMMDDTHHmmss}.json` pattern
    - **Validates: Requirements 10.3**

- [x] 12. Redis store implementation (optional persistent storage)
  - [x] 12.1 Create `src/server/redis_store.go` implementing the `Store` interface
    - Use `go-redis/redis` client
    - Store heartbeats as sorted sets keyed by node, scored by timestamp
    - Store latest heartbeat per node for fast anomaly lookups
    - Configurable TTL (default 7 days)
    - Select via `EARTHWORM_STORE=redis` env var
    - _Requirements: 6.3_

  - [x] 12.2 Wire Redis store selection into `src/server/main.go`
    - Based on `Config.StoreType`, initialize either `MemoryStore` or `RedisStore`
    - Verify connectivity via `Ping` before accepting requests
    - _Requirements: 6.4, 6.5_

- [x] 13. Final checkpoint — Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document (minimum 100 iterations each)
- Unit/rendering tests validate specific examples and edge cases
- Go tests use `pgregory.net/rapid` for property-based testing
- Visualizer tests use `fast-check` for property-based testing and Jest + React Testing Library for rendering tests
