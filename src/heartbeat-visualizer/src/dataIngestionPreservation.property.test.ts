import * as fc from 'fast-check';
import { transformLeasesForChart } from './services/dataService';
import type { LeasePoint, LeasesByNamespace, HeartbeatEvent, Alert } from './types/heartbeat';
import type { LiveEvent } from './LiveActivityPanel';

/**
 * Property 2: Preservation — Functional Equivalence of Chart Data and Live Events
 *
 * **Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7**
 *
 * These tests verify baseline behavior that must be preserved after the fix.
 * They should PASS on unfixed code.
 */

// --- Generators ---

const NS_POOL = ['kube-system', 'production', 'staging', 'monitoring', 'default'];

/**
 * Generate a valid LeasesByNamespace with 1-5 namespaces,
 * each having numPoints lease points with increasing timestamps.
 */
const leasesByNamespaceArb = (
  minPts: number,
  maxPts: number,
): fc.Arbitrary<{ data: LeasesByNamespace; maxPoints: number }> =>
  fc
    .tuple(
      fc.integer({ min: minPts, max: maxPts }),
      fc.integer({ min: 1, max: 5 }),
    )
    .chain(([numPoints, numNs]) =>
      fc
        .array(fc.nat({ max: 50000 }), {
          minLength: numPoints * numNs,
          maxLength: numPoints * numNs,
        })
        .map((gaps) => {
          const namespaces = NS_POOL.slice(0, numNs);
          const data: LeasesByNamespace = {};
          let gapIdx = 0;

          for (const ns of namespaces) {
            let y = 1_700_000_000_000;
            const points: LeasePoint[] = [];
            for (let i = 0; i < numPoints; i++) {
              y += (gaps[gapIdx++] ?? 1000) + 1;
              points.push({ x: i, y });
            }
            data[ns] = points;
          }
          return { data, maxPoints: numPoints };
        }),
    );


// --- Property Tests for transformLeasesForChart ---

describe('Property 2a: transformLeasesForChart output correctness', () => {
  // **Validates: Requirements 3.1, 3.3, 3.4**

  it('output has correct length equal to maxPoints', () => {
    fc.assert(
      fc.property(leasesByNamespaceArb(1, 30), ({ data, maxPoints }) => {
        const result = transformLeasesForChart(data, maxPoints);
        expect(result).toHaveLength(maxPoints);
      }),
      { numRuns: 3 },
    );
  });

  it('each point has correct index matching its position', () => {
    fc.assert(
      fc.property(leasesByNamespaceArb(1, 30), ({ data, maxPoints }) => {
        const result = transformLeasesForChart(data, maxPoints);
        result.forEach((point, i) => {
          expect(point.index).toBe(i);
        });
      }),
      { numRuns: 3 },
    );
  });

  it('each point has keys for all namespaces that have data at that index', () => {
    fc.assert(
      fc.property(leasesByNamespaceArb(1, 30), ({ data, maxPoints }) => {
        const namespaces = Object.keys(data);
        const result = transformLeasesForChart(data, maxPoints);

        result.forEach((point, i) => {
          namespaces.forEach((ns) => {
            if (data[ns][i]) {
              expect(point).toHaveProperty(ns, data[ns][i].x);
            }
          });
        });
      }),
      { numRuns: 3 },
    );
  });

  it('timestamp on each point comes from one of the namespace lease points at that index', () => {
    fc.assert(
      fc.property(leasesByNamespaceArb(1, 30), ({ data, maxPoints }) => {
        const namespaces = Object.keys(data);
        const result = transformLeasesForChart(data, maxPoints);

        result.forEach((point, i) => {
          if (point.timestamp !== undefined) {
            const possibleTimestamps = namespaces
              .filter((ns) => data[ns][i] && data[ns][i].y)
              .map((ns) => data[ns][i].y);
            expect(possibleTimestamps).toContain(point.timestamp);
          }
        });
      }),
      { numRuns: 3 },
    );
  });
});


// --- Property Tests for liveEvents 200-cap behavior ---

/** Simulate the liveEvents accumulation logic from HeartbeatChart */
function simulateLiveEvents(
  messages: Array<{ type: 'heartbeat' | 'alert'; payload: HeartbeatEvent | Alert }>,
): LiveEvent[] {
  let liveEvents: LiveEvent[] = [];
  for (const msg of messages) {
    const now = Date.now();
    if (msg.type === 'alert') {
      liveEvents = [
        ...liveEvents.slice(-199),
        { kind: 'alert', data: msg.payload as Alert, receivedAt: now },
      ];
    } else if (msg.type === 'heartbeat') {
      liveEvents = [
        ...liveEvents.slice(-199),
        { kind: 'heartbeat', data: msg.payload as HeartbeatEvent, receivedAt: now },
      ];
    }
  }
  return liveEvents;
}

/** Generate a heartbeat WebSocket message */
const heartbeatMessageArb: fc.Arbitrary<{
  type: 'heartbeat';
  payload: HeartbeatEvent;
}> = fc
  .tuple(
    fc.integer({ min: 0, max: 99 }),
    fc.constantFrom(...NS_POOL),
    fc.integer({ min: 1_700_000_000_000, max: 1_700_001_000_000 }),
    fc.constantFrom('healthy' as const, 'unhealthy' as const),
  )
  .map(([nodeId, ns, ts, status]) => ({
    type: 'heartbeat' as const,
    payload: {
      nodeName: `node-${nodeId}`,
      namespace: ns,
      timestamp: ts,
      status,
    },
  }));

/** Generate an alert WebSocket message */
const alertMessageArb: fc.Arbitrary<{ type: 'alert'; payload: Alert }> = fc
  .tuple(
    fc.integer({ min: 0, max: 99 }),
    fc.constantFrom(...NS_POOL),
    fc.integer({ min: 10, max: 120 }),
    fc.constantFrom('warning' as const, 'critical' as const),
    fc.integer({ min: 1_700_000_000_000, max: 1_700_001_000_000 }),
  )
  .map(([nodeId, ns, gap, severity, ts]) => ({
    type: 'alert' as const,
    payload: {
      nodeName: `node-${nodeId}`,
      namespace: ns,
      gapSeconds: gap,
      severity,
      timestamp: ts,
    },
  }));

/** Generate a mixed sequence of heartbeat and alert messages */
const wsMessageSequenceArb = (maxLen: number) =>
  fc.array(fc.oneof(heartbeatMessageArb, alertMessageArb), {
    minLength: 0,
    maxLength: maxLen,
  });

describe('Property 2b: liveEvents 200-cap, insertion order, and most-recent preservation', () => {
  // **Validates: Requirements 3.1, 3.2, 3.7**

  it('liveEvents array length is always ≤ 200 for any sequence of ≤ 300 messages', () => {
    fc.assert(
      fc.property(wsMessageSequenceArb(300), (messages) => {
        const result = simulateLiveEvents(messages);
        expect(result.length).toBeLessThanOrEqual(200);
      }),
      { numRuns: 3 },
    );
  });

  it('liveEvents preserves insertion order (kinds match message order for the retained tail)', () => {
    fc.assert(
      fc.property(wsMessageSequenceArb(300), (messages) => {
        const result = simulateLiveEvents(messages);
        const expectedCount = Math.min(messages.length, 200);
        expect(result.length).toBe(expectedCount);

        const tail = messages.slice(-expectedCount);
        for (let i = 0; i < result.length; i++) {
          const msg = tail[i];
          if (msg.type === 'heartbeat') {
            expect(result[i].kind).toBe('heartbeat');
          } else {
            expect(result[i].kind).toBe('alert');
          }
        }
      }),
      { numRuns: 3 },
    );
  });

  it('liveEvents contains the most recent events with correct payload data', () => {
    // Use minLength > 200 directly to avoid slow .filter()
    const overflowSequenceArb = fc.array(
      fc.oneof(heartbeatMessageArb, alertMessageArb),
      { minLength: 210, maxLength: 300 },
    );

    fc.assert(
      fc.property(overflowSequenceArb, (messages) => {
        const result = simulateLiveEvents(messages);
        expect(result.length).toBe(200);

        const last200 = messages.slice(-200);
        for (let i = 0; i < 200; i++) {
          const msg = last200[i];
          expect(result[i].kind).toBe(
            msg.type === 'heartbeat' ? 'heartbeat' : 'alert',
          );
          if (msg.type === 'heartbeat') {
            const hb = result[i].data as HeartbeatEvent;
            expect(hb.nodeName).toBe(msg.payload.nodeName);
            expect(hb.namespace).toBe(msg.payload.namespace);
          } else {
            const alert = result[i].data as Alert;
            expect(alert.nodeName).toBe(msg.payload.nodeName);
            expect(alert.severity).toBe((msg.payload as Alert).severity);
          }
        }
      }),
      { numRuns: 3 },
    );
  });
});
