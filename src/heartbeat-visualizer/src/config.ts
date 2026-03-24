export interface ClusterConfig {
  name: string;
  manifestUrl: string;
  ebpfManifestUrl: string;
  datasetPath: string;
  wsEndpoint: string;
  apiBaseUrl: string;
}

export const clusters: ClusterConfig[] = [
  {
    name: 'production-us-west-1',
    manifestUrl: '/mocking_data/leases.manifest.json',
    ebpfManifestUrl: '/mocking_data/ebpf-leases.manifest.json',
    datasetPath: '/mocking_data/',
    wsEndpoint: 'ws://localhost:8080/ws/heartbeats',
    apiBaseUrl: 'http://localhost:8080',
  },
];

export const config = {
  heartbeatInterval: 10000,
  warningGapThreshold: 10000,
  criticalGapThreshold: 40000,
  colors: {
    healthy: 'rgb(11, 238, 121)',
    warning: 'rgb(255, 205, 86)',
    critical: 'rgb(255, 99, 132)',
    death: '#e00',
    ebpf: '#ff2050',
    hover: [
      'rgb(54, 162, 235)', 'rgb(153, 102, 255)', 'rgb(0, 204, 204)', 'rgb(255, 102, 204)',
      'rgb(153, 102, 51)', 'rgb(128, 128, 128)', 'rgb(255, 0, 255)', 'rgb(0, 102, 204)',
      'rgb(102, 0, 204)', 'rgb(0, 153, 153)', 'rgb(204, 102, 255)',
    ],
  },
  clusterName: clusters[0].name,
  wsEndpoint: clusters[0].wsEndpoint,
  apiBaseUrl: clusters[0].apiBaseUrl,
  reconnect: {
    initialDelayMs: 1000,
    maxDelayMs: 30000,
    maxRetries: 5,
  },
  manifestUrl: clusters[0].manifestUrl,
  ebpfManifestUrl: clusters[0].ebpfManifestUrl,
  datasetPath: clusters[0].datasetPath,
  beepSrc: '/beep.mp3',
};
