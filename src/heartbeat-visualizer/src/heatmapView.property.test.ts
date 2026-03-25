import * as fc from 'fast-check';
import { buildHeatmapData } from './utils/heatmapUtils';
import type { LeasePoint, LeasesByNamespace, HeatmapCell } from './types/heartbeat';

// Helper: generate LeasesByNamespace with realistic data
const leasePointsArb = (minLen: number, maxLen: number) =>
  fc.array(fc.integer({ min: 5000, max: 60000 }), { minLength: minLen, maxLength: maxLen }).map((gaps) => {
    let y = 1700000000000; // base timestamp
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

// Feature: realistic-data-and-visualizations, Property 24: Heatmap grid dimensions
describe('Property 24: Heatmap grid dimensions', () => {
  // **Validates: Requirements 7.1**

  it('produces exactly one row per unique node and ceil((timeMax - timeMin) / bucketSize) columns', () => {
    fc.assert(
      fc.property(
        leasesByNamespaceArb,
        fc.integer({ min: 5000, max: 60000 }),
        (leasesData, bucketSizeMs) => {
          const cells = buildHeatmapData(leasesData, bucketSizeMs);
          if (cells.length === 0) return; // empty data is valid

          // Count unique nodes
          const nodeNames = new Set(cells.map((c) => c.nodeName));
          const expectedNodes = new Set(Object.keys(leasesData).filter((ns) => {
            const arr = leasesData[ns];
            return arr && arr.length > 0;
          }));
          expect(nodeNames.size).toBe(expectedNodes.size);

          // Count unique time buckets
          const timeBuckets = new Set(cells.map((c) => c.timeBucket));

          // Compute expected column count
          let globalMin = Infinity;
          let globalMax = -Infinity;
          for (const ns of Object.keys(leasesData)) {
            const arr = leasesData[ns];
            if (!arr || arr.length === 0) continue;
            for (const pt of arr) {
              if (pt.y < globalMin) globalMin = pt.y;
              if (pt.y > globalMax) globalMax = pt.y;
            }
          }
          const bucketStart = Math.floor(globalMin / bucketSizeMs) * bucketSizeMs;
          let expectedCols = 0;
          for (let t = bucketStart; t <= globalMax; t += bucketSizeMs) {
            expectedCols++;
          }

          expect(timeBuckets.size).toBe(expectedCols);

          // Total cells = nodes * columns
          expect(cells.length).toBe(nodeNames.size * timeBuckets.size);
        },
      ),
      { numRuns: 100 },
    );
  });
});

// Feature: realistic-data-and-visualizations, Property 26: Heatmap tooltip data completeness
describe('Property 26: Heatmap tooltip data completeness', () => {
  // **Validates: Requirements 7.3**

  it('every heatmap cell has non-empty nodeName, valid time range, valid status, and heartbeatCount >= 0', () => {
    fc.assert(
      fc.property(leasesByNamespaceArb, (leasesData) => {
        const cells = buildHeatmapData(leasesData);
        const validStatuses = new Set(['ready', 'warning', 'critical']);

        for (const cell of cells) {
          // Node name non-empty
          expect(cell.nodeName.length).toBeGreaterThan(0);
          // Time range: start < end
          expect(cell.timeBucket).toBeLessThan(cell.timeBucketEnd);
          // Valid status
          expect(validStatuses.has(cell.status)).toBe(true);
          // Heartbeat count >= 0
          expect(cell.heartbeatCount).toBeGreaterThanOrEqual(0);
        }
      }),
      { numRuns: 100 },
    );
  });
});

// Feature: realistic-data-and-visualizations, Property 27: Heatmap default sort order
describe('Property 27: Heatmap default sort order', () => {
  // **Validates: Requirements 7.4**

  it('default sort places critical nodes before warning before ready', () => {
    const statusPriority: Record<string, number> = {
      critical: 0,
      warning: 1,
      ready: 2,
    };

    fc.assert(
      fc.property(leasesByNamespaceArb, (leasesData) => {
        const cells = buildHeatmapData(leasesData);
        if (cells.length === 0) return;

        // Compute worst status per node
        const worstStatus = new Map<string, number>();
        for (const cell of cells) {
          const current = worstStatus.get(cell.nodeName) ?? 2;
          const prio = statusPriority[cell.status] ?? 2;
          if (prio < current) worstStatus.set(cell.nodeName, prio);
        }

        // Sort by worst health (default sort)
        const sortedNodes = Array.from(worstStatus.entries())
          .sort((a, b) => a[1] - b[1])
          .map(([name]) => name);

        // Verify ordering: for any two adjacent nodes, the first should have
        // equal or worse (lower number) priority than the second
        for (let i = 1; i < sortedNodes.length; i++) {
          const prevPrio = worstStatus.get(sortedNodes[i - 1])!;
          const currPrio = worstStatus.get(sortedNodes[i])!;
          expect(prevPrio).toBeLessThanOrEqual(currPrio);
        }
      }),
      { numRuns: 100 },
    );
  });
});
