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

// --- Cluster name (should be loaded from config.yaml or API) ---
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
  const [currentEbpfFileIdx, setCurrentEbpfFileIdx] = useState(0);
  const [ebpfData, setEbpfData] = useState([]);
  const [showEbpf, setShowEbpf] = useState(true); // Show eBPF markers by default

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
      .catch(() => setStep('nodata'));

    // Load corresponding eBPF file (by index)
    if (ebpfManifest.length > 0) {
      const ebpfFile = ebpfManifest[currentFileIdx] || ebpfManifest[0];
      fetch(`${DATASET_PATH}${ebpfFile}`)
        .then(res => res.json())
        .then(data => setEbpfData(data))
        .catch(() => setEbpfData([]));
    }
    setShowEbpf(true); // Show eBPF markers by default
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

  console.log('ebpfData', ebpfData);
  console.log('chartData', chartData);

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

  // --- Helper to check if any namespace is in death state ---
  function hasAnyDeath(leasesData) {
    return Object.values(leasesData).some(data => {
      if (!data || data.length < 2) return false;
      for (let i = 1; i < data.length; i++) {
        if (data[i].y - data[i - 1].y > 40000) return true;
      }
      return false;
    });
  }

  // --- Continuous beep effect for death events ---
  useEffect(() => {
    let beepInterval;
    if (
      noise &&
      leasesData &&
      typeof leasesData === 'object' &&
      hasAnyDeath(leasesData) &&
      audioRef.current
    ) {
      beepInterval = setInterval(() => {
        audioRef.current.currentTime = 0;
        audioRef.current.play();
      }, 400);
    }
    return () => {
      if (beepInterval) clearInterval(beepInterval);
    };
  }, [noise, leasesData]);

  // --- Helper: get anomalies and deaths for summary panel ---
  function getEvents(leasesData) {
    if (!leasesData) return [];
    const events = [];
    Object.entries(leasesData).forEach(([ns, arr]) => {
      if (!arr || arr.length < 2) return;
      for (let i = 1; i < arr.length; i++) {
        const gap = arr[i].y - arr[i - 1].y;
        if (gap > 10000 && gap < 40000) {
          events.push({
            type: 'warning',
            namespace: ns,
            index: i,
            gap,
            from: arr[i - 1].y,
            to: arr[i].y
          });
        }
        if (gap > 40000) {
          events.push({
            type: 'death',
            namespace: ns,
            index: i,
            gap,
            from: arr[i - 1].y,
            to: arr[i].y
          });
        }
      }
    });
    return events;
  }

  // --- Helper: get eBPF markers for current chart data ---
  function getEbpfMarkers(chartData, ebpfData) {
    if (!showEbpf || !ebpfData || ebpfData.length === 0) return [];
    const markers = [];
    chartData.forEach(point => {
      ebpfData.forEach(event => {
        // Match by timestamp (±5s) and namespace
        if (
          point.timestamp &&
          event.timestamp &&
          Math.abs(event.timestamp - point.timestamp) < 5000 &&
          namespaces.includes(event.namespace)
        ) {
          markers.push({
            x: point.timestamp,
            namespace: event.namespace,
            label: `${event.comm} (${event.syscall})`,
            color: EBPF_COLOR
          });
        }
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
    if (!props.active || !props.payload || props.payload.length === 0) return null;
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
              //setCurrentHeartbeat(0);
              setSelectedIdx(null);
            }}
            timestamp={chartData.length ? chartData[chartData.length - 1].timestamp : null}
            leasesData={leasesData}
            events={getEvents(leasesData)}
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
              ? namespaces.map(ns => {
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
            {showEbpf && getEbpfMarkers(chartData, ebpfData).map((marker, idx) => {
              const event = ebpfData.find(
                e =>
                  Math.abs(e.timestamp - marker.x) < 5000 &&
                  e.namespace === marker.namespace
              );
              return (
                <ReferenceDot
                  key={idx}
                  x={marker.x}
                  //y={namespaces.indexOf(marker.namespace)}
                  y={chartData.find(d => d.timestamp === marker.x)?.[marker.namespace]}
                  r={7}
                  fill={marker.color}
                  stroke="#fff"
                  label={marker.label}
                  shape={<EbpfMarkerShape payload={event} />}
                />
              );
            })}
          </LineChart>
          {renderLegend()}
        </>
      )}
      {/* No data UI */}
      {step === 'nodata' && (
        <div style={{ textAlign: 'center', padding: '2rem', color: '#e00', fontSize: '1.2rem' }}>
          No more data was found or connectivity was lost.
        </div>
      )}
    </div>
  );
};

function EbpfMarkerShape({ cx, cy, payload }) {
  if (payload && payload.syscall === 'exit') {
    return (
      <g>
        <circle cx={cx} cy={cy} r={7} fill="#fff" stroke="#e00" strokeWidth={2} />
        <text x={cx} y={cy + 4} textAnchor="middle" fontSize="14" fill="#e00" fontWeight="bold">✖</text>
      </g>
    );
  }
  if (payload && payload.syscall === 'fork') {
    return (
      <g>
        <circle cx={cx} cy={cy} r={7} fill="#fff" stroke="#0e0" strokeWidth={2} />
        <text x={cx} y={cy + 4} textAnchor="middle" fontSize="14" fill="#0e0" fontWeight="bold">★</text>
      </g>
    );
  }
  return (
    <circle cx={cx} cy={cy} r={7} fill={EBPF_COLOR} stroke="#fff" strokeWidth={2} />
  );
}

export default HeartbeatChart;