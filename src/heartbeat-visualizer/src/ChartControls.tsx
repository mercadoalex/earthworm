import React, { useState } from 'react';
import logo from './logo.svg';
import { config } from './config';
import { hasWarning, hasDeath, getAnomalies, formatFullDate } from './utils/chartUtils';
import type { ChartControlsProps } from './types/heartbeat';

// --- Button styles ---
const buttonStyle: React.CSSProperties = {
  fontSize: '0.65rem',
  padding: '0.15rem 0.4rem',
  background: '#222',
  color: '#ccc',
  borderWidth: '1px',
  borderStyle: 'solid',
  borderColor: '#222',
  borderRadius: '2px',
  cursor: 'pointer',
  transition: 'background 0.2s, border-color 0.2s',
};

const buttonHoverStyle: React.CSSProperties = {
  background: '#333',
  borderColor: '#22ff99',
};

const toggleContainerStyle: React.CSSProperties = {
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
  transition: 'background 0.2s',
};

// --- ToggleNoise component ---
interface ToggleNoiseProps {
  active: boolean;
  onToggle: () => void;
}

const ToggleNoise: React.FC<ToggleNoiseProps> = ({ active, onToggle }) => (
  <button
    style={{
      ...toggleContainerStyle,
      background: active ? '#333' : '#222',
      position: 'relative',
    }}
    onClick={onToggle}
    aria-label={active ? 'Disable sound' : 'Enable sound'}
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
      transition: 'background 0.2s',
    }}>
      <span style={{
        position: 'absolute',
        top: '2px',
        left: active ? '16px' : '2px',
        width: '10px',
        height: '10px',
        borderRadius: '50%',
        background: '#ccc',
        transition: 'left 0.2s',
      }} />
    </span>
    {active ? '🔊 Sound On' : '🔇 Sound Off'}
  </button>
);

// --- Button component ---
interface ButtonProps {
  onClick: () => void;
  style: React.CSSProperties;
  hoverStyle: React.CSSProperties;
  label: string;
  ariaLabel?: string;
}

const Button: React.FC<ButtonProps> = ({ onClick, style: btnStyle, hoverStyle: btnHoverStyle, label, ariaLabel }) => {
  const [hover, setHover] = useState(false);
  return (
    <button
      style={hover ? { ...btnStyle, ...btnHoverStyle } : btnStyle}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      onClick={onClick}
      aria-label={ariaLabel || label}
    >
      {label}
    </button>
  );
};

const ChartControls: React.FC<ChartControlsProps> = ({
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
  restoreEbpfData,
}) => {
  const namespaces = Object.keys(leasesData || {});
  const deathNamespaces = namespaces.filter((ns) => leasesData && hasDeath(leasesData[ns]));
  const warningNamespaces = namespaces.filter((ns) => leasesData && hasWarning(leasesData[ns]));

  const [ebpfHover, setEbpfHover] = useState(false);
  const [showAnomalyTooltip, setShowAnomalyTooltip] = useState(false);

  const anomalies = getAnomalies(leasesData);

  const handleEbpfClick = () => {
    if (showEbpf) {
      clearEbpfData?.();
    } else {
      restoreEbpfData?.();
    }
    onEbpfCorrelate();
  };

  return (
    <nav
      aria-label="Chart controls"
      style={{
        display: 'flex',
        flexDirection: 'row',
        justifyContent: 'flex-end',
        alignItems: 'center',
        marginBottom: '6px',
        maxWidth: '100%',
        width: '100%',
        marginLeft: 'auto',
        marginRight: 'auto',
        position: 'relative',
        flexWrap: 'wrap',
        gap: '6px',
      }}
    >
      {/* Anomaly Summary */}
      <div style={{ marginRight: 0, position: 'relative' }}>
        <div style={{
          background: '#222',
          color: deathNamespaces.length > 0
            ? config.colors.death
            : warningNamespaces.length > 0
              ? config.colors.warning
              : '#888',
          borderRadius: '4px',
          padding: '8px 24px',
          fontSize: '0.92rem',
          textAlign: 'left',
          cursor: 'pointer',
        }}>
          <strong
            style={{ marginRight: '12px', fontSize: '0.98rem', cursor: 'pointer' }}
            onClick={() => setShowAnomalyTooltip((v) => !v)}
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
              Warning detected in: {warningNamespaces.map((ns) => (
                <span key={ns} style={{ fontWeight: 600, marginRight: 8 }}>{ns} ⚠️</span>
              ))}
            </span>
          ) : (
            <span style={{ color: '#aaa', fontSize: '0.92rem', marginLeft: '12px' }}>
              NO anomalies detected.
            </span>
          )}
        </div>
        {showAnomalyTooltip && (
          <div style={{
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
            fontSize: '0.95rem',
          }}>
            <div style={{
              color: '#ccc',
              fontWeight: 600,
              fontSize: '0.98rem',
              marginBottom: '8px',
              textAlign: 'center',
            }}>
              Anomaly Details
            </div>
            {anomalies.length === 0 ? (
              <div style={{ color: '#aaa', textAlign: 'center' }}>No anomalies found.</div>
            ) : (
              <ul style={{ paddingLeft: 0, margin: 0 }}>
                {anomalies.map((a, idx) => (
                  <li key={idx} style={{ marginBottom: 6, listStyle: 'none' }}>
                    <span style={{ color: config.colors.warning, fontWeight: 600 }}>{a.namespace}</span>{' '}
                    gap: <span style={{ color: config.colors.death }}>{Math.round(a.gap / 1000)}s</span>{' '}
                    at <span style={{ color: '#09f' }}>{new Date(a.to).toLocaleString()}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </div>
      {/* Controls bar */}
      <div style={{ display: 'flex', gap: '6px', alignItems: 'center' }}>
        <span style={{
          color: '#ccc',
          fontSize: '0.9rem',
          marginRight: '16px',
          fontWeight: 500,
          letterSpacing: '0.02em',
        }}>
          {timestamp ? formatFullDate(timestamp) : ''}
        </span>
        <ToggleNoise active={noise} onToggle={onNoiseToggle} />
        <Button
          onClick={onLanguageToggle}
          style={buttonStyle}
          hoverStyle={buttonHoverStyle}
          label={`🌐 ${language === 'en' ? 'SP' : 'EN'}`}
          ariaLabel={language === 'en' ? 'Switch to Spanish' : 'Switch to English'}
        />
        <Button
          onClick={onRestart}
          style={{ ...buttonStyle, fontSize: '0.85rem', padding: '0.15rem 0.6rem', marginLeft: '12px' }}
          hoverStyle={buttonHoverStyle}
          label="Restart"
          ariaLabel="Restart heartbeat animation"
        />
        <button
          onClick={handleEbpfClick}
          onMouseEnter={() => setEbpfHover(true)}
          onMouseLeave={() => setEbpfHover(false)}
          aria-label={showEbpf ? 'Hide eBPF Correlation' : 'Show eBPF Correlation'}
          title={showEbpf ? 'Hide eBPF Correlation' : 'Show eBPF Correlation'}
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
            marginLeft: '8px',
          }}
        >
          <img
            src={logo}
            alt="eBPF"
            style={{
              width: 28,
              height: 28,
              filter: ebpfHover ? 'grayscale(1) brightness(0.7)' : 'none',
              transition: 'filter 0.2s',
            }}
          />
        </button>
      </div>
    </nav>
  );
};

export default ChartControls;
