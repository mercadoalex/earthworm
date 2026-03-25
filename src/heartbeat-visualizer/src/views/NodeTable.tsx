import React, { useMemo } from 'react';
import type { LeasesByNamespace } from '../types/heartbeat';
import { buildNodeSummaries } from '../utils/nodeTableUtils';
import { getStatusColor } from '../utils/chartUtils';
import Sparkline from '../components/Sparkline';

// Feature: realistic-data-and-visualizations

interface NodeTableProps {
  leasesData: LeasesByNamespace | null;
}

const NodeTable: React.FC<NodeTableProps> = ({ leasesData }) => {
  const summaries = useMemo(() => buildNodeSummaries(leasesData), [leasesData]);

  if (!leasesData || summaries.length === 0) {
    return <div role="status" aria-label="No data available">No data available</div>;
  }

  return (
    <div aria-label="Node table" data-testid="node-table" style={{ overflowX: 'auto' }}>
      <table
        style={{
          width: '100%',
          borderCollapse: 'collapse',
          fontSize: '0.85rem',
          color: '#ddd',
        }}
      >
        <thead>
          <tr style={{ borderBottom: '1px solid #555' }}>
            <th style={thStyle}>Node Name</th>
            <th style={thStyle}>Namespace</th>
            <th style={thStyle}>Status</th>
            <th style={thStyle}>Last Heartbeat</th>
            <th style={thStyle}>Trend</th>
          </tr>
        </thead>
        <tbody>
          {summaries.map((s) => (
            <tr key={s.nodeName} style={{ borderBottom: '1px solid #333' }} data-testid="node-row">
              <td style={tdStyle}>{s.nodeName}</td>
              <td style={tdStyle}>{s.namespace}</td>
              <td style={tdStyle}>
                <span
                  style={{
                    display: 'inline-block',
                    width: 10,
                    height: 10,
                    borderRadius: '50%',
                    background: getStatusColor(s.currentStatus),
                    marginRight: 6,
                  }}
                />
                {s.currentStatus}
              </td>
              <td style={tdStyle}>
                {new Date(s.lastHeartbeat).toLocaleTimeString()}
              </td>
              <td style={tdStyle}>
                <Sparkline intervals={s.recentIntervals} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};

const thStyle: React.CSSProperties = {
  textAlign: 'left',
  padding: '6px 10px',
  fontWeight: 600,
  color: '#aaa',
};

const tdStyle: React.CSSProperties = {
  padding: '4px 10px',
  verticalAlign: 'middle',
};

export default NodeTable;
