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
  type: 'heartbeat' | 'alert' | 'status' | 'ebpf_event' | 'causal_chain' | 'prediction';
  payload: HeartbeatEvent | Alert | EnrichedKernelEvent | CausalChainMessage['payload'] | PredictionMessage['payload'] | Record<string, unknown>;
}

// --- New types for multi-view visualizations ---

export type ViewType = 'line' | 'heatmap' | 'timeline' | 'histogram' | 'table';

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
  eventType: 'syscall' | 'process' | 'network';
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
