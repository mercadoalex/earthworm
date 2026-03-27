import React, { useEffect, useState, useRef, useCallback, useMemo } from 'react';
import { LineChart, Line, XAxis, YAxis, Tooltip, CartesianGrid, ReferenceDot, ReferenceArea } from 'recharts';
import ChartControls from './ChartControls';
import { config } from './config';
import type { ClusterConfig } from './config';
import { hasDeath, hasWarning } from './utils/chartUtils';
import { useHeartbeatData } from './hooks/useHeartbeatData';
import { useEbpfData } from './hooks/useEbpfData';
import { useWebSocket } from './hooks/useWebSocket';
import { useLiveEvents } from './hooks/useLiveEvents';
import { useViewContext } from './contexts/ViewContext';
import type { EbpfEvent, Alert, WebSocketMessage, HeartbeatEvent, ChartDataPoint } from './types/heartbeat';
import LiveActivityPanel from './LiveActivityPanel';
import ViewSelector from './ViewSelector';
import AnomalyBadge from './components/AnomalyBadge';
import HeatmapView from './views/HeatmapView';
import TimelineView from './views/TimelineView';
import HistogramView from './views/HistogramView';
import NodeTable from './views/NodeTable';
import NetworkTopologyView from './views/NetworkTopologyView';
import ResourcePressureView from './views/ResourcePressureView';

// --- Toast notification for alerts ---
interface ToastProps {
  alert: Alert;
  onDismiss: () => void;
}

const Toast: React.FC<ToastProps> = ({ alert, onDismiss }) => {
  useEffect(() => {
    const timer = setTimeout(onDismiss, 10000);
    return () => clearTimeout(timer);
  }, [onDismiss]);

  const bgColor = alert.severity === 'critical' ? config.colors.death : config.colors.warning;
  return (
    <div
      role="alert"
      style={{
        position: 'fixed',
        top: '20px',
        right: '20px',
        background: bgColor,
        color: '#fff',
        padding: '12px 20px',
        borderRadius: '6px',
        boxShadow: '0 4px 12px rgba(0,0,0,0.3)',
        zIndex: 1000,
        fontSize: '0.9rem',
        maxWidth: '350px',
      }}
    >
      <strong>{alert.severity.toUpperCase()}</strong>: {alert.nodeName} ({alert.namespace})
      <br />
      Gap: {alert.gapSeconds.toFixed(1)}s
      <button
        onClick={onDismiss}
        aria-label="Dismiss alert"
        style={{
          position: 'absolute',
          top: '4px',
          right: '8px',
          background: 'none',
          border: 'none',
          color: '#fff',
          cursor: 'pointer',
          fontSize: '1rem',
        }}
      >
        ×
      </button>
    </div>
  );
};

// --- eBPF marker shape ---
interface EbpfMarkerShapeProps {
  cx: number;
  cy: number;
  payload: EbpfEvent;
  hovered: boolean;
}

const EbpfMarkerShape: React.FC<EbpfMarkerShapeProps> = ({ cx, cy, payload, hovered }) => {
  const markerFill = hovered
    ? '#fff200'
    : payload?.syscall === 'exit' || payload?.syscall === 'fork'
      ? '#fff'
      : config.colors.ebpf;
  const markerStroke = hovered
    ? config.colors.ebpf
    : payload?.syscall === 'exit'
      ? config.colors.death
      : payload?.syscall === 'fork'
        ? '#0e0'
        : '#fff';

  if (payload?.syscall === 'exit') {
    return (
      <g>
        <circle cx={cx} cy={cy} r={7} fill={markerFill} stroke={markerStroke} strokeWidth={2} />
        <text x={cx} y={cy + 4} textAnchor="middle" fontSize="14" fill={config.colors.death} fontWeight="bold">✖</text>
      </g>
    );
  }
  if (payload?.syscall === 'fork') {
    return (
      <g>
        <circle cx={cx} cy={cy} r={7} fill={markerFill} stroke={markerStroke} strokeWidth={2} />
        <text x={cx} y={cy + 4} textAnchor="middle" fontSize="14" fill="#0e0" fontWeight="bold">★</text>
      </g>
    );
  }
  return (
    <circle cx={cx} cy={cy} r={7} fill={markerFill} stroke={markerStroke} strokeWidth={2} />
  );
};

// --- Connection status indicator ---
interface ConnectionStatusProps {
  status: 'connecting' | 'connected' | 'disconnected';
}

const ConnectionStatus: React.FC<ConnectionStatusProps> = ({ status }) => {
  if (status === 'connected') return null;
  const color = status === 'connecting' ? config.colors.warning : config.colors.death;
  const label = status === 'connecting' ? 'Connecting...' : 'Disconnected';
  return (
    <div
      role="status"
      aria-live="polite"
      style={{
        position: 'fixed',
        bottom: '20px',
        right: '20px',
        background: color,
        color: '#fff',
        padding: '8px 16px',
        borderRadius: '4px',
        fontSize: '0.85rem',
        zIndex: 999,
        boxShadow: '0 2px 8px rgba(0,0,0,0.3)',
      }}
    >
      {label}
    </div>
  );
};

// --- Custom Tooltip ---
interface CustomTooltipProps {
  active?: boolean;
  payload?: Array<{ name: string; value: number; stroke: string }>;
  label?: string | number;
  labelFormatter?: (label: string | number) => string;
}

const CustomTooltip: React.FC<CustomTooltipProps> = (props) => {
  if (!props.active || !props.payload || props.payload.length === 0) return null;
  return (
    <div style={{
      background: '#222',
      color: '#ccc',
      border: 'none',
      borderRadius: '4px',
      fontSize: '0.95rem',
      padding: '10px 16px',
      minWidth: '180px',
    }}>
      <div style={{
        color: '#ccc',
        fontWeight: 600,
        fontSize: '0.98rem',
        marginBottom: '6px',
        textAlign: 'center',
      }}>
        {config.clusterName}
      </div>
      <div>
        <strong>{props.labelFormatter ? props.labelFormatter(props.label ?? '') : props.label}</strong>
      </div>
      {props.payload.map((entry, idx) => (
        <div key={idx} style={{ color: entry.stroke }}>
          {entry.name}: {entry.value}
        </div>
      ))}
    </div>
  );
};

interface HeartbeatChartProps {
  cluster?: ClusterConfig;
}

const HeartbeatChart: React.FC<HeartbeatChartProps> = ({ cluster }) => {
  // --- Custom hooks for data ---
  const {
    leasesData,
    currentHeartbeat,
    step,
    chartData: baseChartData,
    namespaces: baseNamespaces,
    currentFileIdx,
  } = useHeartbeatData(cluster?.manifestUrl, cluster?.datasetPath);

  const { status: wsStatus, lastMessage } = useWebSocket(cluster?.wsEndpoint);

  // --- Live events and alerts (isolated from chart rendering via useLiveEvents) ---
  const { liveEvents, alerts, dismissAlert } = useLiveEvents(lastMessage);

  // --- Append incoming WS heartbeat events to chart data (Req 5.4) ---
  const [wsChartPoints, setWsChartPoints] = useState<ChartDataPoint[]>([]);
  const lastAppendedMsgRef = useRef<WebSocketMessage | null>(null);

  useEffect(() => {
    if (
      !lastMessage ||
      lastMessage.type !== 'heartbeat' ||
      lastMessage === lastAppendedMsgRef.current
    ) {
      return;
    }
    lastAppendedMsgRef.current = lastMessage;
    const hb = lastMessage.payload as HeartbeatEvent;
    setWsChartPoints((prev) => {
      const nextIndex = (baseChartData.length > 0 ? baseChartData.length : 0) + prev.length;
      const point: ChartDataPoint = {
        index: nextIndex,
        timestamp: hb.timestamp,
        [hb.namespace]: nextIndex,
      };
      return [...prev, point];
    });
  }, [lastMessage, baseChartData.length]);

  // Merge base chart data with WS-appended points
  const chartData = useMemo(
    () => [...baseChartData, ...wsChartPoints],
    [baseChartData, wsChartPoints],
  );

  // Merge namespaces: base namespaces + any new namespaces from WS events
  const namespaces = useMemo(() => {
    const wsNs = new Set<string>();
    for (const pt of wsChartPoints) {
      for (const key of Object.keys(pt)) {
        if (key !== 'index' && key !== 'timestamp') wsNs.add(key);
      }
    }
    const merged = [...baseNamespaces];
    wsNs.forEach((ns) => {
      if (!merged.includes(ns)) merged.push(ns);
    });
    return merged;
  }, [baseNamespaces, wsChartPoints]);

  const {
    showEbpf,
    toggleEbpf,
    clearEbpfData,
    restoreEbpfData,
    ebpfMarkers,
  } = useEbpfData(currentFileIdx, step, cluster?.ebpfManifestUrl, cluster?.datasetPath, chartData, namespaces);

  // --- ViewContext for shared zoom/pan state ---
  const { activeView, xDomain, setXDomain } = useViewContext();

  // --- Local UI state ---
  const [noise, setNoise] = useState(false);
  const audioRef = useRef<HTMLAudioElement>(null);
  const [hoveredIdx, setHoveredIdx] = useState<number | null>(null);
  const [selectedIdx, setSelectedIdx] = useState<number | null>(null);
  const [selectedEbpfEvent, setSelectedEbpfEvent] = useState<EbpfEvent | null>(null);
  const [hoveredEbpfIdx, setHoveredEbpfIdx] = useState<number | null>(null);

  // --- Zoom/Pan state (brush is local, xDomain comes from ViewContext) ---
  const [brushStart, setBrushStart] = useState<number | null>(null);
  const [brushEnd, setBrushEnd] = useState<number | null>(null);
  const [isPanning, setIsPanning] = useState(false);
  const panStartRef = useRef<number | null>(null);
  const panDomainRef = useRef<[number, number] | null>(null);

  // Compute full data range for reset
  const fullRange: [number, number] | null = chartData.length > 0
    ? [chartData[0].timestamp ?? 0, chartData[chartData.length - 1].timestamp ?? 0]
    : null;

  const handleMouseDown = useCallback((e: { activeLabel?: string | number } | null) => {
    if (!e || e.activeLabel === undefined) return;
    const ts = Number(e.activeLabel);
    if (isNaN(ts)) return;
    // Shift+click starts pan, normal click starts brush zoom
    setBrushStart(ts);
    setBrushEnd(null);
  }, []);

  const handleMouseMove = useCallback((e: { activeLabel?: string | number } | null) => {
    if (!e || e.activeLabel === undefined || brushStart === null) return;
    const ts = Number(e.activeLabel);
    if (isNaN(ts)) return;
    setBrushEnd(ts);
  }, [brushStart]);

  const handleMouseUp = useCallback(() => {
    if (brushStart !== null && brushEnd !== null && brushStart !== brushEnd) {
      const left = Math.min(brushStart, brushEnd);
      const right = Math.max(brushStart, brushEnd);
      setXDomain([left, right]);
    }
    setBrushStart(null);
    setBrushEnd(null);
  }, [brushStart, brushEnd, setXDomain]);

  const handleResetZoom = useCallback(() => {
    setXDomain(null);
  }, [setXDomain]);

  // Pan handlers via mouse events on the chart container
  const chartContainerRef = useRef<HTMLDivElement>(null);

  const handlePanStart = useCallback((e: React.MouseEvent) => {
    if (!e.shiftKey || !xDomain) return;
    e.preventDefault();
    setIsPanning(true);
    panStartRef.current = e.clientX;
    panDomainRef.current = xDomain;
  }, [xDomain]);

  const handlePanMove = useCallback((e: React.MouseEvent) => {
    if (!isPanning || panStartRef.current === null || !panDomainRef.current || !fullRange) return;
    const dx = e.clientX - panStartRef.current;
    // Map pixel delta to time delta
    const containerWidth = chartContainerRef.current?.clientWidth || 1000;
    const fullWidth = fullRange[1] - fullRange[0];
    const timeDelta = -(dx / containerWidth) * fullWidth;
    let newLeft = panDomainRef.current[0] + timeDelta;
    let newRight = panDomainRef.current[1] + timeDelta;
    // Clamp to full range
    if (newLeft < fullRange[0]) {
      newRight += fullRange[0] - newLeft;
      newLeft = fullRange[0];
    }
    if (newRight > fullRange[1]) {
      newLeft -= newRight - fullRange[1];
      newRight = fullRange[1];
    }
    setXDomain([Math.max(newLeft, fullRange[0]), Math.min(newRight, fullRange[1])]);
  }, [isPanning, fullRange, setXDomain]);

  const handlePanEnd = useCallback(() => {
    setIsPanning(false);
    panStartRef.current = null;
    panDomainRef.current = null;
  }, []);

  // --- Responsive chart width ---
  const chartWrapperRef = useRef<HTMLDivElement>(null);
  const [chartWidth, setChartWidth] = useState(1000);

  useEffect(() => {
    const el = chartWrapperRef.current;
    if (!el) return;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const w = entry.contentRect.width;
        setChartWidth(Math.max(320, Math.min(w, 1920)));
      }
    });
    observer.observe(el);
    // Set initial width
    setChartWidth(Math.max(320, Math.min(el.clientWidth, 1920)));
    return () => observer.disconnect();
  }, []);

  // --- Clear selected eBPF event on outside click ---
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      const target = e.target as HTMLElement;
      if (
        !target.closest('.ebpf-info-panel') &&
        !target.closest('.recharts-reference-dot')
      ) {
        setSelectedEbpfEvent(null);
      }
    }
    document.addEventListener('click', handleClick);
    return () => document.removeEventListener('click', handleClick);
  }, []);

  // --- Play beep sound on heartbeat ---
  useEffect(() => {
    if (step === 'animate' && noise && audioRef.current) {
      audioRef.current.currentTime = 0;
      audioRef.current.play();
    }
  }, [currentHeartbeat, step, noise]);

  // --- eBPF markers (pre-computed by useEbpfData) ---
  const lastEbpfMarker = ebpfMarkers.length > 0 ? ebpfMarkers[ebpfMarkers.length - 1] : null;

  // --- Legend renderer ---
  const renderLegend = () => (
    <div style={{
      display: 'flex',
      flexWrap: 'wrap',
      justifyContent: 'center',
      alignItems: 'flex-start',
      width: '100%',
      maxWidth: `${chartWidth}px`,
      margin: '0px auto 12px',
      gap: '20px',
      overflow: 'hidden',
    }}>
      {namespaces.map((ns, idx) => {
        const warning = leasesData ? hasWarning(leasesData[ns]) : false;
        const death = leasesData ? hasDeath(leasesData[ns]) : false;
        const color = death
          ? config.colors.death
          : selectedIdx === idx
            ? config.colors.hover[idx % config.colors.hover.length]
            : hoveredIdx === idx
              ? config.colors.hover[idx % config.colors.hover.length]
              : config.colors.healthy;
        return (
          <span
            key={ns}
            onMouseEnter={() => setHoveredIdx(idx)}
            onMouseLeave={() => setHoveredIdx(null)}
            onClick={() => setSelectedIdx(selectedIdx === idx ? null : idx)}
            style={{
              color,
              fontWeight: selectedIdx === idx ? 'bold' : 'normal',
              padding: '2px 8px',
              borderRadius: '2px',
              background: selectedIdx === idx ? '#222' : 'transparent',
              fontSize: '0.85rem',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              flex: '0 1 auto',
              textAlign: 'center',
              transition: 'color 0.2s, background 0.2s',
              cursor: 'pointer',
            }}
          >
            {ns} {death ? '💀' : warning ? '⚠️' : ''}
          </span>
        );
      })}
    </div>
  );

  return (
    <div>
      <audio ref={audioRef} src={config.beepSrc} preload="auto" />

      {/* Toast notifications for alerts */}
      {alerts.map((alert, idx) => (
        <Toast
          key={idx}
          alert={alert}
          onDismiss={() => dismissAlert(idx)}
        />
      ))}

      {/* WebSocket connection status */}
      <ConnectionStatus status={wsStatus} />

      {/* Synchronizing UI */}
      {step === 'sync' && (
        <div style={{ textAlign: 'center', padding: '2rem', color: '#ccc', fontSize: '1.1rem' }}>
          Synchronizing...
          <div style={{
            margin: '0.5rem auto 0',
            width: '60%',
            height: '6px',
            background: '#333',
            borderRadius: '3px',
            overflow: 'hidden',
          }}>
            <div style={{
              width: '100%',
              height: '100%',
              background: `linear-gradient(90deg, ${config.colors.healthy} 40%, #222 100%)`,
              animation: 'syncBar 1s linear forwards',
            }} />
          </div>
          <style>
            {`@keyframes syncBar { from { width: 0%; } to { width: 100%; } }`}
          </style>
        </div>
      )}

      {/* Main chart UI */}
      {(step === 'animate' || step === 'pause') && (
        <div ref={chartWrapperRef} style={{ width: '100%', maxWidth: '1920px', minWidth: '320px' }}>
          <ChartControls
            noise={noise}
            onNoiseToggle={() => setNoise((n) => !n)}
            language="en"
            onLanguageToggle={() => {}}
            onRestart={() => {
              setSelectedIdx(null);
              setSelectedEbpfEvent(null);
            }}
            timestamp={chartData.length ? (chartData[chartData.length - 1].timestamp ?? null) : null}
            leasesData={leasesData}
            showEbpf={showEbpf}
            onEbpfCorrelate={toggleEbpf}
            clearEbpfData={clearEbpfData}
            restoreEbpfData={restoreEbpfData}
          />

          {/* View Selector */}
          <ViewSelector />

          {/* Anomaly Badge */}
          <div style={{ textAlign: 'center', margin: '6px 0' }}>
            <AnomalyBadge
              leasesData={leasesData}
              onZoomToAnomaly={(from, to) => {
                const padding = (to - from) * 0.2;
                setXDomain([from - padding, to + padding]);
              }}
            />
          </div>

          {/* Reset Zoom button */}
          {xDomain && (
            <div style={{ textAlign: 'center', marginBottom: '6px' }}>
              <button
                onClick={handleResetZoom}
                aria-label="Reset zoom to full time range"
                style={{
                  fontSize: '0.75rem',
                  padding: '4px 12px',
                  background: '#333',
                  color: '#ccc',
                  border: '1px solid #555',
                  borderRadius: '3px',
                  cursor: 'pointer',
                }}
              >
                Reset Zoom
              </button>
            </div>
          )}

          {/* Active view based on ViewContext */}
          {activeView === 'line' && (
            <>
              <div
                ref={chartContainerRef}
                onMouseDown={handlePanStart}
                onMouseMove={handlePanMove}
                onMouseUp={handlePanEnd}
                onMouseLeave={handlePanEnd}
                style={{ cursor: isPanning ? 'grabbing' : xDomain ? 'grab' : 'crosshair' }}
              >
              <LineChart
                width={chartWidth}
                height={500}
                data={chartData}
                onMouseDown={handleMouseDown}
                onMouseMove={handleMouseMove}
                onMouseUp={handleMouseUp}
              >
                <CartesianGrid stroke="#ccc" />
                <XAxis
                  dataKey="timestamp"
                  domain={xDomain ?? ['dataMin', 'dataMax']}
                  type="number"
                  tickFormatter={(tick: number) => {
                    const date = new Date(tick);
                    return date.toLocaleTimeString('en-US', { hour12: false });
                  }}
                  tick={{ fontSize: 11, fill: '#ccc' }}
                  allowDataOverflow
                />
                <YAxis tick={{ fontSize: 11, fill: '#ccc' }} />
                <Tooltip
                  contentStyle={{
                    background: '#222',
                    color: '#ccc',
                    border: 'none',
                    borderRadius: '4px',
                    fontSize: '0.95rem',
                  }}
                  itemStyle={{
                    color: selectedIdx !== null
                      ? config.colors.hover[selectedIdx % config.colors.hover.length]
                      : config.colors.healthy,
                  }}
                  labelFormatter={(label) => `heartbeat: ${label}`}
                  content={<CustomTooltip />}
                />

                {/* Render heartbeat lines */}
                {selectedIdx === null
                  ? namespaces.map((ns) => {
                      const death = leasesData ? hasDeath(leasesData[ns]) : false;
                      return (
                        <Line
                          key={ns}
                          type="monotone"
                          dataKey={ns}
                          stroke={death ? config.colors.death : config.colors.healthy}
                          dot={false}
                          strokeWidth={2}
                          isAnimationActive={false}
                        />
                      );
                    })
                  : (
                    <Line
                      key={namespaces[selectedIdx]}
                      type="monotone"
                      dataKey={namespaces[selectedIdx]}
                      stroke={
                        leasesData && hasDeath(leasesData[namespaces[selectedIdx]])
                          ? config.colors.death
                          : config.colors.hover[selectedIdx % config.colors.hover.length]
                      }
                      dot={false}
                      strokeWidth={2}
                      isAnimationActive={false}
                    />
                  )
                }

                {/* eBPF markers */}
                {showEbpf && ebpfMarkers.map((marker, idx) => (
                  <ReferenceDot
                    key={idx}
                    x={marker.x}
                    y={marker.y}
                    r={7}
                    fill={config.colors.ebpf}
                    stroke="#fff"
                    shape={({ cx, cy }: { cx?: number; cy?: number }) =>
                      typeof cx === 'number' && typeof cy === 'number' ? (
                        <g
                          className="recharts-reference-dot"
                          style={{ cursor: 'pointer' }}
                          onClick={(e) => {
                            e.stopPropagation();
                            setSelectedEbpfEvent(marker.event);
                          }}
                          onMouseEnter={() => setHoveredEbpfIdx(idx)}
                          onMouseLeave={() => setHoveredEbpfIdx(null)}
                        >
                          <EbpfMarkerShape
                            payload={marker.event}
                            cx={cx + marker.offset}
                            cy={cy}
                            hovered={hoveredEbpfIdx === idx}
                          />
                        </g>
                      ) : <g />
                    }
                  />
                ))}

                {/* Brush selection area for zoom */}
                {brushStart !== null && brushEnd !== null && (
                  <ReferenceArea
                    x1={brushStart}
                    x2={brushEnd}
                    strokeOpacity={0.3}
                    fill="rgba(54, 162, 235, 0.3)"
                  />
                )}
              </LineChart>
              </div>

              {renderLegend()}

              {/* eBPF Info Panel */}
              {(selectedEbpfEvent || lastEbpfMarker) && (
                <div
                  className="ebpf-info-panel"
                  style={{
                    background: '#222',
                    color: '#fff',
                    padding: '12px',
                    margin: '10px auto 2px',
                    borderRadius: '6px',
                    width: '100%',
                    maxWidth: `${chartWidth}px`,
                    fontSize: '1rem',
                    boxShadow: '0 2px 8px #0008',
                    boxSizing: 'border-box',
                  }}
                >
                  <strong>eBPF Event Info</strong><br />
                  Namespace: {(selectedEbpfEvent || lastEbpfMarker!.event).namespace}<br />
                  Timestamp: {
                    (() => {
                      const ts = (selectedEbpfEvent || lastEbpfMarker!.event).timestamp;
                      const date = new Date(ts);
                      return date.toLocaleString('en-US', {
                        year: '2-digit',
                        month: '2-digit',
                        day: '2-digit',
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit',
                        hour12: false,
                      });
                    })()
                  }<br />
                  Comm: {(selectedEbpfEvent || lastEbpfMarker!.event).comm} ¬
                  Syscall: {(selectedEbpfEvent || lastEbpfMarker!.event).syscall} ¬
                  PID: {(selectedEbpfEvent || lastEbpfMarker!.event).pid} ¬
                  {selectedEbpfEvent && (
                    <div style={{ marginTop: '8px', fontSize: '0.8em', color: '#aaa' }}>
                      (Click anywhere outside a marker to clear selection)
                    </div>
                  )}
                </div>
              )}
            </>
          )}

          {activeView === 'heatmap' && (
            <HeatmapView leasesData={leasesData} width={chartWidth} />
          )}

          {activeView === 'timeline' && (
            <TimelineView leasesData={leasesData} width={chartWidth} />
          )}

          {activeView === 'histogram' && (
            <HistogramView leasesData={leasesData} />
          )}

          {activeView === 'table' && (
            <NodeTable leasesData={leasesData} />
          )}

          {activeView === 'network-topology' && (
            <NetworkTopologyView wsLastMessage={lastMessage} />
          )}

          {activeView === 'resource-pressure' && (
            <ResourcePressureView events={[]} />
          )}

          {/* Persistent NodeTable panel below active chart view */}
          {activeView !== 'table' && (
            <div style={{ marginTop: '16px', borderTop: '1px solid #333', paddingTop: '12px' }}>
              <NodeTable leasesData={leasesData} />
            </div>
          )}
        </div>
      )}

      {/* No data UI */}
      {step === 'nodata' && (
        <div style={{ textAlign: 'center', padding: '2rem', color: config.colors.death, fontSize: '1.2rem' }}>
          No more data was found or connectivity was lost.
        </div>
      )}

      {/* Live Activity Feed — always visible */}
      <LiveActivityPanel events={liveEvents} wsStatus={wsStatus} />
    </div>
  );
};

export default HeartbeatChart;
