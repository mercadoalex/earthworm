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
  type: 'heartbeat' | 'alert' | 'status';
  payload: HeartbeatEvent | Alert | Record<string, unknown>;
}
