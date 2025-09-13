import React from 'react';

// --- Button styles ---
const buttonStyle = {
  fontSize: '0.65rem',
  padding: '0.15rem 0.4rem',
  background: '#222',
  color: '#ccc',
  border: 'none',
  borderRadius: '2px',
  cursor: 'pointer',
  transition: 'background 0.2s, border-color 0.2s'
};

const buttonHoverStyle = {
  background: '#333',
  borderColor: '#22ff99'
};

// --- Toggle switch style ---
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

// --- Button component ---
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

// --- Main ChartControls component ---
const ChartControls = ({
  noise,
  onNoiseToggle,
  language,
  onLanguageToggle,
  onRestart,
  timestamp,
  leasesData
}) => {
  const namespaces = Object.keys(leasesData || {});

  const hasDeath = (data) => {
    if (!data || data.length < 2) return false;
    for (let i = 1; i < data.length; i++) {
      if (data[i].y - data[i - 1].y > 40000) return true;
    }
    return false;
  };

  const hasWarning = (data) => {
    if (!data || data.length < 2) return false;
    for (let i = 1; i < data.length; i++) {
      const gap = data[i].y - data[i - 1].y;
      if (gap > 10000 && gap < 40000) return true;
    }
    return false;
  };

  const deathNamespaces = namespaces.filter(ns => hasDeath(leasesData[ns]));
  const warningNamespaces = namespaces.filter(ns => hasWarning(leasesData[ns]));

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'row',
        justifyContent: 'flex-end',
        //justifyContent: 'center', // <-- center horizontally
        alignItems: 'center',
        marginBottom: '6px',
        maxWidth: '1000px',
        marginLeft: 'auto',      // <-- center container
        marginRight: 'auto'      // <-- center container
      }}
    >
      {/* Anomaly Summary: flush left, no margin */}
      <div style={{ marginRight: 0 }}>
        <div style={{
          background: '#222',
          color: deathNamespaces.length > 0 ? '#e00' : (warningNamespaces.length > 0 ? '#ffcc00' : '#888'),
          borderRadius: '4px',
          padding: '8px 24px',
          fontSize: '0.92rem', // <-- smaller font size
          textAlign: 'left'
        }}>
          <strong style={{ marginRight: '12px', fontSize: '0.98rem' }}>Anomaly Summary:</strong>
          {deathNamespaces.length > 0 ? (
            deathNamespaces.length === namespaces.length ? (
              <span>
                Several namespaces are death 💀💀💀💀
              </span>
            ) : deathNamespaces.length === 1 ? (
              <span>
                Death detected in: <span style={{ fontWeight: 600 }}>{deathNamespaces[0]}</span> 💀
              </span>
            ) : (
              <span>
                Multiple namespaces are dead 💀
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
      </div>
      {/* Controls bar: sound, language, restart, date */}
      <div
        style={{
          display: 'flex',
          gap: '6px',
          alignItems: 'center'
        }}
      >
        <span style={{
          color: '#ccc',
          fontSize: '0.9rem',
          marginRight: '16px',
          fontWeight: 500,
          letterSpacing: '0.02em'
        }}>
          {timestamp ? formatFullDate(timestamp) : ''}
        </span>
        <ToggleNoise
          active={noise}
          onToggle={onNoiseToggle}
        />
        <Button
          onClick={onLanguageToggle}
          style={buttonStyle}
          hoverStyle={buttonHoverStyle}
          label={`🌐 ${language === 'en' ? 'SP' : 'EN'}`}
        />
        <Button
          onClick={onRestart}
          style={{ ...buttonStyle, fontSize: '0.85rem', padding: '0.15rem 0.6rem', marginLeft: '12px' }}
          hoverStyle={buttonHoverStyle}
          label="Restart"
        />
      </div>
    </div>
  );
};

export default ChartControls;