// src/types/index.ts

// This file defines TypeScript types and interfaces used throughout the Earthworm project.
// It includes types for heartbeat data and any other relevant structures, with comments explaining their purpose.

// HeartbeatData interface represents the structure of the heartbeat data collected from Kubernetes clusters.
// It includes properties for the timestamp, cluster ID, and the heartbeat status.
export interface HeartbeatData {
    timestamp: string; // The time when the heartbeat was recorded, in ISO format.
    clusterId: string; // Unique identifier for the Kubernetes cluster.
    status: 'healthy' | 'unhealthy'; // Status of the heartbeat, indicating if the cluster is healthy or not.
}

// HeartbeatResponse interface represents the structure of the response sent from the server to the UI.
// It includes an array of HeartbeatData objects and a message for additional context.
export interface HeartbeatResponse {
    data: HeartbeatData[]; // Array of heartbeat data objects.
    message: string; // Additional message or status from the server.
}

// This file can be expanded with more types and interfaces as the project grows.
// For example, you might want to add types for error responses, configuration settings, or other data structures used in the application.