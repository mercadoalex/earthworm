import React, { useEffect, useState, useCallback } from 'react';
import { config } from '../config';
import type { ConnectionRecord } from '../types/heartbeat';

interface NetworkTopologyViewProps {
  wsLastMessage?: { type: string; payload?: unknown } | null;
}

const NetworkTopologyView: React.FC<NetworkTopologyViewProps> = ({ wsLastMessage }) => {
  const [connections, setConnections] = useState<ConnectionRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchTopology = useCallback(async () => {
    try {
      const res = await fetch(`${config.apiBaseUrl}/api/network/topology`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: ConnectionRecord[] = await res.json();
      setConnections(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch topology');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchTopology();
  }, [fetchTopology]);

  // Listen for network_topology_update WebSocket messages
  useEffect(() => {
    if (!wsLastMessage || (wsLastMessage as any).type !== 'network_topology_update') return;
    const record = (wsLastMessage as any).payload as ConnectionRecord;
    if (!record) return;
    setConnections((prev) => {
      const key = `${record.sourcePod}|${record.dstAddr}|${record.dstPort}|${record.protocol}`;
      const idx = prev.findIndex(
        (c) => `${c.sourcePod}|${c.dstAddr}|${c.dstPort}|${c.protocol}` === key,
      );
      if (idx >= 0) {
        const updated = [...prev];
        updated[idx] = record;
        return updated;
      }
      return [record, ...prev];
    });
  }, [wsLastMessage]);

  if (loading) {
    return <div role="status" style={{ color: '#ccc', textAlign: 'center', padding: '20px' }}>Loading topology...</div>;
  }

  if (error) {
    return (
      <div role="alert" style={{ color: config.colors.warning, textAlign: 'center', padding: '20px' }}>
        {error}
      </div>
    );
  }

  return (
    <div aria-label="Network topology view" style={{ width: '100%', overflowX: 'auto' }}>
      <table style={{
        width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem', color: '#ccc',
      }}>
        <thead>
          <tr style={{ borderBottom: '1px solid #444' }}>
            <th style={thStyle}>Source Pod</th>
            <th style={thStyle}>Namespace</th>
            <th style={thStyle}>Destination</th>
            <th style={thStyle}>Port</th>
            <th style={thStyle}>Protocol</th>
            <th style={thStyle}>Last Seen</th>
          </tr>
        </thead>
        <tbody>
          {connections.length === 0 && (
            <tr><td colSpan={6} style={{ textAlign: 'center', padding: '20px', color: '#666' }}>No connections recorded</td></tr>
          )}
          {connections.map((c, i) => (
            <tr key={i} style={{ borderBottom: '1px solid #333' }}>
              <td style={tdStyle}>{c.sourcePod || '(host)'}</td>
              <td style={tdStyle}>{c.sourceNamespace || '—'}</td>
              <td style={tdStyle}>{c.dstAddr}</td>
              <td style={tdStyle}>{c.dstPort}</td>
              <td style={tdStyle}>{c.protocol}</td>
              <td style={tdStyle}>{new Date(c.lastSeen).toLocaleTimeString('en-US', { hour12: false })}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};

const thStyle: React.CSSProperties = {
  textAlign: 'left', padding: '8px 12px', color: '#aaa', fontWeight: 600,
};

const tdStyle: React.CSSProperties = {
  padding: '6px 12px',
};

export default NetworkTopologyView;
