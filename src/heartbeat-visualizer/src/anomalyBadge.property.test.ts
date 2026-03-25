import * as fc from 'fast-check';
import { getAnomalies } from './utils/chartUtils';
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
    leasePointsArb(2, 15),
    { minKeys: 1, maxKeys: 5 },
  )
  .map((dict) => dict as LeasesByNamespace);

// Feature: realistic-data-and-visualizations, Property 32: Anomaly badge count
describe('Property 32: Anomaly badge count', () => {
  // **Validates: Requirements 10.3**

  it('anomaly badge count equals the length of getAnomalies() result', () => {
    fc.assert(
      fc.property(leasesByNamespaceArb, (leasesData) => {
        const anomalies = getAnomalies(leasesData);

        // Manually count anomalies (gaps in warning range)
        let expectedCount = 0;
        for (const ns of Object.keys(leasesData)) {
          const arr = leasesData[ns];
          if (!arr || arr.length < 2) continue;
          for (let i = 1; i < arr.length; i++) {
            const gap = arr[i].y - arr[i - 1].y;
            if (gap > config.warningGapThreshold && gap < config.criticalGapThreshold) {
              expectedCount++;
            }
          }
        }

        expect(anomalies.length).toBe(expectedCount);
      }),
      { numRuns: 100 },
    );
  });
});

// Unit test: AnomalyBadge click selects most recent anomaly
describe('AnomalyBadge unit tests', () => {
  it('getAnomalies returns empty array for null input', () => {
    expect(getAnomalies(null)).toEqual([]);
  });

  it('getAnomalies returns empty array for empty data', () => {
    expect(getAnomalies({})).toEqual([]);
  });

  it('getAnomalies detects warning-range gaps correctly', () => {
    const data: LeasesByNamespace = {
      'test-node': [
        { x: 0, y: 1000000 },
        { x: 1, y: 1025000 }, // 25s gap — in warning range (10s < gap < 40s)
        { x: 2, y: 1035000 }, // 10s gap — normal
      ],
    };
    const anomalies = getAnomalies(data);
    expect(anomalies.length).toBe(1);
    expect(anomalies[0].namespace).toBe('test-node');
    expect(anomalies[0].gap).toBe(25000);
  });

  it('most recent anomaly is the one with the largest "to" timestamp', () => {
    const data: LeasesByNamespace = {
      'node-a': [
        { x: 0, y: 1000000 },
        { x: 1, y: 1025000 }, // warning gap
        { x: 2, y: 1035000 },
      ],
      'node-b': [
        { x: 0, y: 1000000 },
        { x: 1, y: 1010000 },
        { x: 2, y: 1040000 }, // 30s gap — warning range
      ],
    };
    const anomalies = getAnomalies(data);
    expect(anomalies.length).toBe(2);
    const mostRecent = anomalies.reduce((latest, a) => (a.to > latest.to ? a : latest));
    expect(mostRecent.to).toBe(1040000);
  });
});
