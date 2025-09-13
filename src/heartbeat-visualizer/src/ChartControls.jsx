import React from 'react';

// Subtle button style for all controls
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

// Subtle toggle switch style
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

const toggleBarStyle = (active) => ({
  width: '28px',
  height: '14px',
  borderRadius: '7px',
  background: active ? 'rgb(7, 144, 73)' : '#444',
  position: 'relative',
  transition: 'background 0.2s'
});

const toggleKnobStyle = (active) => ({
  position: 'absolute',
  top: '2px',
  left: active ? '16px' : '2px',
  width: '10px',
  height: '10px',
  borderRadius: '50%',
  background: '#ccc',
  transition: 'left 0.2s'
});

// Helper to format date as "Monday, April 1st, 2025"
function formatFullDate(ms) {
  if (!ms) return '';
  const date = new Date(ms);
  const dayName = date.toLocaleDateString('en-US', { weekday: 'long' });
  const monthName = date.toLocaleDateString('en-US', { month: 'long' });
  const day = date.getDate();
  // Get ordinal suffix
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

// ChartControls now has sound, language, restart button, and date label
const ChartControls = ({ noise, onNoiseToggle, language, onLanguageToggle, onRestart, timestamp }) => (
  <div
    style={{
      display: 'flex',
      gap: '6px',
      justifyContent: 'flex-end',
      marginBottom: '6px',
      alignItems: 'center'
    }}
  >
    {/* Date label */}
    <span style={{
      color: '#ccc',
      fontSize: '0.9rem',
      marginRight: '16px',
      fontWeight: 500,
      letterSpacing: '0.02em'
    }}>
      {formatFullDate(timestamp)}
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
);

// Toggle switch for noise/sound
function ToggleNoise({ active, onToggle }) {
  const [hover, setHover] = React.useState(false);
  return (
    <div
      style={{
        ...toggleContainerStyle,
        ...(hover ? { background: '#333' } : {})
      }}
      onClick={onToggle}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      title="Toggle sound"
    >
      <span style={{ fontSize: '1rem' }}>📢</span>
      <div style={toggleBarStyle(active)}>
        <div style={toggleKnobStyle(active)} />
      </div>
      <span style={{ marginLeft: '4px' }}>{active ? 'On' : 'Off'}</span>
    </div>
  );
}

// Small, subtle button with hover effect
function Button({ onClick, style, hoverStyle, label }) {
  const [hover, setHover] = React.useState(false);
  return (
    <button
      onClick={onClick}
      style={hover ? { ...style, ...hoverStyle } : style}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
    >
      {label}
    </button>
  );
}

export default ChartControls;