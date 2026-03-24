# Requirements Document

## Introduction

The Earthworm project is a Kubernetes cluster heartbeat monitoring system that uses eBPF technology to intercept heartbeat signals and visualizes them through a React-based cardiogram-style UI. This requirements document covers improvements across four areas: codebase refactoring, new features (real-time data streaming, alerting), visualization UI enhancements, and proper test coverage. The current system uses in-memory storage, mock data, and has no automated tests.

## Glossary

- **Heartbeat_Server**: The Go HTTP server (`src/server/main.go`) that receives, stores, and serves heartbeat event data to clients.
- **Visualizer**: The React application (`src/heartbeat-visualizer/`) that renders heartbeat data as a cardiogram-style chart using Recharts.
- **eBPF_Collector**: The eBPF program (`src/ebpf/heartbeat.c`) that intercepts process context switches on Kubernetes nodes and emits heartbeat telemetry.
- **K8s_Monitor**: The Go Kubernetes client (`src/kubernetes/`) that watches Lease objects and Pod events for heartbeat data and correlates eBPF events with Kubernetes resources.
- **Lease_Parser**: The Node.js script (`src/heartbeat-visualizer/src/parseLeases.js`) that converts `leases.yaml` into timestamped JSON files consumed by the Visualizer.
- **Heartbeat_Event**: A single data point representing a node heartbeat, containing node name, timestamp, status, and optionally eBPF metadata (PID, comm, cgroup path).
- **Anomaly**: A gap between consecutive heartbeats exceeding the expected interval; a warning anomaly is a gap between 10 and 40 seconds, and a critical anomaly (death) is a gap exceeding 40 seconds.
- **Data_Store**: The persistence layer for heartbeat events; currently in-memory, planned to support Redis, MongoDB, or Kafka.
- **WebSocket_Stream**: A persistent bidirectional connection between the Heartbeat_Server and the Visualizer for pushing real-time heartbeat updates.
- **Alert**: A notification triggered when an Anomaly is detected, delivered via webhook, log entry, or UI notification.

## Requirements

### Requirement 1: Migrate to TypeScript in the Visualizer

**User Story:** As a developer, I want the Visualizer codebase to use TypeScript, so that type safety reduces runtime errors and improves maintainability.

#### Acceptance Criteria

1. THE Visualizer SHALL use TypeScript (`.tsx`/`.ts`) for all component and utility source files.
2. THE Visualizer SHALL define typed interfaces for all props passed between components (HeartbeatChart, ChartControls, Footer).
3. THE Visualizer SHALL define typed interfaces for all data structures including Heartbeat_Event, lease data, and eBPF event payloads.
4. WHEN a TypeScript compilation is run, THE Visualizer SHALL produce zero type errors.

### Requirement 2: Extract Shared Configuration Constants

**User Story:** As a developer, I want all magic numbers and hardcoded values centralized in configuration files, so that tuning parameters does not require searching through source code.

#### Acceptance Criteria

1. THE Heartbeat_Server SHALL read configurable parameters (port, log file path, CORS origins) from environment variables or a configuration file.
2. THE Visualizer SHALL read configurable parameters (heartbeat interval, color thresholds, warning gap threshold of 10 seconds, critical gap threshold of 40 seconds, cluster name) from a single configuration module.
3. THE Visualizer SHALL remove all hardcoded color strings, interval values, and threshold constants from component files.

### Requirement 3: Separate Data Fetching from Presentation in the Visualizer

**User Story:** As a developer, I want data fetching logic separated from rendering logic, so that components are easier to test and maintain.

#### Acceptance Criteria

1. THE Visualizer SHALL implement a dedicated data service module that handles all HTTP fetch calls and data transformation.
2. THE Visualizer SHALL implement custom React hooks for heartbeat data loading, eBPF data loading, and animation state management.
3. THE HeartbeatChart component SHALL contain only rendering logic and delegate data operations to the data service module and custom hooks.
4. THE Visualizer SHALL move the `getSegmentColor`, `hasWarning`, `hasDeath`, and `getAnomalies` functions into a shared utility module.

### Requirement 4: Introduce Structured Error Handling in the Heartbeat_Server

**User Story:** As a developer, I want the Heartbeat_Server to return structured JSON error responses, so that clients can programmatically handle failures.

#### Acceptance Criteria

1. IF the Heartbeat_Server receives a request with an invalid HTTP method, THEN THE Heartbeat_Server SHALL return a JSON response with an `error` field and HTTP status 405.
2. IF the Heartbeat_Server receives a POST request with malformed JSON, THEN THE Heartbeat_Server SHALL return a JSON response with an `error` field describing the parse failure and HTTP status 400.
3. THE Heartbeat_Server SHALL implement a middleware that logs each request with method, path, status code, and duration.
4. IF an internal error occurs during request processing, THEN THE Heartbeat_Server SHALL return a JSON response with a generic error message and HTTP status 500 without exposing internal details.

### Requirement 5: Real-Time Data Streaming via WebSocket

**User Story:** As a user, I want to see heartbeat data update in real time without refreshing the page, so that I can monitor cluster health continuously.

#### Acceptance Criteria

1. THE Heartbeat_Server SHALL expose a WebSocket endpoint at `/ws/heartbeats`.
2. WHEN a new Heartbeat_Event is received via POST, THE Heartbeat_Server SHALL broadcast the event to all connected WebSocket clients within 500 milliseconds.
3. THE Visualizer SHALL establish a WebSocket_Stream connection to the Heartbeat_Server on mount.
4. WHEN the Visualizer receives a Heartbeat_Event via WebSocket_Stream, THE Visualizer SHALL append the data point to the chart without reloading the full dataset.
5. IF the WebSocket_Stream connection is lost, THEN THE Visualizer SHALL attempt reconnection with exponential backoff starting at 1 second and capping at 30 seconds.
6. WHILE the WebSocket_Stream connection is disconnected, THE Visualizer SHALL display a visible connection status indicator showing "Disconnected".

### Requirement 6: Pluggable Persistent Data_Store

**User Story:** As an operator, I want heartbeat data persisted beyond server restarts, so that historical data is available for analysis.

#### Acceptance Criteria

1. THE Heartbeat_Server SHALL define a storage interface with methods for saving a Heartbeat_Event and retrieving Heartbeat_Events by time range.
2. THE Heartbeat_Server SHALL provide an in-memory implementation of the storage interface as the default.
3. THE Heartbeat_Server SHALL support a Redis implementation of the storage interface selectable via configuration.
4. WHEN the Heartbeat_Server starts, THE Heartbeat_Server SHALL initialize the configured Data_Store and verify connectivity before accepting requests.
5. IF the Data_Store becomes unreachable during operation, THEN THE Heartbeat_Server SHALL log the error and return HTTP status 503 for data retrieval requests.

### Requirement 7: Anomaly Detection and Alerting

**User Story:** As an operator, I want to be alerted when a node misses heartbeats, so that I can respond to potential node failures promptly.

#### Acceptance Criteria

1. THE Heartbeat_Server SHALL evaluate incoming Heartbeat_Events against configurable warning and critical gap thresholds.
2. WHEN the gap between consecutive heartbeats for a node exceeds the warning threshold, THE Heartbeat_Server SHALL emit a warning Alert.
3. WHEN the gap between consecutive heartbeats for a node exceeds the critical threshold, THE Heartbeat_Server SHALL emit a critical Alert.
4. THE Heartbeat_Server SHALL support delivering Alerts via a configurable webhook URL.
5. THE Heartbeat_Server SHALL include the node name, namespace, gap duration, and severity level in each Alert payload.
6. WHEN an Alert is emitted, THE Heartbeat_Server SHALL broadcast the Alert to connected WebSocket clients.
7. WHEN the Visualizer receives an Alert via WebSocket_Stream, THE Visualizer SHALL display a toast notification with the Alert severity and node name.

### Requirement 8: Enhanced Cardiogram Visualization

**User Story:** As a user, I want a richer cardiogram visualization with zoom, pan, and multi-cluster support, so that I can analyze heartbeat patterns across different time ranges and clusters.

#### Acceptance Criteria

1. THE Visualizer SHALL support zooming into a time range by click-and-drag selection on the chart X axis.
2. THE Visualizer SHALL support panning the chart along the time axis via drag interaction.
3. THE Visualizer SHALL provide a reset button that restores the chart to the full time range.
4. THE Visualizer SHALL support displaying heartbeat data from multiple clusters, each identified by cluster name.
5. WHEN multiple clusters are loaded, THE Visualizer SHALL render each cluster as a separate chart panel or as selectable tabs.
6. THE Visualizer SHALL render the chart responsively, adapting width to the browser viewport between 320px and 1920px.

### Requirement 9: Accessible and Responsive UI

**User Story:** As a user, I want the Visualizer to be usable on different screen sizes and with assistive technologies, so that the tool is inclusive and flexible.

#### Acceptance Criteria

1. THE Visualizer SHALL use semantic HTML elements (header, main, footer, nav) for page structure.
2. THE Visualizer SHALL provide ARIA labels for all interactive controls (sound toggle, language toggle, restart button, eBPF correlation button).
3. THE Visualizer SHALL support keyboard navigation for all interactive elements with visible focus indicators.
4. THE Visualizer SHALL maintain a minimum color contrast ratio of 4.5:1 for all text against its background.
5. THE Visualizer SHALL adapt layout from a single-column view on viewports narrower than 768px to a wider layout on larger viewports.

### Requirement 10: Lease Data Parser with Round-Trip Verification

**User Story:** As a developer, I want the Lease_Parser to reliably convert between YAML and JSON formats, so that data integrity is guaranteed through the pipeline.

#### Acceptance Criteria

1. WHEN a valid `leases.yaml` file is provided, THE Lease_Parser SHALL parse it into a namespaced JSON structure grouped by namespace with `{x: index, y: timestamp}` entries.
2. IF an invalid or missing `leases.yaml` file is provided, THEN THE Lease_Parser SHALL exit with a descriptive error message and non-zero exit code.
3. THE Lease_Parser SHALL generate output filenames with a timestamp in `YYYYMMDDTHHmmss` format.
4. THE Lease_Parser SHALL provide a serializer that converts the namespaced JSON structure back into a valid YAML Lease list.
5. FOR ALL valid leases.yaml inputs, parsing to JSON then serializing back to YAML then parsing again SHALL produce an equivalent JSON structure (round-trip property).

### Requirement 11: Go Server Unit and Integration Tests

**User Story:** As a developer, I want comprehensive test coverage for the Go server and Kubernetes client code, so that regressions are caught before deployment.

#### Acceptance Criteria

1. THE Heartbeat_Server SHALL have unit tests covering the POST heartbeat handler for valid input, malformed input, and wrong HTTP method scenarios.
2. THE Heartbeat_Server SHALL have unit tests covering the GET heartbeats handler for empty store and populated store scenarios.
3. THE K8s_Monitor SHALL have unit tests for the `CorrelateEBPFEvent` function covering matching and non-matching cgroup paths.
4. THE K8s_Monitor SHALL have unit tests for `GenerateMockNodes` verifying the function returns 50 nodes with valid fields.
5. THE K8s_Monitor SHALL have unit tests for `GenerateMockEBPFEvents` verifying event count and field population.
6. THE Heartbeat_Server SHALL have integration tests verifying the full POST-then-GET flow returns the posted heartbeat data.

### Requirement 12: Visualizer Component Tests

**User Story:** As a developer, I want automated tests for the React components, so that UI changes do not introduce visual or functional regressions.

#### Acceptance Criteria

1. THE Visualizer SHALL have unit tests for the `hasWarning` utility function covering gaps below, within, and above the warning range.
2. THE Visualizer SHALL have unit tests for the `hasDeath` utility function covering gaps below and above the critical threshold.
3. THE Visualizer SHALL have unit tests for the `getAnomalies` utility function verifying correct anomaly detection across multiple namespaces.
4. THE Visualizer SHALL have unit tests for the `getSegmentColor` function verifying green for normal intervals, yellow for warning intervals, and red for critical intervals.
5. THE Visualizer SHALL have rendering tests for the HeartbeatChart component verifying that chart elements render with mock data.
6. THE Visualizer SHALL have rendering tests for the ChartControls component verifying that all control buttons render and respond to click events.
7. THE Visualizer SHALL have rendering tests for the App component verifying that the header, chart, and footer are present.

### Requirement 13: End-to-End Data Flow Test

**User Story:** As a developer, I want an end-to-end test that validates the full pipeline from heartbeat ingestion to visualization, so that integration issues are detected early.

#### Acceptance Criteria

1. THE test suite SHALL include an end-to-end test that posts a Heartbeat_Event to the Heartbeat_Server, retrieves it via the GET endpoint, and verifies the response matches the posted data.
2. THE test suite SHALL include an end-to-end test that verifies a Heartbeat_Event posted to the Heartbeat_Server is received by a WebSocket client connected to `/ws/heartbeats`.
3. THE test suite SHALL include a test that verifies the Visualizer renders a data point received from a mocked WebSocket_Stream.
