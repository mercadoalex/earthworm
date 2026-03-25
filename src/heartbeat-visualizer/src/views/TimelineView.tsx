import React, { useMemo, useState } from 'react';
import type { LeasesByNamespace, EbpfEvent, SwimSegment } from '../types/heartbeat';
import { buildSwimSegments } from '../utils/timelineUtils';
import { getStatusColor } from '../utils/chartUtils';
import { getNodeAnomalies } from '../utils/chartUtils';

// Feature: realistic-data-and-visualizations

interface TimelineViewProps {
  leasesData: LeasesByNamespace | null;
  ebpfEvents?: EbpfEvent[];
  width?: number;
}

interface GapDetail {
  segment: SwimSegment;
  x: number;
  y: number;
}

const LANE_HEIGHT = 24;
const LABEL_WIDTH = 120;
const HEADER_HEIGHT = 30;
const LANE_GAP = 2;

const TimelineView: React.FC<TimelineViewProps> = ({ leasesData, ebpfEvents, width = 800 }) => {
  const [gapDetail, setGapDetail] = useState<GapDetail | null>(null);

  const segments = useMemo(
    () => buildSwimSegments(leasesData, ebpfEvents),
    [leasesData, ebpfEvents],
  );

  const anomalySet = useMemo(() => {
    const anomalies = getNodeAnomalies(leasesData);
    const set = new Set<string>();
    anomalies.forEach((a) => set.add(`${a.nodeName}-${a.from}-${a.to}`));
    return set;
  }, [leasesData]);

  const { nodeNames, timeMin, timeMax } = useMemo(() => {
    const nodeSet = new Set<string>();
    let tMin = Infinity;
    let tMax = -Infinity;
    for (const seg of segments) {
      nodeSet.add(seg.nodeName);
      if (seg.start < tMin) tMin = seg.start;
      if (seg.end > tMax) tMax = seg.end;
    }
    return {
      nodeNames: Array.from(nodeSet).sort(),
      timeMin: tMin,
      timeMax: tMax,
    };
  }, [segments]);

  if (!leasesData || segments.length === 0) {
    return <div role="status" aria-label="No data available">No data available</div>;
  }

  const chartWidth = width - LABEL_WIDTH;
  const timeRange = timeMax - timeMin || 1;
  const toX = (t: number) => LABEL_WIDTH + ((t - timeMin) / timeRange) * chartWidth;
  const svgHeight = HEADER_HEIGHT + nodeNames.length * (LANE_HEIGHT + LANE_GAP);

  const nodeIndex = new Map(nodeNames.map((n, i) => [n, i]));

  const isAnomaly = (seg: SwimSegment): boolean => {
    return anomalySet.has(`${seg.nodeName}-${seg.start}-${seg.end}`);
  };

  return (
    <div aria-label="Timeline view" role="img" style={{ position: 'relative', overflowX: 'auto' }}>
      <svg width={width} height={svgHeight} data-testid="timeline-svg">
        {/* Time axis */}
        <g className="timeline-header">
          {Array.from({ length: 6 }).map((_, i) => {
            const t = timeMin + (timeRange * i) / 5;
            return (
              <text
                key={i}
                x={toX(t)}
                y={HEADER_HEIGHT - 8}
                textAnchor="middle"
                fontSize={9}
                fill="#aaa"
              >
                {new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
              </text>
            );
          })}
        </g>

        {/* Swimlanes */}
        {nodeNames.map((node, idx) => (
          <g key={node}>
            <text
              x={LABEL_WIDTH - 4}
              y={HEADER_HEIGHT + idx * (LANE_HEIGHT + LANE_GAP) + LANE_HEIGHT / 2 + 4}
              textAnchor="end"
              fontSize={10}
              fill="#ccc"
            >
              {node.length > 14 ? node.slice(0, 14) + '…' : node}
            </text>
          </g>
        ))}

        {/* Segments */}
        {segments.map((seg, i) => {
          const idx = nodeIndex.get(seg.nodeName);
          if (idx === undefined) return null;
          const x1 = toX(seg.start);
          const x2 = toX(seg.end);
          const y = HEADER_HEIGHT + idx * (LANE_HEIGHT + LANE_GAP);
          const w = Math.max(x2 - x1, 1);
          const color = getStatusColor(seg.status);
          const anomaly = isAnomaly(seg);

          return (
            <rect
              key={i}
              x={x1}
              y={y}
              width={w}
              height={LANE_HEIGHT}
              fill={color}
              opacity={anomaly ? 1 : 0.75}
              rx={2}
              stroke={anomaly ? '#fff' : 'none'}
              strokeWidth={anomaly ? 1.5 : 0}
              className={anomaly ? 'timeline-anomaly' : undefined}
              style={{ cursor: seg.status !== 'Ready' ? 'pointer' : 'default' }}
              onClick={(e) => {
                if (seg.status !== 'Ready') {
                  setGapDetail({ segment: seg, x: e.clientX, y: e.clientY });
                }
              }}
              data-testid="timeline-segment"
              data-status={seg.status}
              data-node={seg.nodeName}
            />
          );
        })}

        {/* eBPF event markers */}
        {ebpfEvents && ebpfEvents.map((evt, i) => {
          const idx = nodeIndex.get(evt.pod) ?? nodeIndex.get(evt.namespace);
          if (idx === undefined) return null;
          if (evt.timestamp < timeMin || evt.timestamp > timeMax) return null;
          const cx = toX(evt.timestamp);
          const cy = HEADER_HEIGHT + idx * (LANE_HEIGHT + LANE_GAP) + LANE_HEIGHT / 2;
          return (
            <circle
              key={`ebpf-${i}`}
              cx={cx}
              cy={cy}
              r={4}
              fill="#ff2050"
              opacity={0.9}
              data-testid="ebpf-marker"
              data-node={evt.pod || evt.namespace}
              data-timestamp={evt.timestamp}
            >
              <title>{`${evt.comm} (${evt.syscall}) @ ${new Date(evt.timestamp).toLocaleTimeString()}`}</title>
            </circle>
          );
        })}
      </svg>

      {/* Gap detail panel */}
      {gapDetail && (
        <div
          role="dialog"
          aria-label="Gap detail"
          data-testid="gap-detail-panel"
          style={{
            position: 'fixed',
            left: gapDetail.x + 12,
            top: gapDetail.y + 12,
            background: '#222',
            color: '#eee',
            padding: '10px 14px',
            borderRadius: 6,
            fontSize: '0.8rem',
            zIndex: 1000,
            border: '1px solid #555',
            maxWidth: 300,
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
            <strong>{gapDetail.segment.nodeName}</strong>
            <button
              onClick={() => setGapDetail(null)}
              aria-label="Close gap detail"
              style={{ background: 'none', border: 'none', color: '#aaa', cursor: 'pointer' }}
            >
              ✕
            </button>
          </div>
          <div>Status: {gapDetail.segment.status}</div>
          <div>Duration: {((gapDetail.segment.end - gapDetail.segment.start) / 1000).toFixed(1)}s</div>
          {gapDetail.segment.cause && <div>Cause: {gapDetail.segment.cause}</div>}
          <div>From: {new Date(gapDetail.segment.start).toLocaleTimeString()}</div>
          <div>To: {new Date(gapDetail.segment.end).toLocaleTimeString()}</div>
          {gapDetail.segment.ebpfEvents && gapDetail.segment.ebpfEvents.length > 0 && (
            <div style={{ marginTop: 4 }}>
              <strong>eBPF Events:</strong>
              {gapDetail.segment.ebpfEvents.map((e, i) => (
                <div key={i} style={{ fontSize: '0.75rem' }}>
                  {e.comm} ({e.syscall}) @ {new Date(e.timestamp).toLocaleTimeString()}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      <style>{`
        .timeline-anomaly {
          filter: drop-shadow(0 0 3px rgba(255, 255, 255, 0.6));
        }
      `}</style>
    </div>
  );
};

export default TimelineView;
