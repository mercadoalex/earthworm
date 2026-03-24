import React from 'react';
import type { ClusterConfig } from './config';

interface ClusterSelectorProps {
  clusters: ClusterConfig[];
  selectedIndex: number;
  onSelect: (index: number) => void;
}

const ClusterSelector: React.FC<ClusterSelectorProps> = ({ clusters, selectedIndex, onSelect }) => {
  if (clusters.length <= 1) return null;

  return (
    <nav aria-label="Cluster selector" style={{ marginBottom: '12px', textAlign: 'center' }}>
      <div
        role="tablist"
        aria-label="Select cluster"
        style={{
          display: 'inline-flex',
          gap: '2px',
          background: '#1a1a1a',
          borderRadius: '6px',
          padding: '3px',
        }}
      >
        {clusters.map((cluster, idx) => (
          <button
            key={cluster.name}
            role="tab"
            aria-selected={idx === selectedIndex}
            aria-label={`View cluster ${cluster.name}`}
            tabIndex={idx === selectedIndex ? 0 : -1}
            onClick={() => onSelect(idx)}
            onKeyDown={(e) => {
              if (e.key === 'ArrowRight') {
                e.preventDefault();
                onSelect((idx + 1) % clusters.length);
              } else if (e.key === 'ArrowLeft') {
                e.preventDefault();
                onSelect((idx - 1 + clusters.length) % clusters.length);
              }
            }}
            style={{
              padding: '6px 16px',
              fontSize: '0.8rem',
              background: idx === selectedIndex ? '#333' : 'transparent',
              color: idx === selectedIndex ? '#fff' : '#888',
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
              fontWeight: idx === selectedIndex ? 600 : 400,
              transition: 'background 0.2s, color 0.2s',
            }}
          >
            {cluster.name}
          </button>
        ))}
      </div>
    </nav>
  );
};

export default ClusterSelector;
