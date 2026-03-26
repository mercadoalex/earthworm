import React from 'react';
import { render, screen, act } from '@testing-library/react';
import '@testing-library/jest-dom';
import HeartbeatChart from './HeartbeatChart';
import { ViewProvider } from './contexts/ViewContext';
import type { WebSocketMessage, HeartbeatEvent } from './types/heartbeat';

// Feature: earthworm-improvements, Task 14.3
// Verify E2E test: Visualizer renders data point from mocked WS stream
// **Validates: Requirements 13.3**

// Track the lastMessage callback so we can simulate WS messages
let wsLastMessage: WebSocketMessage | null = null;
let setWsLastMessage: (msg: WebSocketMessage) => void = () => {};

jest.mock('./hooks/useHeartbeatData', () => ({
  useHeartbeatData: () => ({
    leasesData: {
      'test-ns': [
        { x: 0, y: 1000000 },
        { x: 1, y: 1005000 },
        { x: 2, y: 1010000 },
      ],
    },
    currentHeartbeat: 1010000,
    step: 'animate',
    chartData: [
      { timestamp: 1000000, 'test-ns': 1000000 },
      { timestamp: 1005000, 'test-ns': 1005000 },
      { timestamp: 1010000, 'test-ns': 1010000 },
    ],
    namespaces: ['test-ns'],
    currentFileIdx: 0,
  }),
}));

jest.mock('./hooks/useEbpfData', () => ({
  useEbpfData: () => ({
    showEbpf: false,
    toggleEbpf: jest.fn(),
    clearEbpfData: jest.fn(),
    restoreEbpfData: jest.fn(),
    getEbpfMarkers: () => [],
  }),
}));

// Use a stateful mock for useWebSocket so we can trigger message updates
jest.mock('./hooks/useWebSocket', () => ({
  useWebSocket: () => {
    const React = require('react');
    const [msg, setMsg] = React.useState<WebSocketMessage | null>(null);
    // Expose setter so the test can push messages
    setWsLastMessage = (m: WebSocketMessage) => setMsg(m);
    wsLastMessage = msg;
    return {
      status: 'connected' as const,
      lastMessage: msg,
      lastEbpfEvent: null,
      lastCausalChain: null,
      lastPrediction: null,
      sendMessage: jest.fn(),
    };
  },
}));

beforeEach(() => {
  global.fetch = jest.fn(() =>
    Promise.resolve({ ok: true, json: () => Promise.resolve([]) }),
  ) as jest.Mock;

  (global as any).WebSocket = jest.fn(() => ({
    close: jest.fn(),
    send: jest.fn(),
    onopen: null,
    onclose: null,
    onmessage: null,
    onerror: null,
    readyState: 0,
  }));

  (global as any).ResizeObserver = jest.fn(() => ({
    observe: jest.fn(),
    unobserve: jest.fn(),
    disconnect: jest.fn(),
  }));
});

afterEach(() => {
  jest.restoreAllMocks();
});

describe('E2E: Visualizer renders data point from mocked WS stream', () => {
  it('renders a heartbeat event received via WebSocket in the Live Activity Feed', () => {
    render(
      <ViewProvider>
        <HeartbeatChart />
      </ViewProvider>,
    );

    // Initially the live activity feed should show no heartbeat events
    expect(screen.queryByText(/ws-node-42/)).not.toBeInTheDocument();

    // Simulate receiving a heartbeat message via WebSocket
    const heartbeatMsg: WebSocketMessage = {
      type: 'heartbeat',
      payload: {
        nodeName: 'ws-node-42',
        namespace: 'production',
        timestamp: Date.now(),
        status: 'healthy',
      } as HeartbeatEvent,
    };

    act(() => {
      setWsLastMessage(heartbeatMsg);
    });

    // The LiveActivityPanel should now render the heartbeat event
    expect(screen.getByText(/ws-node-42/)).toBeInTheDocument();
    expect(screen.getByText(/production/)).toBeInTheDocument();
  });
});
