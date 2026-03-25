import { useEffect, useRef, useState, useCallback } from 'react';
import { config } from '../config';
import type { WebSocketMessage, EnrichedKernelEvent, CausalChainMessage, PredictionMessage } from '../types/heartbeat';

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected';

export interface UseWebSocketReturn {
  status: ConnectionStatus;
  lastMessage: WebSocketMessage | null;
  lastEbpfEvent: EnrichedKernelEvent | null;
  lastCausalChain: CausalChainMessage['payload'] | null;
  lastPrediction: PredictionMessage['payload'] | null;
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
  const [lastEbpfEvent, setLastEbpfEvent] = useState<EnrichedKernelEvent | null>(null);
  const [lastCausalChain, setLastCausalChain] = useState<CausalChainMessage['payload'] | null>(null);
  const [lastPrediction, setLastPrediction] = useState<PredictionMessage['payload'] | null>(null);
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

        // Route to type-specific state
        switch (parsed.type) {
          case 'ebpf_event':
            setLastEbpfEvent(parsed.payload as EnrichedKernelEvent);
            break;
          case 'causal_chain':
            setLastCausalChain(parsed.payload as CausalChainMessage['payload']);
            break;
          case 'prediction':
            setLastPrediction(parsed.payload as PredictionMessage['payload']);
            break;
        }
      } catch {
        // Skip malformed messages
      }
    };

    ws.onclose = () => {
      if (unmountedRef.current) return;
      setStatus('disconnected');
      if (failureCountRef.current >= config.reconnect.maxRetries) {
        console.warn(`WebSocket: gave up after ${config.reconnect.maxRetries} retries. Start the Go server and refresh to reconnect.`);
        return;
      }
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

  return { status, lastMessage, lastEbpfEvent, lastCausalChain, lastPrediction, sendMessage };
}
