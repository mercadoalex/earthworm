import React, { useMemo, useState } from 'react';
import { BarChart, Bar, XAxis, YAxis, Tooltip, CartesianGrid, Cell, ResponsiveContainer } from 'recharts';
import type { LeasesByNamespace, HistogramBin } from '../types/heartbeat';
import { buildHistogramBins } from '../utils/histogramUtils';
import { getStatusColor } from '../utils/chartUtils';

// Feature: realistic-data-and-visualizations

interface HistogramViewProps {
  leasesData: LeasesByNamespace | null;
  namespaceFilter?: string;
}

const severityToStatus: Record<string, string> = {
  normal: 'ready',
  warning: 'warning',
  critical: 'critical',
};

const HistogramView: React.FC<HistogramViewProps> = ({ leasesData, namespaceFilter }) => {
  const bins = useMemo(
    () => buildHistogramBins(leasesData, 1, namespaceFilter),
    [leasesData, namespaceFilter],
  );

  if (!leasesData || bins.length === 0) {
    return <div role="status" aria-label="No intervals to display">No intervals to display</div>;
  }

  const chartData = bins.map((bin) => ({
    name: `${bin.rangeStart.toFixed(0)}–${bin.rangeEnd.toFixed(0)}s`,
    count: bin.count,
    percentage: bin.percentage,
    severity: bin.severity,
    rangeStart: bin.rangeStart,
    rangeEnd: bin.rangeEnd,
  }));

  return (
    <div aria-label="Histogram view" data-testid="histogram-view">
      {namespaceFilter && (
        <div style={{ fontSize: '0.8rem', color: '#aaa', marginBottom: 4 }}>
          Filtered: {namespaceFilter}
        </div>
      )}
      <ResponsiveContainer width="100%" height={300}>
        <BarChart data={chartData} margin={{ top: 10, right: 20, bottom: 20, left: 20 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#333" />
          <XAxis
            dataKey="name"
            tick={{ fontSize: 10, fill: '#aaa' }}
            label={{ value: 'Interval (s)', position: 'insideBottom', offset: -10, fill: '#aaa', fontSize: 11 }}
          />
          <YAxis
            tick={{ fontSize: 10, fill: '#aaa' }}
            label={{ value: 'Count', angle: -90, position: 'insideLeft', fill: '#aaa', fontSize: 11 }}
          />
          <Tooltip
            content={({ active, payload }) => {
              if (!active || !payload || !payload[0]) return null;
              const d = payload[0].payload;
              return (
                <div
                  data-testid="histogram-tooltip"
                  style={{
                    background: '#222',
                    color: '#eee',
                    padding: '8px 12px',
                    borderRadius: 6,
                    fontSize: '0.8rem',
                    border: '1px solid #555',
                  }}
                >
                  <div>Range: {d.rangeStart.toFixed(1)}s – {d.rangeEnd.toFixed(1)}s</div>
                  <div>Count: {d.count}</div>
                  <div>Percentage: {d.percentage.toFixed(1)}%</div>
                </div>
              );
            }}
          />
          <Bar dataKey="count" isAnimationActive={false}>
            {chartData.map((entry, idx) => (
              <Cell
                key={idx}
                fill={getStatusColor(severityToStatus[entry.severity] || 'ready')}
              />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
};

export default HistogramView;
