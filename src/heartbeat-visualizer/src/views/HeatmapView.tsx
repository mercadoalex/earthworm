import React, { useMemo, useState } from 'react';
import type { LeasesByNamespace, HeatmapCell, EbpfEvent } from '../types/heartbeat';
import { buildHeatmapData } from '../utils/heatmapUtils';
import { getStatusColor } from '../utils/chartUtils';
import { getNodeAnomalies } from '../utils/chartUtils';

// Feature: realistic-data-and-visualizations

interface HeatmapViewProps {
  leasesData: LeasesByNamespace | null;
  ebpfEvents?: EbpfEvent[];
  width?: number;
}

type SortMode = 'health' | 'alpha';

interface TooltipData {
  nodeName: string;
  timeRange: string;
  status: string;
  heartbeatCount: number;
  x: number;
  y: number;
}

const CELL_HEIGHT = 20;
const CELL_MIN_WIDTH = 6;
const LABEL_WIDTH = 120;
const HEADER_HEIGHT = 30;
const MAX_VISIBLE_ROWS = 30;

const statusPriority: Record<string, number> = {
  critical: 0,
  warning: 1,
  ready: 2,
};

const HeatmapView: React.FC<HeatmapViewProps> = ({ leasesData, ebpfEvents, width = 800 }) => {
  const [sortMode, setSortMode] = useState<SortMode>('health');
  const [tooltip, setTooltip] = useState<TooltipData | null>(null);

  const cells = useMemo(() => buildHeatmapData(leasesData), [leasesData]);

  const anomalySet = useMemo(() => {
    const anomalies = getNodeAnomalies(leasesData);
    const set = new Set<string>();
    anomalies.forEach((a) => {
      set.add(`${a.nodeName}-${a.from}-${a.to}`);
    });
    return set;
  }, [leasesData]);

  // Derive unique nodes and time buckets
  const { nodeNames, timeBuckets, cellMap } = useMemo(() => {
    const nodeSet = new Set<string>();
    const bucketSet = new Set<number>();
    const map = new Map<string, HeatmapCell>();

    for (const cell of cells) {
      nodeSet.add(cell.nodeName);
      bucketSet.add(cell.timeBucket);
      map.set(`${cell.nodeName}-${cell.timeBucket}`, cell);
    }

    const buckets = Array.from(bucketSet).sort((a, b) => a - b);

    // Sort nodes
    let nodes = Array.from(nodeSet);
    if (sortMode === 'health') {
      // Worst health first: find worst status per node
      const worstStatus = new Map<string, number>();
      for (const cell of cells) {
        const current = worstStatus.get(cell.nodeName) ?? 2;
        const prio = statusPriority[cell.status] ?? 2;
        if (prio < current) worstStatus.set(cell.nodeName, prio);
      }
      nodes.sort((a, b) => (worstStatus.get(a) ?? 2) - (worstStatus.get(b) ?? 2));
    } else {
      nodes.sort();
    }

    return { nodeNames: nodes, timeBuckets: buckets, cellMap: map };
  }, [cells, sortMode]);

  if (!leasesData || cells.length === 0) {
    return <div role="status" aria-label="No data available">No data available</div>;
  }

  const chartWidth = width - LABEL_WIDTH;
  const cellWidth = Math.max(CELL_MIN_WIDTH, chartWidth / Math.max(timeBuckets.length, 1));
  const svgWidth = LABEL_WIDTH + timeBuckets.length * cellWidth;
  const totalHeight = HEADER_HEIGHT + nodeNames.length * CELL_HEIGHT;
  const needsScroll = nodeNames.length > MAX_VISIBLE_ROWS;
  const visibleHeight = needsScroll
    ? HEADER_HEIGHT + MAX_VISIBLE_ROWS * CELL_HEIGHT
    : totalHeight;

  const isAnomaly = (cell: HeatmapCell): boolean => {
    return anomalySet.has(`${cell.nodeName}-${cell.timeBucket}-${cell.timeBucketEnd}`);
  };

  const handleCellHover = (cell: HeatmapCell, event: React.MouseEvent) => {
    const startDate = new Date(cell.timeBucket).toLocaleTimeString();
    const endDate = new Date(cell.timeBucketEnd).toLocaleTimeString();
    setTooltip({
      nodeName: cell.nodeName,
      timeRange: `${startDate} – ${endDate}`,
      status: cell.status,
      heartbeatCount: cell.heartbeatCount,
      x: event.clientX,
      y: event.clientY,
    });
  };

  return (
    <div aria-label="Heatmap view" role="img" style={{ position: 'relative' }}>
      <div style={{ marginBottom: 8 }}>
        <button
          onClick={() => setSortMode(sortMode === 'health' ? 'alpha' : 'health')}
          aria-label={`Sort by ${sortMode === 'health' ? 'alphabetical' : 'health status'}`}
          style={{
            padding: '4px 10px',
            fontSize: '0.8rem',
            background: '#333',
            color: '#ccc',
            border: '1px solid #555',
            borderRadius: 4,
            cursor: 'pointer',
          }}
        >
          Sort: {sortMode === 'health' ? 'Worst Health' : 'Alphabetical'}
        </button>
      </div>

      <div
        style={{
          maxHeight: visibleHeight,
          overflowY: needsScroll ? 'auto' : 'visible',
          overflowX: 'auto',
          position: 'relative',
        }}
      >
        <svg
          width={svgWidth}
          height={totalHeight}
          data-testid="heatmap-svg"
        >
          {/* Time axis header */}
          <g className="heatmap-header">
            {timeBuckets.filter((_, i) => i % Math.max(1, Math.floor(timeBuckets.length / 10)) === 0).map((t, i) => {
              const idx = timeBuckets.indexOf(t);
              return (
                <text
                  key={i}
                  x={LABEL_WIDTH + idx * cellWidth + cellWidth / 2}
                  y={HEADER_HEIGHT - 8}
                  textAnchor="middle"
                  fontSize={9}
                  fill="#aaa"
                >
                  {new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                </text>
              );
            })}
          </g>

          {/* Node labels and cells */}
          {nodeNames.map((node, rowIdx) => (
            <g key={node}>
              <text
                x={LABEL_WIDTH - 4}
                y={HEADER_HEIGHT + rowIdx * CELL_HEIGHT + CELL_HEIGHT / 2 + 4}
                textAnchor="end"
                fontSize={10}
                fill="#ccc"
              >
                {node.length > 14 ? node.slice(0, 14) + '…' : node}
              </text>
              {timeBuckets.map((t, colIdx) => {
                const cell = cellMap.get(`${node}-${t}`);
                if (!cell) return null;
                const color = getStatusColor(cell.status);
                const anomaly = isAnomaly(cell);
                return (
                  <rect
                    key={`${node}-${t}`}
                    x={LABEL_WIDTH + colIdx * cellWidth}
                    y={HEADER_HEIGHT + rowIdx * CELL_HEIGHT}
                    width={cellWidth - 1}
                    height={CELL_HEIGHT - 1}
                    fill={color}
                    opacity={0.85}
                    rx={1}
                    className={anomaly ? 'heatmap-anomaly-pulse' : undefined}
                    stroke={anomaly ? '#fff' : 'none'}
                    strokeWidth={anomaly ? 1.5 : 0}
                    onMouseEnter={(e) => handleCellHover(cell, e)}
                    onMouseLeave={() => setTooltip(null)}
                    data-testid="heatmap-cell"
                    data-status={cell.status}
                    data-node={cell.nodeName}
                  />
                );
              })}
            </g>
          ))}
        </svg>
      </div>

      {/* Tooltip */}
      {tooltip && (
        <div
          role="tooltip"
          data-testid="heatmap-tooltip"
          style={{
            position: 'fixed',
            left: tooltip.x + 12,
            top: tooltip.y + 12,
            background: '#222',
            color: '#eee',
            padding: '8px 12px',
            borderRadius: 6,
            fontSize: '0.8rem',
            pointerEvents: 'none',
            zIndex: 1000,
            border: '1px solid #555',
          }}
        >
          <div><strong>{tooltip.nodeName}</strong></div>
          <div>{tooltip.timeRange}</div>
          <div>Status: {tooltip.status}</div>
          <div>Heartbeats: {tooltip.heartbeatCount}</div>
        </div>
      )}

      <style>{`
        @keyframes anomalyPulse {
          0%, 100% { stroke-opacity: 1; }
          50% { stroke-opacity: 0.3; }
        }
        .heatmap-anomaly-pulse {
          animation: anomalyPulse 1.5s ease-in-out infinite;
        }
      `}</style>
    </div>
  );
};

export default HeatmapView;
