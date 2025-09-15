import React, { useState } from 'react';
import logo from './logo.svg'; // Make sure logo.svg is in the same directory

// --- Button styles for generic buttons ---
const buttonStyle = {
  fontSize: '0.65rem',
  padding: '0.15rem 0.4rem',
  background: '#222',
  color: '#ccc',
  borderWidth: '1px',
  borderStyle: 'solid',
  borderColor: '#222',
  borderRadius: '2px',
  cursor: 'pointer',
  transition: 'background 0.2s, border-color 0.2s'
};

const buttonHoverStyle = {
  background: '#333',
  borderColor: '#22ff99'
};

// --- Toggle switch style for sound ---
const toggleContainerStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: '4px',
  background: '#222',
  borderRadius: '2px',
  padding: '0.15rem 0.4rem',
  color: '#ccc',
  cursor: 'pointer',
  fontSize: '0.65rem',
  border: 'none',
  transition: 'background 0.2s'
};

// --- Helper: Format date as "Monday, April 1st, 2025" ---
function formatFullDate(ms) {
  if (!ms) return '';
  const date = new Date(ms);
  const dayName = date.toLocaleDateString('en-US', { weekday: 'long' });
  const monthName = date.toLocaleDateString('en-US', { month: 'long' });
  const day = date.getDate();
  const ordinal = (n) => {
    if (n > 3 && n < 21) return 'th';
    switch (n % 10) {
      case 1: return 'st';
      case 2: return 'nd';
      case 3: return 'rd';
      default: return 'th';
    }
  };
  const year = date.getFullYear();
  return `${dayName}, ${monthName} ${day}${ordinal(day)}, ${year}`;
}

// --- Helper: Detect anomalies (gaps >10s and <40s) in leasesData ---
function getAnomalies(leasesData) {
  if (!leasesData) return [];
  const anomalies = [];
  Object.entries(leasesData).forEach(([ns, arr]) => {
    if (!arr || arr.length < 2) return;
    for (let i = 1; i < arr.length; i++) {
      const gap = arr[i].y - arr[i - 1].y;
      if (gap > 10000 && gap < 40000) {
        anomalies.push({
          namespace: ns,
          index: i,
          gap,
          from: arr[i - 1].y,
          to: arr[i].y
        });
      }
    }
  });
  return anomalies;
}

// --- ToggleNoise component with custom bar ---
const ToggleNoise = ({ active, onToggle }) => (
  <button
    style={{
      ...toggleContainerStyle,
      background: active ? '#333' : '#222',
      position: 'relative'
    }}
    onClick={onToggle}
    title={active ? 'Disable sound' : 'Enable sound'}
  >
    {/* Visual toggle bar */}
    <span style={{
      display: 'inline-block',
      width: '28px',
      height: '14px',
      borderRadius: '7px',
      background: active ? 'rgb(7, 144, 73)' : '#444',
      position: 'relative',
      marginRight: '8px',
      verticalAlign: 'middle',
      transition: 'background 0.2s'
    }}>
      {/* Toggle knob */}
      <span style={{
        position: 'absolute',
        top: '2px',
        left: active ? '16px' : '2px',
        width: '10px',
        height: '10px',
        borderRadius: '50%',
        background: '#ccc',
        transition: 'left 0.2s'
      }} />
    </span>
    {active ? '🔊 Sound On' : '🔇 Sound Off'}
  </button>
);

// --- Button component for generic buttons ---
const Button = ({ onClick, style, hoverStyle, label }) => {
  const [hover, setHover] = React.useState(false);
  return (
    <button
      style={hover ? { ...style, ...hoverStyle } : style}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      onClick={onClick}
    >
      {label}
    </button>
  );
};

/**
 * ChartControls component
 * Props:
 * - noise: boolean, sound on/off
 * - onNoiseToggle: function, toggles sound
 * - language: string, current language
 * - onLanguageToggle: function, toggles language
 * - onRestart: function, restarts chart
 * - timestamp: number, current timestamp
 * - leasesData: object, heartbeat lease data
 * - showEbpf: boolean, whether to show ebpf markers
 * - onEbpfCorrelate: function, toggles ebpf markers
 * - clearEbpfData: function, sets ebpfData to []
 * - restoreEbpfData: function, reloads ebpfData from file
 */
const ChartControls = ({
  noise,
  onNoiseToggle,
  language,
  onLanguageToggle,
  onRestart,
  timestamp,
  leasesData,
  showEbpf,
  onEbpfCorrelate,
  clearEbpfData,
  restoreEbpfData
}) => {
  // Get all namespace keys from leasesData
  const namespaces = Object.keys(leasesData || {});

  // Helper: returns true if any gap between consecutive y values is >40s (death)
  const hasDeath = (data) => {
    if (!data || data.length < 2) return false;
    for (let i = 1; i < data.length; i++) {
      if (data[i].y - data[i - 1].y > 40000) return true;
    }
    return false;
  };

  // Helper: returns true if any gap between consecutive y values is >10s and <40s (warning)
  const hasWarning = (data) => {
    if (!data || data.length < 2) return false;
    for (let i = 1; i < data.length; i++) {
      const gap = data[i].y - data[i - 1].y;
      if (gap > 10000 && gap < 40000) return true;
    }
    return false;
  };

  // Get namespaces with death or warning events
  const deathNamespaces = namespaces.filter(ns => hasDeath(leasesData[ns]));
  const warningNamespaces = namespaces.filter(ns => hasWarning(leasesData[ns]));

  // State for hover effect on eBPF button
  const [ebpfHover, setEbpfHover] = useState(false);

  // State to show/hide anomaly tooltip
  const [showAnomalyTooltip, setShowAnomalyTooltip] = useState(false);

  // Get anomalies data
  const anomalies = getAnomalies(leasesData);

  // --- eBPF Button click handler ---
  // When clicked, toggles eBPF data and icon state
  const handleEbpfClick = () => {
    if (showEbpf) {
      // Hide eBPF: clear only ebpf data
      if (clearEbpfData) clearEbpfData();
    } else {
      // Show eBPF: restore ebpf data
      if (restoreEbpfData) restoreEbpfData();
    }
    // Toggle the marker visibility state
    if (onEbpfCorrelate) onEbpfCorrelate();
  };

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'row',
        justifyContent: 'flex-end',
        alignItems: 'center',
        marginBottom: '6px',
        maxWidth: '1000px',
        marginLeft: 'auto',
        marginRight: 'auto',
        position: 'relative'
      }}
    >
      {/* Anomaly Summary */}
      <div style={{ marginRight: 0, position: 'relative' }}>
        <div style={{
          background: '#222',
          color: deathNamespaces.length > 0 ? '#e00' : (warningNamespaces.length > 0 ? '#ffcc00' : '#888'),
          borderRadius: '4px',
          padding: '8px 24px',
          fontSize: '0.92rem',
          textAlign: 'left',
          cursor: 'pointer'
        }}>
          <strong
            style={{ marginRight: '12px', fontSize: '0.98rem', cursor: 'pointer' }}
            onClick={() => setShowAnomalyTooltip(v => !v)}
            title="Click to view anomaly details"
          >
            Anomaly Summary:
          </strong>
          {deathNamespaces.length > 0 ? (
            deathNamespaces.length === 1 ? (
              <span>
                Death detected in: <span style={{ fontWeight: 600 }}>{deathNamespaces[0]}</span> 💀
              </span>
            ) : (
              <span>
                {deathNamespaces.length} namespaces are failing{' '}
                {Array(deathNamespaces.length).fill('💀').join('')}
              </span>
            )
          ) : warningNamespaces.length > 0 ? (
            <span>
              Warning detected in: {warningNamespaces.map(ns => (
                <span key={ns} style={{ fontWeight: 600, marginRight: 8 }}>{ns} ⚠️</span>
              ))}
            </span>
          ) : (
            <span style={{ color: '#aaa', fontSize: '0.92rem', marginLeft: '12px' }}>
              NO anomalies detected.
            </span>
          )}
        </div>
        {/* Tooltip box for anomaly details */}
        {showAnomalyTooltip && (
          <div
            style={{
              position: 'absolute',
              top: '110%',
              left: 0,
              background: '#222',
              color: '#ccc',
              borderRadius: '4px',
              boxShadow: '0 2px 12px #0008',
              padding: '16px 24px',
              minWidth: '260px',
              zIndex: 10,
              fontSize: '0.95rem'
            }}
          >
            <div style={{
              color: '#ccc',
              fontWeight: 600,
              fontSize: '0.98rem',
              marginBottom: '8px',
              textAlign: 'center'
            }}>
              Anomaly Details
            </div>
            {anomalies.length === 0 ? (
              <div style={{ color: '#aaa', textAlign: 'center' }}>No anomalies found.</div>
            ) : (
              <ul style={{ paddingLeft: 0, margin: 0 }}>
                {anomalies.map((a, idx) => (
                  <li key={idx} style={{ marginBottom: 6, listStyle: 'none' }}>
                    <span style={{ color: '#ffcc00', fontWeight: 600 }}>{a.namespace}</span>{' '}
                    gap: <span style={{ color: '#e00' }}>{Math.round(a.gap / 1000)}s</span>{' '}
                    at <span style={{ color: '#09f' }}>{new Date(a.to).toLocaleString()}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </div>
      {/* Controls bar: sound, language, restart, date, eBPF button */}
      <div
        style={{
          display: 'flex',
          gap: '6px',
          alignItems: 'center'
        }}
      >
        {/* Date display */}
        <span style={{
          color: '#ccc',
          fontSize: '0.9rem',
          marginRight: '16px',
          fontWeight: 500,
          letterSpacing: '0.02em'
        }}>
          {timestamp ? formatFullDate(timestamp) : ''}
        </span>
        {/* Sound toggle */}
        <ToggleNoise
          active={noise}
          onToggle={onNoiseToggle}
        />
        {/* Language toggle button */}
        <Button
          onClick={onLanguageToggle}
          style={buttonStyle}
          hoverStyle={buttonHoverStyle}
          label={`🌐 ${language === 'en' ? 'SP' : 'EN'}`}
        />
        {/* Restart button */}
        <Button
          onClick={onRestart}
          style={{ ...buttonStyle, fontSize: '0.85rem', padding: '0.15rem 0.6rem', marginLeft: '12px' }}
          hoverStyle={buttonHoverStyle}
          label="Restart"
        />
        {/* eBPF Correlation Button - placed at the right of Restart */}
        <button
          onClick={handleEbpfClick}
          onMouseEnter={() => setEbpfHover(true)}
          onMouseLeave={() => setEbpfHover(false)}
          style={{
            background: '#222',
            border: 'none',
            borderRadius: '50%',
            padding: '6px',
            cursor: 'pointer',
            boxShadow: '0 2px 8px #0003',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            marginLeft: '8px'
          }}
          title={showEbpf ? "Hide eBPF Correlation" : "Show eBPF Correlation"}
        >
          {/* The logo image: normal color by default, grayscale on hover */}
          <img
            src={logo}
            alt="eBPF"
            style={{
              width: 28,
              height: 28,
              filter: ebpfHover ? 'grayscale(1) brightness(0.7)' : 'none',
              transition: 'filter 0.2s'
            }}
          />
        </button>
      </div>
    </div>
  );
};

export default ChartControls;