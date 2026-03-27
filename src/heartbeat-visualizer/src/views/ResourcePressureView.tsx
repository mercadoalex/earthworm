import React, { useMemo } from 'react';
import { config } from '../config';
import type { EnrichedKernelEvent, CgroupResourceEvent } from '../types/heartbeat';

interface ResourcePressureViewProps {
  events: EnrichedKernelEvent[];
}

interface PodResource {
  podName: string;
  namespace: string;
  cpuUsageNs: number;
  memoryUsageBytes: number;
  memoryLimitBytes: number;
  memoryPressure: boolean;
  lastSeen: number;
}

const ResourcePressureView: React.FC<ResourcePressureViewProps> = ({ events }) => {
  const podResources = useMemo(() => {
    const map = new Map<string, PodResource>();
    for (const e of events) {
      if (e.eventType !== 'cgroup_resource') continue;
      const cgroupEvent = e as CgroupResourceEvent;
      const key = `${e.podName || e.comm}|${e.namespace || ''}`;
      const existing = map.get(key);
      if (!existing || e.timestamp > existing.lastSeen) {
        map.set(key, {
          podName: e.podName || e.comm,
          namespace: e.namespace || '',
          cpuUsageNs: cgroupEvent.cpuUsageNs ?? 0,
          memoryUsageBytes: cgroupEvent.memoryUsageBytes ?? 0,
          memoryLimitBytes: cgroupEvent.memoryLimitBytes ?? 0,
          memoryPressure: cgroupEvent.memoryPressure ?? false,
          lastSeen: e.timestamp,
        });
      }
    }
    return Array.from(map.values()).sort((a, b) => {
      if (a.memoryPressure !== b.memoryPressure) return a.memoryPressure ? -1 : 1;
      return b.memoryUsageBytes - a.memoryUsageBytes;
    });
  }, [events]);

  if (podResources.length === 0) {
    return (
      <div aria-label="Resource pressure view" role="status" style={{ color: '#666', textAlign: 'center', padding: '20px', fontSize: '0.85rem' }}>
        No cgroup resource events received yet
      </div>
    );
  }

  return (
    <div aria-label="Resource pressure view" style={{ width: '100%', overflowX: 'auto' }}>
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem', color: '#ccc' }}>
        <thead>
          <tr style={{ borderBottom: '1px solid #444' }}>
            <th style={thStyle}>Pod</th>
            <th style={thStyle}>Namespace</th>
            <th style={thStyle}>CPU (s)</th>
            <th style={thStyle}>Memory</th>
            <th style={thStyle}>Limit</th>
            <th style={thStyle}>Usage %</th>
            <th style={thStyle}>Status</th>
          </tr>
        </thead>
        <tbody>
          {podResources.map((p, i) => {
            const memPct = p.memoryLimitBytes > 0
              ? (p.memoryUsageBytes / p.memoryLimitBytes) * 100
              : 0;
            const pressureColor = p.memoryPressure ? config.colors.death : memPct > 90 ? config.colors.warning : config.colors.healthy;
            return (
              <tr key={i} style={{ borderBottom: '1px solid #333' }}>
                <td style={tdStyle}>{p.podName}</td>
                <td style={tdStyle}>{p.namespace || '—'}</td>
                <td style={tdStyle}>{(p.cpuUsageNs / 1e9).toFixed(1)}</td>
                <td style={tdStyle}>{formatBytes(p.memoryUsageBytes)}</td>
                <td style={tdStyle}>{formatBytes(p.memoryLimitBytes)}</td>
                <td style={tdStyle}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                    <div style={{
                      width: '60px', height: '8px', background: '#333', borderRadius: '4px', overflow: 'hidden',
                    }}>
                      <div style={{
                        width: `${Math.min(memPct, 100)}%`, height: '100%',
                        background: pressureColor, borderRadius: '4px',
                      }} />
                    </div>
                    <span style={{ color: pressureColor }}>{memPct.toFixed(0)}%</span>
                  </div>
                </td>
                <td style={tdStyle}>
                  {p.memoryPressure ? (
                    <span data-testid="pressure-indicator" style={{
                      color: config.colors.death, fontWeight: 700, fontSize: '0.7rem',
                      padding: '1px 6px', border: `1px solid ${config.colors.death}`,
                      borderRadius: '3px',
                    }}>PRESSURE</span>
                  ) : (
                    <span style={{ color: config.colors.healthy, fontSize: '0.7rem' }}>OK</span>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
};

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

const thStyle: React.CSSProperties = {
  textAlign: 'left', padding: '8px 12px', color: '#aaa', fontWeight: 600,
};

const tdStyle: React.CSSProperties = {
  padding: '6px 12px',
};

export default ResourcePressureView;
