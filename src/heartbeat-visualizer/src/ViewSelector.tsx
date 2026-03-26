import React from 'react';
import { useViewContext } from './contexts/ViewContext';
import type { ViewType } from './types/heartbeat';

const VIEW_OPTIONS: { key: ViewType; label: string }[] = [
  { key: 'line', label: 'Line Chart' },
  { key: 'heatmap', label: 'Heatmap' },
  { key: 'timeline', label: 'Timeline' },
  { key: 'histogram', label: 'Histogram' },
  { key: 'table', label: 'Table' },
  { key: 'network-topology', label: 'Network Topology' },
  { key: 'resource-pressure', label: 'Resource Pressure' },
];

const ViewSelector: React.FC = () => {
  const { activeView, setActiveView } = useViewContext();

  return (
    <nav aria-label="View selector" style={{ display: 'flex', justifyContent: 'center', gap: '4px', margin: '8px 0' }}>
      {VIEW_OPTIONS.map(({ key, label }) => (
        <button
          key={key}
          role="tab"
          aria-selected={activeView === key}
          aria-label={label}
          onClick={() => setActiveView(key)}
          style={{
            padding: '6px 14px',
            fontSize: '0.85rem',
            border: '1px solid #555',
            borderRadius: '4px',
            cursor: 'pointer',
            background: activeView === key ? '#444' : '#222',
            color: activeView === key ? '#fff' : '#aaa',
            fontWeight: activeView === key ? 'bold' : 'normal',
          }}
        >
          {label}
        </button>
      ))}
    </nav>
  );
};

export default ViewSelector;
