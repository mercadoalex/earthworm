# Earthworm Project

## Overview
The Earthworm project is designed to monitor the heartbeat signals of Kubernetes clusters using eBPF (Extended Berkeley Packet Filter) technology. The project visualizes this heartbeat data in a web-based interface, resembling a cardiogram, allowing users to monitor the health of their Kubernetes environments effectively. The name "Earthworm" symbolizes the project's ability to manage multiple "hearts" (Kubernetes clusters) simultaneously.

## Project Structure
The project is organized into several directories, each serving a specific purpose:

- **src/ebpf**: Contains the eBPF program that intercepts heartbeat signals from Kubernetes clusters.
- **src/kubernetes**: Implements a Kubernetes client that listens for heartbeat events and forwards the data to the server.
- **src/ui**: The React application that visualizes the heartbeat data.
- **src/server**: The Go server that receives heartbeat data and serves it to the UI.
- **src/types**: Defines TypeScript types and interfaces used throughout the project.

## Getting Started

### Prerequisites
- Go (version 1.16 or later)
- Node.js (version 14 or later)
- Kubernetes cluster (for testing)

### Installation
1. Clone the repository:
   ```
   git clone https://github.com/yourusername/earthworm.git
   cd earthworm
   ```

2. Install Go dependencies:
   ```
   cd src/server
   go mod tidy
   ```

3. Install Node.js dependencies:
   ```
   cd src/ui
   npm install
   ```

### Running the Project
1. **Start the Go server**:
   ```
   cd src/server
   go run main.go
   ```

2. **Run the eBPF program**:
   - Compile and load the eBPF program located in `src/ebpf/heartbeat.c` using the appropriate tools (e.g., `clang`, `bpftool`).

3. **Start the React application**:
   ```
   cd src/ui
   npm start
   ```

### Architecture
The Earthworm project consists of the following components:
- **eBPF Program**: Intercepts heartbeat signals from Kubernetes clusters and collects the data.
- **Kubernetes Monitor**: Listens for heartbeat events using the Kubernetes API and forwards the data to the Go server.
- **Go Server**: Receives heartbeat data from the Kubernetes monitor and serves it to the React UI.
- **React UI**: Visualizes the heartbeat data in a graph format, providing real-time updates.

## Conclusion
The Earthworm project provides a comprehensive solution for monitoring Kubernetes cluster health through innovative use of eBPF technology. By visualizing heartbeat data, users can gain insights into the performance and reliability of their Kubernetes environments. This project serves as an educational resource for understanding eBPF, Kubernetes, and modern web application development.

## License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.