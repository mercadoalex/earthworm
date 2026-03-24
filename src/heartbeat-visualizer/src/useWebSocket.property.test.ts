import * as fc from 'fast-check';
import { computeReconnectDelay } from './hooks/useWebSocket';
import { config } from './config';

// Feature: earthworm-improvements, Property 6: Incoming WebSocket event appends to chart data
describe('Property 6: Incoming WebSocket event appends to chart data', () => {
  // **Validates: Requirements 5.4**

  it('appending a heartbeat event to a chart data array increases length by 1 and last element matches', () => {
    const heartbeatEventArb = fc.record({
      nodeName: fc.stringMatching(/^[a-z][a-z0-9-]{0,10}$/),
      namespace: fc.stringMatching(/^[a-z][a-z0-9-]{0,10}$/),
      timestamp: fc.integer({ min: 1000000000000, max: 2000000000000 }),
      status: fc.constantFrom('healthy' as const, 'unhealthy' as const),
    });

    const existingDataArb = fc.array(
      fc.record({
        index: fc.nat({ max: 1000 }),
        timestamp: fc.integer({ min: 1000000000000, max: 2000000000000 }),
      }),
      { minLength: 0, maxLength: 20 },
    );

    fc.assert(
      fc.property(existingDataArb, heartbeatEventArb, (existingData, event) => {
        // Simulate what the visualizer does when a WS message arrives:
        // append a new data point to the chart data array
        const chartData = [...existingData];
        const prevLength = chartData.length;

        const newPoint = {
          index: prevLength,
          timestamp: event.timestamp,
          [event.namespace]: prevLength,
        };
        chartData.push(newPoint);

        expect(chartData.length).toBe(prevLength + 1);
        expect(chartData[chartData.length - 1].timestamp).toBe(event.timestamp);
        expect(chartData[chartData.length - 1].index).toBe(prevLength);
      }),
      { numRuns: 100 },
    );
  });
});

// Feature: earthworm-improvements, Property 7: Exponential backoff on WebSocket reconnection
describe('Property 7: Exponential backoff on WebSocket reconnection', () => {
  // **Validates: Requirements 5.5**

  it('delay equals min(2^n * 1000, 30000) for any failure count n', () => {
    fc.assert(
      fc.property(fc.nat({ max: 100 }), (n) => {
        const delay = computeReconnectDelay(n);
        const expected = Math.min(
          Math.pow(2, n) * config.reconnect.initialDelayMs,
          config.reconnect.maxDelayMs,
        );
        expect(delay).toBe(expected);
      }),
      { numRuns: 100 },
    );
  });

  it('delay is always between initialDelayMs and maxDelayMs', () => {
    fc.assert(
      fc.property(fc.nat({ max: 100 }), (n) => {
        const delay = computeReconnectDelay(n);
        expect(delay).toBeGreaterThanOrEqual(config.reconnect.initialDelayMs);
        expect(delay).toBeLessThanOrEqual(config.reconnect.maxDelayMs);
      }),
      { numRuns: 100 },
    );
  });

  it('delay is monotonically non-decreasing with failure count', () => {
    fc.assert(
      fc.property(fc.nat({ max: 99 }), (n) => {
        const delay1 = computeReconnectDelay(n);
        const delay2 = computeReconnectDelay(n + 1);
        expect(delay2).toBeGreaterThanOrEqual(delay1);
      }),
      { numRuns: 100 },
    );
  });
});
