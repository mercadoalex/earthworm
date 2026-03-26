import React, { useEffect, useRef } from 'react';
import { config } from './config';
import type { HeartbeatEvent, Alert } from './types/heartbeat';

export type LiveEvent =
  | { kind: 'heartbeat'; data: HeartbeatEvent; receivedAt: number }
  | { kind: 'alert'; data: Alert; receivedAt: number };

interface LiveActivityPanelProps {
  events: LiveEvent[];
  wsStatus: 'connecting' | 'connected' | 'disconnected';
}

const LiveActivityPanel: React.FC<LiveActivityPanelProps> = ({ events, wsStatus }) => {
  const listRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom on new events
  useEffect(() => {
    if (listRef.current) {
      listRef.current.scrollTop = listRef.current.scrollHeight;
    }
  }, [events.length]);

  const statusColor =
    wsStatus === 'connected' ? config.colors.healthy
    : wsStatus === 'connecting' ? config.colors.warning
    : config.colors.death;

  const statusLabel =
    wsStatus === 'connected' ? '● LIVE'
    : wsStatus === 'connecting' ? '◌ Connecting...'
    : '○ Offline';

  return (
    <div style={{
      background: '#1a1d23',
      border: '1px solid #333',
      borderRadius: '8px',
      width: '100%',
      maxWidth: '1920px',
      margin: '16px auto',
      overflow: 'hidden',
    }}>
      {/* Header */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '10px 16px',
        borderBottom: '1px solid #333',
        background: '#22252b',
      }}>
        <span style={{ color: '#ccc', fontWeight: 600, fontSize: '0.9rem' }}>
          Live Activity Feed
        </span>
        <span style={{
          color: statusColor,
          fontSize: '0.8rem',
          fontWeight: 600,
          display: 'flex',
          alignItems: 'center',
          gap: '6px',
        }}>
          {wsStatus === 'connected' && (
            <span style={{
              display: 'inline-block',
              width: '8px',
              height: '8px',
              borderRadius: '50%',
              background: statusColor,
              animation: 'pulse 2s infinite',
            }} />
          )}
          {statusLabel}
        </span>
      </div>

      {/* Event list */}
      <div
        ref={listRef}
        role="log"
        aria-live="polite"
        aria-label="Live heartbeat and alert events"
        style={{
          maxHeight: '200px',
          overflowY: 'auto',
          padding: '8px 0',
        }}
      >
        {events.length === 0 && (
          <div style={{ color: '#666', textAlign: 'center', padding: '20px', fontSize: '0.85rem' }}>
            {wsStatus === 'connected'
              ? 'Waiting for events...'
              : wsStatus === 'connecting'
                ? 'Connecting to server...'
                : 'Start the Go server to see live events'}
          </div>
        )}
        {events.map((evt, idx) => {
          const time = new Date(evt.receivedAt).toLocaleTimeString('en-US', { hour12: false });
          if (evt.kind === 'alert') {
            const a = evt.data;
            const sevColor = a.severity === 'critical' ? config.colors.death : config.colors.warning;
            return (
              <div key={idx} style={{
                display: 'flex',
                alignItems: 'center',
                padding: '4px 16px',
                fontSize: '0.8rem',
                gap: '10px',
                borderLeft: `3px solid ${sevColor}`,
                background: 'rgba(255,0,0,0.05)',
              }}>
                <span style={{ color: '#666', minWidth: '70px', fontFamily: 'monospace' }}>{time}</span>
                <span style={{
                  color: sevColor,
                  fontWeight: 700,
                  fontSize: '0.7rem',
                  padding: '1px 6px',
                  border: `1px solid ${sevColor}`,
                  borderRadius: '3px',
                  textTransform: 'uppercase',
                }}>
                  {a.severity}
                </span>
                <span style={{ color: '#ccc' }}>
                  {a.nodeName} — gap {a.gapSeconds.toFixed(1)}s
                </span>
              </div>
            );
          }
          const hb = evt.data;
          return (
            <div key={idx} style={{
              display: 'flex',
              alignItems: 'center',
              padding: '4px 16px',
              fontSize: '0.8rem',
              gap: '10px',
              borderLeft: `3px solid ${config.colors.healthy}`,
            }}>
              <span style={{ color: '#666', minWidth: '70px', fontFamily: 'monospace' }}>{time}</span>
              <span style={{
                color: config.colors.healthy,
                fontWeight: 600,
                fontSize: '0.7rem',
                padding: '1px 6px',
                border: `1px solid ${config.colors.healthy}`,
                borderRadius: '3px',
              }}>
                HB
              </span>
              <span style={{ color: '#ccc' }}>
                {hb.nodeName} ({hb.namespace}) — {hb.status}
              </span>
            </div>
          );
        })}
      </div>

      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.3; }
        }
      `}</style>
    </div>
  );
};

export default React.memo(LiveActivityPanel);
