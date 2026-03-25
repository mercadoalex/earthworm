import * as fc from 'fast-check';
import { buildNodeSummaries } from './utils/nodeTableUtils';
import { getSparklineColor } from './utils/sparklineUtils';
import { config } from './config';
import type { LeasePoint, LeasesByNamespace } from './types/heartbeat';

// Helper: generate LeasesByNamespace with realistic data
const leasePointsArb = (minLen: number, maxLen: number) =>
  fc.array(fc.integer({ min: 5000, max: 60000 }), { minLength: minLen, maxLength: maxLen }).map((gaps) => {
    let y = 1700000000000;
    const points: LeasePoint[] = [{ x: 0, y }];
    for (let i = 0; i < gaps.length; i++) {
      y += gaps[i];
      points.push({ x: i + 1, y });
    }
    return points;
  });

const leasesByNamespaceArb = fc
  .dictionary(
    fc.stringMatching(/^[a-z][a-z0-9-]{0,8}$/).filter((s) => s.length > 0),
    leasePointsArb(1, 25),
    { minKeys: 1, maxKeys: 5 },
  )
  .map((dict) => dict as LeasesByNamespace);

// Feature: realistic-data-and-visualizations, Property 30: Node summary table completeness
describe('Property 30: Node summary table completeness', () => {
  // **Validates: Requirements 9.1, 9.2**

  it('contains one row per unique node with valid fields', () => {
    fc.assert(
      fc.property(leasesByNamespaceArb, (leasesData) => {
        const summaries = buildNodeSummaries(leasesData);

        // Count expected nodes (namespaces with at least 1 point)
        const expectedNodes = Object.keys(leasesData).filter(
          (ns) => leasesData[ns] && leasesData[ns].length > 0,
        );

        expect(summaries.length).toBe(expectedNodes.length);

        const validStatuses = new Set(['Ready', 'Warning', 'NotReady', 'Unknown']);

        for (const s of summaries) {
          // Non-empty nodeName
          expect(s.nodeName.length).toBeGreaterThan(0);
          // Non-empty namespace
          expect(s.namespace.length).toBeGreaterThan(0);
          // Valid currentStatus
          expect(validStatuses.has(s.currentStatus)).toBe(true);
          // lastHeartbeat > 0
          expect(s.lastHeartbeat).toBeGreaterThan(0);
          // recentIntervals length <= 20
          expect(s.recentIntervals.length).toBeLessThanOrEqual(20);
        }
      }),
      { numRuns: 100 },
    );
  });
});

// Feature: realistic-data-and-visualizations, Property 31: Sparkline threshold coloring
describe('Property 31: Sparkline threshold coloring', () => {
  // **Validates: Requirements 9.3, 9.4**

  it('returns correct color based on interval vs thresholds', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 0, max: 120000 }),
        (intervalMs) => {
          const color = getSparklineColor(intervalMs);

          if (intervalMs > config.criticalGapThreshold) {
            expect(color).toBe(config.colors.critical);
          } else if (intervalMs > config.warningGapThreshold) {
            expect(color).toBe(config.colors.warning);
          } else {
            expect(color).toBe(config.colors.healthy);
          }
        },
      ),
      { numRuns: 100 },
    );
  });
});
