import React, { useMemo, useState } from 'react';
import type { LeasesByNamespace, EbpfEvent, SwimSegment, EnrichedKernelEvent, CausalChainMessage } from '../types/heartbeat';
import { buildSwimSegments } from '../utils/timelineUtils';
import { getStatusColor } from '../utils/chartUtils';
import { getNodeAnomalies } from '../utils/chartUtils';
import { config } from '../config';

// Feature: realistic-data-and-visualizations

interface TimelineViewProps {
  leasesData: LeasesByNamespace | null;
  ebpfEvents?: EbpfEvent[];
  causalChain?: CausalChainMessage['payload'] | null;
  width?: number;
}

interface GapDetail {
  segment: SwimSegment;
  x: number;
  y: number;
}

interface ReplayEvent {
  events: EnrichedKernelEvent[];
  loading: boolean;
}

const LANE_HEIGHT = 24;
const LABEL_WIDTH = 120;
const HEADER_HEIGHT = 30;
const LANE_GAP = 2;

const TimelineView: React.FC<TimelineViewProps> = ({ leasesData, ebpfEvents, causalChain, width = 800 }) => {
  const [gapDetail, setGapDetail] = useState<GapDetail | null>(null);
  const [replayData, setReplayData] = useState<ReplayEvent>({ events: [], loading: false });

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

        {/* Causal chain markers and arrows */}
        {causalChain && causalChain.events.length > 0 && (() => {
          const chainEvents = causalChain.events.filter(evt => {
            const idx = nodeIndex.get(evt.nodeName);
            return idx !== undefined && evt.timestamp >= timeMin && evt.timestamp <= timeMax;
          });
          return (
            <g data-testid="causal-chain-overlay">
              <defs>
                <marker id="causal-arrow" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
                  <path d="M0,0 L8,3 L0,6" fill="#ffa500" />
                </marker>
              </defs>
              {chainEvents.map((evt, i) => {
                const idx = nodeIndex.get(evt.nodeName);
                if (idx === undefined) return null;
                const cx = toX(evt.timestamp);
                const cy = HEADER_HEIGHT + idx * (LANE_HEIGHT + LANE_GAP) + LANE_HEIGHT / 2;
                return (
                  <g key={`causal-${i}`}>
                    <circle
                      cx={cx}
                      cy={cy}
                      r={5}
                      fill="none"
                      stroke="#ffa500"
                      strokeWidth={2}
                      data-testid="causal-marker"
                    >
                      <title>{`[Causal] ${evt.eventType}: ${evt.comm} @ ${new Date(evt.timestamp).toLocaleTimeString()}`}</title>
                    </circle>
                    {/* Arrow to next event in chain */}
                    {i < chainEvents.length - 1 && (() => {
                      const next = chainEvents[i + 1];
                      const nextIdx = nodeIndex.get(next.nodeName);
                      if (nextIdx === undefined) return null;
                      const nx = toX(next.timestamp);
                      const ny = HEADER_HEIGHT + nextIdx * (LANE_HEIGHT + LANE_GAP) + LANE_HEIGHT / 2;
                      return (
                        <line
                          x1={cx + 6}
                          y1={cy}
                          x2={nx - 6}
                          y2={ny}
                          stroke="#ffa500"
                          strokeWidth={1.5}
                          markerEnd="url(#causal-arrow)"
                          opacity={0.7}
                          data-testid="causal-arrow-line"
                        />
                      );
                    })()}
                  </g>
                );
              })}
            </g>
          );
        })()}
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
              onClick={() => { setGapDetail(null); setReplayData({ events: [], loading: false }); }}
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

          {/* Causal chain summary */}
          {causalChain && causalChain.nodeName === gapDetail.segment.nodeName && (
            <div style={{ marginTop: 6, padding: '4px 6px', background: '#2a2a1a', borderRadius: 4, border: '1px solid #665500' }} data-testid="causal-chain-summary">
              <strong style={{ color: '#ffa500' }}>Causal Chain:</strong>
              <div style={{ fontSize: '0.75rem', marginTop: 2 }}>{causalChain.summary}</div>
              {causalChain.rootCause && (
                <div style={{ fontSize: '0.75rem', color: '#ff8800' }}>Root cause: {causalChain.rootCause}</div>
              )}
            </div>
          )}

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

          {/* Replay button */}
          <button
            data-testid="replay-button"
            onClick={async () => {
              setReplayData({ events: [], loading: true });
              try {
                const from = new Date(gapDetail.segment.start).toISOString();
                const to = new Date(gapDetail.segment.end).toISOString();
                const resp = await fetch(
                  `${config.apiBaseUrl}/api/replay?node=${encodeURIComponent(gapDetail.segment.nodeName)}&from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`
                );
                if (resp.ok) {
                  const data = await resp.json();
                  setReplayData({ events: data.events || [], loading: false });
                } else {
                  setReplayData({ events: [], loading: false });
                }
              } catch {
                setReplayData({ events: [], loading: false });
              }
            }}
            style={{
              marginTop: 6,
              padding: '4px 10px',
              fontSize: '0.75rem',
              background: '#335',
              color: '#aaf',
              border: '1px solid #558',
              borderRadius: 4,
              cursor: 'pointer',
              width: '100%',
            }}
          >
            {replayData.loading ? 'Loading…' : '▶ Replay'}
          </button>

          {/* Replay event list */}
          {replayData.events.length > 0 && (
            <div style={{ marginTop: 4, maxHeight: 150, overflowY: 'auto' }} data-testid="replay-event-list">
              <strong>Replay ({replayData.events.length} events):</strong>
              {replayData.events.map((evt, i) => (
                <div key={i} style={{ fontSize: '0.7rem', borderBottom: '1px solid #333', padding: '2px 0' }}>
                  {new Date(evt.timestamp).toLocaleTimeString()} — {evt.eventType}: {evt.comm}
                  {evt.syscallName && ` (${evt.syscallName})`}
                  {evt.podName && ` [${evt.podName}]`}
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
