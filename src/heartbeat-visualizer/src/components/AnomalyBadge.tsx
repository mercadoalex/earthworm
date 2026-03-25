import React, { useMemo } from 'react';
import type { LeasesByNamespace } from '../types/heartbeat';
import { getAnomalies } from '../utils/chartUtils';

// Feature: realistic-data-and-visualizations

interface AnomalyBadgeProps {
  leasesData: LeasesByNamespace | null;
  onZoomToAnomaly?: (from: number, to: number) => void;
}

const AnomalyBadge: React.FC<AnomalyBadgeProps> = ({ leasesData, onZoomToAnomaly }) => {
  const anomalies = useMemo(() => getAnomalies(leasesData), [leasesData]);
  const count = anomalies.length;

  const handleClick = () => {
    if (count === 0 || !onZoomToAnomaly) return;
    // Zoom to the most recent anomaly
    const mostRecent = anomalies.reduce((latest, a) =>
      a.to > latest.to ? a : latest,
    );
    onZoomToAnomaly(mostRecent.from, mostRecent.to);
  };

  return (
    <button
      aria-label={`${count} anomalies detected`}
      data-testid="anomaly-badge"
      onClick={handleClick}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 6,
        padding: '4px 12px',
        fontSize: '0.8rem',
        background: count > 0 ? '#5a1a1a' : '#333',
        color: count > 0 ? '#ff9999' : '#888',
        border: `1px solid ${count > 0 ? '#ff4444' : '#555'}`,
        borderRadius: 16,
        cursor: count > 0 && onZoomToAnomaly ? 'pointer' : 'default',
      }}
    >
      <span
        style={{
          display: 'inline-block',
          width: 8,
          height: 8,
          borderRadius: '50%',
          background: count > 0 ? '#ff4444' : '#666',
        }}
      />
      {count} {count === 1 ? 'anomaly' : 'anomalies'}
    </button>
  );
};

export default AnomalyBadge;
