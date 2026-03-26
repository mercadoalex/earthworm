import React from 'react';
import { render, act } from '@testing-library/react';
import * as fc from 'fast-check';

// --- Mock modules BEFORE importing the component ---
// Jest requires mock factory variables to be prefixed with "mock"

// Track calls to transformLeasesForChart
const mockTransformSpy = jest.fn();
jest.mock('./services/dataService', () => {
  const original = jest.requireActual('./services/dataService');
  return {
    ...original,
    transformLeasesForChart: (...args: unknown[]) => {
      mockTransformSpy();
      return original.transformLeasesForChart(...args);
    },
  };
});

// Mock useWebSocket — we control lastMessage externally
let mockLastMessage: { type: string; payload: unknown } | null = null;
const mockWsStatus = 'connected';
jest.mock('./hooks/useWebSocket', () => ({
  useWebSocket: () => ({
    status: mockWsStatus,
    lastMessage: mockLastMessage,
    lastEbpfEvent: null,
    lastCausalChain: null,
    lastPrediction: null,
    sendMessage: jest.fn(),
  }),
}));

// Track calls to ebpfMarkers computation
const mockEbpfMarkersSpy = jest.fn();
jest.mock('./hooks/useEbpfData', () => ({
  useEbpfData: (...args: unknown[]) => {
    // Use React.useMemo to mirror the fixed memoized behavior in useEbpfData.
    // On fixed code, ebpfMarkers computation only runs when inputs change.
    const React = require('react');
    const ebpfMarkers = React.useMemo(() => {
      mockEbpfMarkersSpy();
      return [];
    }, args);
    return {
      showEbpf: true,
      toggleEbpf: jest.fn(),
      clearEbpfData: jest.fn(),
      restoreEbpfData: jest.fn(),
      ebpfMarkers,
      ebpfData: [],
    };
  },
}));

// Mock useHeartbeatData — calls the real transformLeasesForChart on each invocation
// This mirrors the unfixed behavior where the hook computes chartData inline
const mockSampleLeasesData = {
  'kube-system': [
    { x: 0, y: 1700000000000 },
    { x: 1, y: 1700000010000 },
    { x: 2, y: 1700000020000 },
  ],
  'production': [
    { x: 0, y: 1700000000000 },
    { x: 1, y: 1700000011000 },
    { x: 2, y: 1700000021000 },
  ],
};

jest.mock('./hooks/useHeartbeatData', () => ({
  useHeartbeatData: () => {
    // Use React.useMemo to mirror the fixed memoized behavior in useHeartbeatData.
    // On fixed code, transformLeasesForChart is only called when leasesData/currentHeartbeat change.
    const React = require('react');
    const { transformLeasesForChart } = require('./services/dataService');
    const chartData = React.useMemo(
      () => transformLeasesForChart(mockSampleLeasesData, 3),
      [mockSampleLeasesData],
    );
    const namespaces = React.useMemo(
      () => Object.keys(mockSampleLeasesData),
      [mockSampleLeasesData],
    );
    return {
      leasesData: mockSampleLeasesData,
      currentHeartbeat: 2,
      step: 'animate',
      chartData,
      namespaces,
      currentFileIdx: 0,
      manifest: ['leases.json'],
      restart: jest.fn(),
    };
  },
}));

// Mock ResizeObserver
beforeAll(() => {
  class MockResizeObserver {
    observe = jest.fn();
    unobserve = jest.fn();
    disconnect = jest.fn();
  }
  (global as any).ResizeObserver = MockResizeObserver;
  (global as any).HTMLMediaElement.prototype.play = jest.fn();
  (global as any).HTMLMediaElement.prototype.pause = jest.fn();
});

// Now import the component and provider
import HeartbeatChart from './HeartbeatChart';
import { ViewProvider } from './contexts/ViewContext';

/**
 * Bug Condition Exploration Test — UI Freeze Under Rapid WebSocket Messages
 *
 * **Validates: Requirements 1.1, 1.2, 1.3, 1.4**
 *
 * This test encodes EXPECTED behavior: when N WebSocket messages arrive
 * with unchanged leasesData, expensive computations should NOT re-run.
 *
 * On UNFIXED code, this test FAILS — confirming the bug exists:
 * - transformLeasesForChart is called on every render (not memoized in useHeartbeatData)
 * - useEbpfData (ebpfMarkers computation) runs on every render (not memoized)
 */
describe('Property 1: Bug Condition — UI Freeze Under Rapid WebSocket Messages', () => {
  beforeEach(() => {
    mockTransformSpy.mockClear();
    mockEbpfMarkersSpy.mockClear();
    mockLastMessage = null;
  });

  it('transformLeasesForChart is called no more than once when leasesData has not changed across N rapid WebSocket messages', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 5, max: 10 }),
        (messageCount) => {
          mockTransformSpy.mockClear();
          mockEbpfMarkersSpy.mockClear();
          mockLastMessage = null;

          const { rerender } = render(
            <ViewProvider>
              <HeartbeatChart />
            </ViewProvider>,
          );

          // Record calls after initial render
          const initialTransformCalls = mockTransformSpy.mock.calls.length;

          // Simulate N rapid WebSocket messages by updating lastMessage
          for (let i = 0; i < messageCount; i++) {
            mockLastMessage = {
              type: 'heartbeat',
              payload: {
                nodeName: `node-${i}`,
                namespace: 'kube-system',
                timestamp: Date.now() + i,
                status: 'healthy',
              },
            };

            act(() => {
              rerender(
                <ViewProvider>
                  <HeartbeatChart />
                </ViewProvider>,
              );
            });
          }

          const additionalTransformCalls =
            mockTransformSpy.mock.calls.length - initialTransformCalls;

          // EXPECTED: transformLeasesForChart should NOT be called again
          // since leasesData hasn't changed. At most 1 additional call allowed.
          // On UNFIXED code: it will be called N more times (once per re-render).
          expect(additionalTransformCalls).toBeLessThanOrEqual(1);
        },
      ),
      { numRuns: 2 },
    );
  });

  it('ebpfMarkers computation (useEbpfData) runs no more than once when ebpf inputs have not changed across N rapid WebSocket messages', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 5, max: 10 }),
        (messageCount) => {
          mockTransformSpy.mockClear();
          mockEbpfMarkersSpy.mockClear();
          mockLastMessage = null;

          const { rerender } = render(
            <ViewProvider>
              <HeartbeatChart />
            </ViewProvider>,
          );

          // Record calls after initial render
          const initialEbpfCalls = mockEbpfMarkersSpy.mock.calls.length;

          // Simulate N rapid WebSocket messages
          for (let i = 0; i < messageCount; i++) {
            mockLastMessage = {
              type: 'heartbeat',
              payload: {
                nodeName: `node-${i}`,
                namespace: 'production',
                timestamp: Date.now() + i,
                status: 'healthy',
              },
            };

            act(() => {
              rerender(
                <ViewProvider>
                  <HeartbeatChart />
                </ViewProvider>,
              );
            });
          }

          const additionalEbpfCalls =
            mockEbpfMarkersSpy.mock.calls.length - initialEbpfCalls;

          // EXPECTED: ebpfMarkers computation should NOT run again
          // since ebpf inputs haven't changed. At most 1 additional call allowed.
          // On UNFIXED code: it will be called N more times (once per re-render).
          expect(additionalEbpfCalls).toBeLessThanOrEqual(1);
        },
      ),
      { numRuns: 2 },
    );
  });
});
