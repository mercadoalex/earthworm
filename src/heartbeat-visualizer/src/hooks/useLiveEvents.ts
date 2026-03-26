import { useEffect, useState, useRef, useCallback } from 'react';
import type { Alert, HeartbeatEvent, WebSocketMessage } from '../types/heartbeat';
import type { LiveEvent } from '../LiveActivityPanel';

export interface UseLiveEventsReturn {
  liveEvents: LiveEvent[];
  alerts: Alert[];
  dismissAlert: (index: number) => void;
}

/**
 * Hook that processes incoming WebSocket messages into liveEvents and alerts,
 * with batching/throttling to avoid per-message re-renders.
 *
 * Messages are accumulated in a ref buffer and flushed via requestAnimationFrame,
 * so rapid messages are batched into a single state update.
 */
export function useLiveEvents(lastMessage: WebSocketMessage | null): UseLiveEventsReturn {
  const [liveEvents, setLiveEvents] = useState<LiveEvent[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);

  // Buffer for batching incoming messages
  const messageBufferRef = useRef<WebSocketMessage[]>([]);
  const rafIdRef = useRef<number | null>(null);

  // Track the last processed message to detect new ones
  const lastProcessedRef = useRef<WebSocketMessage | null>(null);

  const flushBuffer = useCallback(() => {
    rafIdRef.current = null;
    const buffered = messageBufferRef.current;
    if (buffered.length === 0) return;
    messageBufferRef.current = [];

    const now = Date.now();
    const newLiveEvents: LiveEvent[] = [];
    const newAlerts: Alert[] = [];

    for (const msg of buffered) {
      if (msg.type === 'alert') {
        const alert = msg.payload as Alert;
        newAlerts.push(alert);
        newLiveEvents.push({ kind: 'alert', data: alert, receivedAt: now });
      } else if (msg.type === 'heartbeat') {
        const hb = msg.payload as HeartbeatEvent;
        newLiveEvents.push({ kind: 'heartbeat', data: hb, receivedAt: now });
      }
    }

    if (newAlerts.length > 0) {
      setAlerts((prev) => [...prev, ...newAlerts]);
    }
    if (newLiveEvents.length > 0) {
      setLiveEvents((prev) => [...prev, ...newLiveEvents].slice(-200));
    }
  }, []);

  useEffect(() => {
    if (!lastMessage || lastMessage === lastProcessedRef.current) return;
    lastProcessedRef.current = lastMessage;

    messageBufferRef.current.push(lastMessage);

    // Schedule a flush on the next animation frame if not already scheduled
    if (rafIdRef.current === null) {
      rafIdRef.current = requestAnimationFrame(flushBuffer);
    }
  }, [lastMessage, flushBuffer]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (rafIdRef.current !== null) {
        cancelAnimationFrame(rafIdRef.current);
      }
    };
  }, []);

  const dismissAlert = useCallback((index: number) => {
    setAlerts((prev) => prev.filter((_, i) => i !== index));
  }, []);

  return { liveEvents, alerts, dismissAlert };
}
