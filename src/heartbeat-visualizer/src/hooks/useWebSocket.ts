import { useEffect, useRef, useState, useCallback } from 'react';
import { config } from '../config';
import type { WebSocketMessage } from '../types/heartbeat';

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected';

export interface UseWebSocketReturn {
  status: ConnectionStatus;
  lastMessage: WebSocketMessage | null;
  sendMessage: (data: string) => void;
}

/**
 * Computes the reconnection delay with exponential backoff.
 * delay = min(2^n * initialDelayMs, maxDelayMs)
 */
export function computeReconnectDelay(failureCount: number): number {
  const { initialDelayMs, maxDelayMs } = config.reconnect;
  return Math.min(Math.pow(2, failureCount) * initialDelayMs, maxDelayMs);
}

export function useWebSocket(url: string = config.wsEndpoint): UseWebSocketReturn {
  const [status, setStatus] = useState<ConnectionStatus>('disconnected');
  const [lastMessage, setLastMessage] = useState<WebSocketMessage | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const failureCountRef = useRef(0);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const unmountedRef = useRef(false);

  const connect = useCallback(() => {
    if (unmountedRef.current) return;

    setStatus('connecting');
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      if (unmountedRef.current) return;
      failureCountRef.current = 0;
      setStatus('connected');
    };

    ws.onmessage = (event) => {
      if (unmountedRef.current) return;
      try {
        const parsed: WebSocketMessage = JSON.parse(event.data);
        setLastMessage(parsed);
      } catch {
        // Skip malformed messages
      }
    };

    ws.onclose = () => {
      if (unmountedRef.current) return;
      setStatus('disconnected');
      const delay = computeReconnectDelay(failureCountRef.current);
      failureCountRef.current++;
      reconnectTimerRef.current = setTimeout(connect, delay);
    };

    ws.onerror = () => {
      // onclose will fire after onerror, triggering reconnect
    };
  }, [url]);

  useEffect(() => {
    unmountedRef.current = false;
    connect();

    return () => {
      unmountedRef.current = true;
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [connect]);

  const sendMessage = useCallback((data: string) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(data);
    }
  }, []);

  return { status, lastMessage, sendMessage };
}
