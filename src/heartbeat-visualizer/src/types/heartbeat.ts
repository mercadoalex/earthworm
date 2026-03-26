export interface EbpfMetadata {
  pid: number;
  comm: string;
  syscall: string;
  cgroupPath: string;
}

export interface HeartbeatEvent {
  nodeName: string;
  namespace: string;
  timestamp: number; // epoch ms
  status: 'healthy' | 'unhealthy';
  ebpf?: EbpfMetadata;
}

export interface LeasePoint {
  x: number; // index
  y: number; // timestamp ms
}

export interface LeasesByNamespace {
  [namespace: string]: LeasePoint[];
}

export interface Alert {
  nodeName: string;
  namespace: string;
  gapSeconds: number;
  severity: 'warning' | 'critical';
  timestamp: number;
}

export interface ChartControlsProps {
  noise: boolean;
  onNoiseToggle: () => void;
  language: string;
  onLanguageToggle: () => void;
  onRestart: () => void;
  timestamp: number | null;
  leasesData: LeasesByNamespace | null;
  showEbpf: boolean;
  onEbpfCorrelate: () => void;
  clearEbpfData?: () => void;
  restoreEbpfData?: () => void;
}

export interface EbpfEvent {
  timestamp: number;
  namespace: string;
  pod: string;
  pid: number;
  comm: string;
  syscall: string;
}

export interface EbpfMarker {
  x: number;
  y: number;
  namespace: string;
  event: EbpfEvent;
  offset: number;
}

export interface Anomaly {
  namespace: string;
  index: number;
  gap: number;
  from: number;
  to: number;
}

export interface ChartDataPoint {
  index: number;
  timestamp?: number;
  [namespace: string]: number | undefined;
}

export interface WebSocketMessage {
  type: 'heartbeat' | 'alert' | 'status' | 'ebpf_event' | 'causal_chain' | 'prediction' | 'network_topology_update';
  payload: HeartbeatEvent | Alert | EnrichedKernelEvent | CausalChainMessage['payload'] | PredictionMessage['payload'] | NetworkTopologyUpdate['payload'] | Record<string, unknown>;
}

// --- New types for multi-view visualizations ---

export type ViewType = 'line' | 'heatmap' | 'timeline' | 'histogram' | 'table' | 'network-topology' | 'resource-pressure';

export interface HeatmapCell {
  nodeName: string;
  namespace: string;
  timeBucket: number;
  timeBucketEnd: number;
  status: 'ready' | 'warning' | 'critical';
  heartbeatCount: number;
}

export interface SwimSegment {
  nodeName: string;
  namespace: string;
  start: number;
  end: number;
  status: 'Ready' | 'NotReady' | 'Unknown';
  cause?: string;
  ebpfEvents?: EbpfEvent[];
}

export interface NodeSummary {
  nodeName: string;
  namespace: string;
  currentStatus: string;
  lastHeartbeat: number;
  recentIntervals: number[];
}

export interface HistogramBin {
  rangeStart: number;
  rangeEnd: number;
  count: number;
  percentage: number;
  severity: 'normal' | 'warning' | 'critical';
}

export interface NodeAnomaly extends Anomaly {
  nodeName: string;
}

// --- eBPF Kernel Observability types ---

export interface EnrichedKernelEvent {
  timestamp: number;
  pid: number;
  ppid: number;
  comm: string;
  cgroupId: number;
  eventType: 'syscall' | 'process' | 'network' | 'filesystem_io' | 'memory_pressure' | 'dns_resolution' | 'cgroup_resource' | 'network_audit';
  // Syscall-specific
  syscallName?: string;
  returnValue?: number;
  latencyNs?: number;
  slowSyscall?: boolean;
  // Process-specific
  childPid?: number;
  exitCode?: number;
  criticalExit?: boolean;
  // Network-specific
  srcAddr?: string;
  dstAddr?: string;
  srcPort?: number;
  dstPort?: number;
  netEventType?: 'retransmit' | 'reset' | 'rtt_high';
  rttUs?: number;
  // Enrichment
  podName?: string;
  namespace?: string;
  containerName?: string;
  nodeName: string;
  hostLevel?: boolean;
}

export interface CausalChainMessage {
  type: 'causal_chain';
  payload: {
    nodeName: string;
    timestamp: number;
    events: EnrichedKernelEvent[];
    summary: string;
    rootCause: string;
  };
}

export interface PredictionMessage {
  type: 'prediction';
  payload: {
    nodeName: string;
    confidence: number;
    ttfSeconds: number;
    patterns: string[];
  };
}

// --- eBPF Advanced Probes: new event variant interfaces ---

export interface FilesystemIOEvent extends EnrichedKernelEvent {
  eventType: 'filesystem_io';
  filePath: string;
  ioLatencyNs: number;
  bytesXfer: number;
  slowIO: boolean;
  ioOpType: 'read' | 'write';
}

export interface MemoryPressureEvent extends EnrichedKernelEvent {
  eventType: 'memory_pressure';
  oomSubType: 'oom_kill' | 'alloc_failure';
  killedPid?: number;
  killedComm?: string;
  oomScoreAdj?: number;
  pageOrder?: number;
  gfpFlags?: number;
}

export interface DNSResolutionEvent extends EnrichedKernelEvent {
  eventType: 'dns_resolution';
  domain: string;
  dnsLatencyNs: number;
  responseCode: number;
  timedOut: boolean;
}

export interface CgroupResourceEvent extends EnrichedKernelEvent {
  eventType: 'cgroup_resource';
  cpuUsageNs: number;
  memoryUsageBytes: number;
  memoryLimitBytes: number;
  memoryPressure: boolean;
}

export interface NetworkAuditEvent extends EnrichedKernelEvent {
  eventType: 'network_audit';
  auditDstAddr: string;
  auditDstPort: number;
  auditProtocol: 'tcp' | 'udp';
}

// --- Network Topology types ---

export interface ConnectionRecord {
  sourcePod: string;
  sourceNamespace: string;
  dstAddr: string;
  dstPort: number;
  protocol: string;
  lastSeen: string;
  nodeName: string;
}

export interface NetworkTopologyUpdate {
  type: 'network_topology_update';
  payload: ConnectionRecord;
}
