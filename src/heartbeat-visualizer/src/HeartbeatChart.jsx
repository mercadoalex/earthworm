import React, { useEffect, useState, useRef } from 'react';
import ChartControls from './ChartControls';
import { LineChart, Line, XAxis, YAxis, Tooltip, CartesianGrid, ReferenceDot } from 'recharts';

// --- Data source URLs and constants ---
const MANIFEST_URL = '/mocking_data/leases.manifest.json'; // Heartbeat datasets
const EBPF_MANIFEST_URL = '/mocking_data/ebpf-leases.manifest.json'; // eBPF datasets
const DATASET_PATH = '/mocking_data/';
const BEEP_SRC = '/beep.mp3';

const HEARTBEAT_INTERVAL = 10000; // 10 seconds per heartbeat animation step

// --- Color constants ---
const NAMESPACE_COLOR = 'rgb(11, 238, 121)';
const DEATH_COLOR = '#e00';
const EBPF_COLOR = '#ff2050'; // Color for eBPF markers
const HOVER_COLORS = [
  'rgb(54, 162, 235)', 'rgb(153, 102, 255)', 'rgb(0, 204, 204)', 'rgb(255, 102, 204)',
  'rgb(153, 102, 51)', 'rgb(128, 128, 128)', 'rgb(255, 0, 255)', 'rgb(0, 102, 204)',
  'rgb(102, 0, 204)', 'rgb(0, 153, 153)', 'rgb(204, 102, 255)'
];

const CLUSTER_NAME = 'production-us-west-1';

const HeartbeatChart = () => {
  // --- State variables ---
  const [manifest, setManifest] = useState([]);
  const [currentFileIdx, setCurrentFileIdx] = useState(0);
  const [leasesData, setLeasesData] = useState(null);
  const [currentHeartbeat, setCurrentHeartbeat] = useState(0);
  const [step, setStep] = useState('sync');
  const [noise, setNoise] = useState(false);
  const audioRef = useRef(null);
  const [hoveredIdx, setHoveredIdx] = useState(null);
  const [selectedIdx, setSelectedIdx] = useState(null);

  // --- eBPF state ---
  const [ebpfManifest, setEbpfManifest] = useState([]);
  const [ebpfData, setEbpfData] = useState([]);
  const [showEbpf, setShowEbpf] = useState(true);

  // --- Selected eBPF event for info panel ---
  const [selectedEbpfEvent, setSelectedEbpfEvent] = useState(null);

  // --- Hovered eBPF marker index ---
  const [hoveredEbpfIdx, setHoveredEbpfIdx] = useState(null);

  // --- Clear selected eBPF event when clicking outside markers/info panel ---
  useEffect(() => {
    function handleClick(e) {
      // Only clear if clicking outside the info panel and markers
      if (
        !e.target.closest('.ebpf-info-panel') &&
        !e.target.closest('.recharts-reference-dot')
      ) {
        setSelectedEbpfEvent(null);
      }
    }
    document.addEventListener('click', handleClick);
    return () => document.removeEventListener('click', handleClick);
  }, []);

  // --- 1. Fetch heartbeat manifest on mount ---
  useEffect(() => {
    fetch(MANIFEST_URL)
      .then(res => res.json())
      .then(files => {
        const sorted = files.sort((a, b) => {
          if (a === 'leases.json') return -1;
          if (b === 'leases.json') return 1;
          return a.localeCompare(b);
        });
        setManifest(sorted);
        if (!files || files.length === 0) setStep('nodata');
      })
      .catch(() => setStep('nodata'));
  }, []);

  // --- 2. Fetch eBPF manifest on mount ---
  useEffect(() => {
    fetch(EBPF_MANIFEST_URL)
      .then(res => res.json())
      .then(files => setEbpfManifest(files))
      .catch(() => setEbpfManifest([]));
  }, []);

  // --- 3. When in 'sync' step, load heartbeat and eBPF data ---
  useEffect(() => {
    if (step !== 'sync' || manifest.length === 0) return;
    setCurrentHeartbeat(0);
    const file = manifest[currentFileIdx];
    fetch(`${DATASET_PATH}${file}`)
      .then(res => res.json())
      .then(data => {
        setLeasesData(data);
        setTimeout(() => setStep('animate'), 1000);
      })
      .catch(() => {
        setLeasesData([]);
        setStep('nodata');
      });

    if (ebpfManifest.length > 0) {
      const ebpfFile = ebpfManifest[currentFileIdx] || ebpfManifest[0];
      fetch(`${DATASET_PATH}${ebpfFile}`)
        .then(res => res.json())
        .then(data => {
          setEbpfData(Array.isArray(data) ? data : []);
        })
        .catch(() => setEbpfData([]));
    } else {
      setEbpfData([]);
    }
    setShowEbpf(true);
  }, [step, manifest, currentFileIdx, ebpfManifest]);

  // --- 4. Animate heartbeats robustly ---
  useEffect(() => {
    if (step !== 'animate' || !leasesData) return;
    const totalHeartbeats = Math.max(...Object.values(leasesData).map(nsArr => nsArr.length));
    if (currentHeartbeat < totalHeartbeats - 1) {
      const timer = setTimeout(() => {
        setCurrentHeartbeat(hb => hb + 1);
      }, HEARTBEAT_INTERVAL);
      return () => clearTimeout(timer);
    } else {
      setTimeout(() => setStep('pause'), 2000);
    }
  }, [step, leasesData, currentHeartbeat, currentFileIdx]);

  // --- 5. Play beep sound on heartbeat if noise is enabled ---
  useEffect(() => {
    if (step === 'animate' && noise && audioRef.current) {
      audioRef.current.currentTime = 0;
      audioRef.current.play();
    }
  }, [currentHeartbeat, step, noise]);

  // --- 6. In 'pause' step, go to next dataset or finish ---
  useEffect(() => {
    if (step !== 'pause') return;
    if (currentFileIdx < manifest.length - 1) {
      setTimeout(() => {
        setCurrentFileIdx(idx => idx + 1);
        setStep('sync');
      }, 1000);
    } else {
      setTimeout(() => setStep('nodata'), 1200);
    }
  }, [step, currentFileIdx, manifest.length]);

  // --- 7. Prepare data for Recharts ---
  let chartData = [];
  let namespaces = [];
  if (leasesData) {
    namespaces = Object.keys(leasesData);
    const maxPoints = currentHeartbeat + 1;
    for (let i = 0; i < maxPoints; i++) {
      const point = { index: i };
      for (const ns of namespaces) {
        if (leasesData[ns][i] && leasesData[ns][i].y) {
          point.timestamp = leasesData[ns][i].y;
          break;
        }
      }
      namespaces.forEach(ns => {
        if (leasesData[ns][i]) {
          point[ns] = leasesData[ns][i].x;
        }
      });
      chartData.push(point);
    }
  }

  // --- Debug logs for troubleshooting ---
  console.log('chartData:', chartData);
  console.log('namespaces:', namespaces);

  // --- Helper: returns true if any gap between consecutive y values is >10s and <40s (warning) ---
  function hasWarning(data) {
    if (!data || data.length < 2) return false;
    for (let i = 1; i < data.length; i++) {
      const gap = data[i].y - data[i - 1].y;
      if (gap > 10000 && gap < 40000) return true;
    }
    return false;
  }

  // --- Helper: returns true if any gap between consecutive y values is >40s (death) ---
  function hasDeath(data) {
    if (!data || data.length < 2) return false;
    for (let i = 1; i < data.length; i++) {
      const gap = data[i].y - data[i - 1].y;
      if (gap > 40000) return true;
    }
    return false;
  }

  // --- Helper: get eBPF markers for current chart data ---
  // Returns array of marker objects with offset for overlapping events
  function getEbpfMarkers(chartData, ebpfData) {
    console.log('ebpfData:', ebpfData);
    console.log('chartData:', chartData);
    console.log('namespaces:', namespaces);
    console.log('showEbpf:', showEbpf);

    // --- DEBUG: Print matches for each chart point and namespace ---
    chartData.forEach(point => {
      namespaces.forEach(ns => {
        const matchingEvents = ebpfData.filter(event =>
          event.namespace === ns &&
          Math.abs(event.timestamp - point.timestamp) < 60000 // <-- 1 minute window for debugging
        );
        if (matchingEvents.length > 0) {
          console.log(
            `MATCH: ns=${ns}, point.timestamp=${point.timestamp}, events=`,
            matchingEvents
          );
        }
      });
    });

    if (!showEbpf || !ebpfData || ebpfData.length === 0) return [];
    const markers = [];
    chartData.forEach(point => {
      namespaces.forEach(ns => {
        // Find all eBPF events for this namespace and timestamp (±1min for debugging)
        const matchingEvents = ebpfData.filter(event =>
          event.namespace === ns &&
          Math.abs(event.timestamp - point.timestamp) < 60000 // <-- 1 minute window for debugging
        );
        matchingEvents.forEach((event, i) => {
          markers.push({
            x: point.timestamp,
            y: point[ns],
            namespace: ns,
            event,
            offset: (i - Math.floor(matchingEvents.length / 2)) * 12 // Spread overlapping markers
          });
        });
      });
    });
    return markers;
  }

  // --- Legend renderer: namespace labels centered below the chart, clickable to show only one line ---
  const renderLegend = () => (
    <div
      style={{
        display: 'flex',
        flexWrap: 'wrap',
        justifyContent: 'center',
        alignItems: 'flex-start',
        width: '1000px',
        margin: '0px auto 12px',
        gap: '50px',
        overflow: 'hidden'
      }}
    >
      {namespaces.map((ns, idx) => {
        const warning = hasWarning(leasesData[ns]);
        const death = hasDeath(leasesData[ns]);
        const color = death
          ? DEATH_COLOR
          : selectedIdx === idx
          ? HOVER_COLORS[idx % HOVER_COLORS.length]
          : hoveredIdx === idx
          ? HOVER_COLORS[idx % HOVER_COLORS.length]
          : NAMESPACE_COLOR;
        return (
          <span
            key={ns}
            onMouseEnter={() => setHoveredIdx(idx)}
            onMouseLeave={() => setHoveredIdx(null)}
            onClick={() => setSelectedIdx(selectedIdx === idx ? null : idx)}
            style={{
              color: color,
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
              cursor: 'pointer'
            }}
          >
            {ns} {death ? '💀' : warning ? '⚠️' : ''}
          </span>
        );
      })}
    </div>
  );

  // --- Custom Tooltip renderer to show cluster name at the top ---
  const CustomTooltip = (props) => {
    if (!props.active || !props.payload || !props.payload.length === 0) return null;
    return (
      <div style={{
        background: '#222',
        color: '#ccc',
        border: 'none',
        borderRadius: '4px',
        fontSize: '0.95rem',
        padding: '10px 16px',
        minWidth: '180px'
      }}>
        <div style={{
          color: '#ccc',
          fontWeight: 600,
          fontSize: '0.98rem',
          marginBottom: '6px',
          textAlign: 'center'
        }}>
          {CLUSTER_NAME}
        </div>
        <div>
          <strong>{props.labelFormatter ? props.labelFormatter(props.label) : props.label}</strong>
        </div>
        {props.payload.map((entry, idx) => (
          <div key={idx} style={{ color: entry.stroke }}>
            {entry.name}: {entry.value}
          </div>
        ))}
      </div>
    );
  };

  // --- Get all eBPF markers for current chart ---
  const ebpfMarkers = getEbpfMarkers(chartData, ebpfData);

  // --- Get the last marker for auto info ---
  const lastEbpfMarker = ebpfMarkers.length > 0 ? ebpfMarkers[ebpfMarkers.length - 1] : null;

  // --- Render logic ---
  return (
    <div>
      {/* Audio element for beep sound */}
      <audio ref={audioRef} src={BEEP_SRC} preload="auto" />
      {/* Synchronizing/loading UI */}
      {step === 'sync' && (
        <div style={{ textAlign: 'center', padding: '2rem', color: '#ccc', fontSize: '1.1rem' }}>
          Synchronizing...
          <div style={{
            margin: '0.5rem auto 0',
            width: '60%',
            height: '6px',
            background: '#333',
            borderRadius: '3px',
            overflow: 'hidden'
          }}>
            <div style={{
              width: '100%',
              height: '100%',
              background: 'linear-gradient(90deg, rgb(11,238,121) 40%, #222 100%)',
              animation: 'syncBar 1s linear forwards'
            }} />
          </div>
          <style>
            {`
              @keyframes syncBar {
                from { width: 0%; }
                to { width: 100%; }
              }
            `}
          </style>
        </div>
      )}
      {/* Main chart UI */}
      {(step === 'animate' || step === 'pause') && (
        <>
          {/* Controls bar (sound, language, restart, summary panel, ebpf toggle) */}
          <ChartControls
            noise={noise}
            onNoiseToggle={() => setNoise(n => !n)}
            language={'en'}
            onLanguageToggle={() => {/* ... */}}
            onRestart={() => {
              setSelectedIdx(null);
              setSelectedEbpfEvent(null);
            }}
            timestamp={chartData.length ? chartData[chartData.length - 1].timestamp : null}
            leasesData={leasesData}
            events={[]} // You can pass getEvents(leasesData) if you want
            showEbpf={showEbpf}
            onEbpfCorrelate={() => setShowEbpf(v => !v)}
          />
          {/* Heartbeat chart */}
          <LineChart width={1000} height={500} data={chartData}>
            <CartesianGrid stroke="#ccc" />
            <XAxis
              dataKey="timestamp"
              tickFormatter={tick => {
                const date = new Date(tick);
                return date.toLocaleTimeString('en-US', { hour12: false });
              }}
              tick={{ fontSize: 11, fill: '#ccc' }}
            />
            <YAxis
              tick={{ fontSize: 11, fill: '#ccc' }}
            />
            <Tooltip
              contentStyle={{
                background: '#222',
                color: '#ccc',
                border: 'none',
                borderRadius: '4px',
                fontSize: '0.95rem'
              }}
              itemStyle={{
                color:
                  selectedIdx !== null
                    ? HOVER_COLORS[selectedIdx % HOVER_COLORS.length]
                    : NAMESPACE_COLOR
              }}
              labelFormatter={label => `heartbeat: ${label}`}
              content={CustomTooltip}
            />
            {/* Render heartbeat lines */}
            {selectedIdx === null
              ? namespaces.map((ns, idx) => {
                  const death = hasDeath(leasesData[ns]);
                  return (
                    <Line
                      key={ns}
                      type="monotone"
                      dataKey={ns}
                      stroke={death ? DEATH_COLOR : NAMESPACE_COLOR}
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
                    hasDeath(leasesData[namespaces[selectedIdx]])
                      ? DEATH_COLOR
                      : HOVER_COLORS[selectedIdx % HOVER_COLORS.length]
                  }
                  dot={false}
                  strokeWidth={2}
                  isAnimationActive={false}
                />
              )
            }
            {/* Render eBPF markers with custom shapes if enabled */}
            {showEbpf && ebpfMarkers.map((marker, idx) => (
              <ReferenceDot
                key={idx}
                x={marker.x}
                y={marker.y}
                r={7}
                fill={EBPF_COLOR}
                stroke="#fff"
                shape={({ cx, cy }) =>
                  typeof cx === 'number' && typeof cy === 'number' ? (
                    <g
                      className="recharts-reference-dot"
                      style={{ cursor: 'pointer' }}
                      onClick={e => {
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
                  ) : null
                }
              />
            ))}
          </LineChart>
          {renderLegend()}
          {/* --- eBPF Info Panel: Show info for selected marker or last marker automatically --- */}
          {(selectedEbpfEvent || lastEbpfMarker) && (
            <div
              className="ebpf-info-panel"
              style={{
                background: '#222',
                color: '#fff',
                padding: '12px',
                margin: '10px auto 2',
                borderRadius: '6px',
                width: '1000px', // Match chart width
                fontSize: '1rem',
                boxShadow: '0 2px 8px #0008'
              }}
            >
              <strong>eBPF Event Info</strong><br />
              Namespace: {(selectedEbpfEvent || lastEbpfMarker.event).namespace}<br />
              Timestamp: {
                (() => {
                  const ts = (selectedEbpfEvent || lastEbpfMarker.event).timestamp;
                  const date = new Date(ts);
                  return date.toLocaleString('en-US', {
                    year: '2-digit',
                    month: '2-digit',
                    day: '2-digit',
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit',
                    hour12: false
                  });
                })()
              }<br />
              Comm: {(selectedEbpfEvent || lastEbpfMarker.event).comm} ¬
              Syscall: {(selectedEbpfEvent || lastEbpfMarker.event).syscall} ¬
              PID: {(selectedEbpfEvent || lastEbpfMarker.event).pid} ¬
              {selectedEbpfEvent && (
                <div style={{ marginTop: '8px', fontSize: '0.8em', color: '#aaa' }}>
                  (Click anywhere outside a marker to clear selection)
                </div>
              )}
            </div>
          )}
        </>
      )}
      {/* No data UI */}
      {step === 'nodata' && (
        <div style={{ textAlign: 'center', padding: '2rem', color: DEATH_COLOR, fontSize: '1.2rem' }}>
          No more data was found or connectivity was lost.
        </div>
      )}
    </div>
  );
};

/**
 * Custom shape for eBPF markers.
 * - "exit": red X
 * - "fork": green star
 * - others: default eBPF color dot
 */
function EbpfMarkerShape({ cx, cy, payload, hovered }) {
  const markerFill = hovered ? '#fff200' : (payload && payload.syscall === 'exit' ? '#fff' : payload && payload.syscall === 'fork' ? '#fff' : EBPF_COLOR);
  const markerStroke = hovered ? '#ff2050' : (payload && payload.syscall === 'exit' ? '#e00' : payload && payload.syscall === 'fork' ? '#0e0' : '#fff');
  if (payload && payload.syscall === 'exit') {
    return (
      <g>
        <circle cx={cx} cy={cy} r={7} fill={markerFill} stroke={markerStroke} strokeWidth={2} />
        <text x={cx} y={cy + 4} textAnchor="middle" fontSize="14" fill="#e00" fontWeight="bold">✖</text>
      </g>
    );
  }
  if (payload && payload.syscall === 'fork') {
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
}

export default HeartbeatChart;