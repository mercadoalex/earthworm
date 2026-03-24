import * as fc from 'fast-check';
import { hasWarning, hasDeath, getAnomalies, getSegmentColor } from './utils/chartUtils';
import { config } from './config';
import type { LeasePoint, LeasesByNamespace } from './types/heartbeat';

// Helper: generate an array of LeasePoints with controlled gaps
const leasePointsArb = (minLen: number, maxLen: number) =>
  fc.array(fc.nat({ max: 100000 }), { minLength: minLen, maxLength: maxLen }).map((gaps) => {
    let y = 1000000;
    const points: LeasePoint[] = [{ x: 0, y }];
    for (let i = 0; i < gaps.length; i++) {
      y += gaps[i];
      points.push({ x: i + 1, y });
    }
    return points;
  });

// Feature: earthworm-improvements, Property 16: Gap classification (hasWarning and hasDeath)
describe('Property 16: Gap classification (hasWarning and hasDeath)', () => {
  // **Validates: Requirements 12.1, 12.2**

  it('hasWarning returns true iff there exists a gap in (10000, 40000)', () => {
    fc.assert(
      fc.property(leasePointsArb(1, 20), (points) => {
        const result = hasWarning(points);
        let expected = false;
        for (let i = 1; i < points.length; i++) {
          const gap = points[i].y - points[i - 1].y;
          if (gap > config.warningGapThreshold && gap < config.criticalGapThreshold) {
            expected = true;
            break;
          }
        }
        expect(result).toBe(expected);
      }),
      { numRuns: 100 },
    );
  });

  it('hasDeath returns true iff there exists a gap > 40000', () => {
    fc.assert(
      fc.property(leasePointsArb(1, 20), (points) => {
        const result = hasDeath(points);
        let expected = false;
        for (let i = 1; i < points.length; i++) {
          const gap = points[i].y - points[i - 1].y;
          if (gap > config.criticalGapThreshold) {
            expected = true;
            break;
          }
        }
        expect(result).toBe(expected);
      }),
      { numRuns: 100 },
    );
  });

  it('hasWarning returns false for arrays with fewer than 2 points', () => {
    fc.assert(
      fc.property(
        fc.oneof(fc.constant([]), fc.constant([{ x: 0, y: 5000 }])),
        (points) => {
          expect(hasWarning(points)).toBe(false);
        },
      ),
      { numRuns: 100 },
    );
  });

  it('hasDeath returns false for arrays with fewer than 2 points', () => {
    fc.assert(
      fc.property(
        fc.oneof(fc.constant([]), fc.constant([{ x: 0, y: 5000 }])),
        (points) => {
          expect(hasDeath(points)).toBe(false);
        },
      ),
      { numRuns: 100 },
    );
  });
});

// Feature: earthworm-improvements, Property 17: Anomaly detection utility (getAnomalies)
describe('Property 17: Anomaly detection utility (getAnomalies)', () => {
  // **Validates: Requirements 12.3**

  const leasesByNamespaceArb = fc
    .dictionary(
      fc.stringMatching(/^[a-z][a-z0-9-]{0,8}$/).filter((s) => s.length > 0),
      leasePointsArb(1, 10),
      { minKeys: 1, maxKeys: 4 },
    )
    .map((dict) => dict as LeasesByNamespace);

  it('returns one anomaly per gap in (10000, 40000) with correct namespace and gap', () => {
    fc.assert(
      fc.property(leasesByNamespaceArb, (leasesData) => {
        const anomalies = getAnomalies(leasesData);

        // Compute expected anomalies manually
        const expected: Array<{ namespace: string; gap: number; index: number }> = [];
        for (const [ns, arr] of Object.entries(leasesData)) {
          if (!arr || arr.length < 2) continue;
          for (let i = 1; i < arr.length; i++) {
            const gap = arr[i].y - arr[i - 1].y;
            if (gap > config.warningGapThreshold && gap < config.criticalGapThreshold) {
              expected.push({ namespace: ns, gap, index: i });
            }
          }
        }

        expect(anomalies.length).toBe(expected.length);
        for (let i = 0; i < expected.length; i++) {
          expect(anomalies[i].namespace).toBe(expected[i].namespace);
          expect(anomalies[i].gap).toBe(expected[i].gap);
          expect(anomalies[i].index).toBe(expected[i].index);
        }
      }),
      { numRuns: 100 },
    );
  });

  it('returns empty array for null input', () => {
    expect(getAnomalies(null)).toEqual([]);
  });
});

// Feature: earthworm-improvements, Property 18: Segment color mapping
describe('Property 18: Segment color mapping', () => {
  // **Validates: Requirements 12.4**

  it('returns correct color based on gap between consecutive timestamps', () => {
    // Generate a base timestamp and a gap, then build a 2-point array
    const gapArb = fc.nat({ max: 120000 }); // up to 120 seconds in ms

    fc.assert(
      fc.property(fc.nat({ max: 1000000000 }), gapArb, (baseTs, gapMs) => {
        const points: LeasePoint[] = [
          { x: 0, y: baseTs },
          { x: 1, y: baseTs + gapMs },
        ];

        const color = getSegmentColor(points, 1);
        const intervalSec = gapMs / 1000;

        if (intervalSec <= config.criticalGapThreshold / 1000) {
          // Normal or warning — getSegmentColor returns healthy for anything <= criticalGapThreshold
          expect(color).toBe(config.colors.healthy);
        } else {
          // Above critical threshold with only 2 points (criticalCount = 1), returns warning
          expect(color).toBe(config.colors.warning);
        }
      }),
      { numRuns: 100 },
    );
  });

  it('returns healthy color for index 0', () => {
    fc.assert(
      fc.property(fc.nat({ max: 1000000 }), (ts) => {
        const points: LeasePoint[] = [{ x: 0, y: ts }];
        expect(getSegmentColor(points, 0)).toBe(config.colors.healthy);
      }),
      { numRuns: 100 },
    );
  });

  it('returns critical color when multiple consecutive intervals exceed critical threshold', () => {
    fc.assert(
      fc.property(
        fc.nat({ max: 1000000 }),
        fc.integer({ min: 41000, max: 120000 }),
        fc.integer({ min: 41000, max: 120000 }),
        (baseTs, gap1, gap2) => {
          // 3 points with 2 consecutive critical gaps
          const points: LeasePoint[] = [
            { x: 0, y: baseTs },
            { x: 1, y: baseTs + gap1 },
            { x: 2, y: baseTs + gap1 + gap2 },
          ];
          const color = getSegmentColor(points, 2);
          // With 2 consecutive critical intervals, criticalCount >= 2 → critical color
          expect(color).toBe(config.colors.critical);
        },
      ),
      { numRuns: 100 },
    );
  });
});
