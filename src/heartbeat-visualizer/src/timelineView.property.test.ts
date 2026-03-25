import * as fc from 'fast-check';
import { buildSwimSegments } from './utils/timelineUtils';
import type { LeasePoint, LeasesByNamespace, EbpfEvent } from './types/heartbeat';

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

// Feature: realistic-data-and-visualizations, Property 28: Swimlane segment coverage
describe('Property 28: Swimlane segment coverage', () => {
  // **Validates: Requirements 8.1**

  it('segments are contiguous and cover the entire time range per node', () => {
    fc.assert(
      fc.property(leasesByNamespaceArb, (leasesData) => {
        const segments = buildSwimSegments(leasesData);
        if (segments.length === 0) return;

        // Group segments by node
        const byNode = new Map<string, typeof segments>();
        for (const seg of segments) {
          const arr = byNode.get(seg.nodeName) || [];
          arr.push(seg);
          byNode.set(seg.nodeName, arr);
        }

        byNode.forEach((nodeSegments, nodeName) => {
          // Sort by start time
          const sorted = [...nodeSegments].sort((a, b) => a.start - b.start);

          // Check contiguity: each segment's end should equal next segment's start
          // (since buildSwimSegments creates segments from consecutive lease pairs,
          //  the end of segment i is arr[i].y and start of segment i+1 is arr[i].y)
          for (let i = 1; i < sorted.length; i++) {
            // The end of segment i-1 should equal the start of segment i
            // because segments are built from consecutive lease points
            expect(sorted[i].start).toBe(sorted[i - 1].end);
          }

          // Verify coverage: first segment starts at first lease, last ends at last lease
          const leaseArr = leasesData[nodeName];
          if (leaseArr && leaseArr.length >= 2) {
            expect(sorted[0].start).toBe(leaseArr[0].y);
            expect(sorted[sorted.length - 1].end).toBe(leaseArr[leaseArr.length - 1].y);
          }
        });
      }),
      { numRuns: 100 },
    );
  });
});

// Feature: realistic-data-and-visualizations, Property 29: eBPF marker placement on swimlane
describe('Property 29: eBPF marker placement on swimlane', () => {
  // **Validates: Requirements 8.5**

  it('eBPF events are placed on the correct node swimlane at the correct timestamp', () => {
    // Generate leases and correlated eBPF events
    const arbWithEbpf = leasesByNamespaceArb.chain((leasesData) => {
      const namespaces = Object.keys(leasesData);
      if (namespaces.length === 0) return fc.constant({ leasesData, ebpfEvents: [] as EbpfEvent[] });

      // Find time range
      let tMin = Infinity;
      let tMax = -Infinity;
      for (const ns of namespaces) {
        const arr = leasesData[ns];
        if (!arr || arr.length === 0) continue;
        for (const pt of arr) {
          if (pt.y < tMin) tMin = pt.y;
          if (pt.y > tMax) tMax = pt.y;
        }
      }
      if (!isFinite(tMin) || !isFinite(tMax) || tMin >= tMax) {
        return fc.constant({ leasesData, ebpfEvents: [] as EbpfEvent[] });
      }

      const ebpfArb = fc.array(
        fc.record({
          timestamp: fc.integer({ min: Math.floor(tMin), max: Math.floor(tMax) }),
          namespace: fc.constantFrom(...namespaces),
          pod: fc.constantFrom(...namespaces), // pod matches namespace (node name)
          pid: fc.integer({ min: 1, max: 65535 }),
          comm: fc.constantFrom('kubelet', 'oom_reaper', 'containerd'),
          syscall: fc.constantFrom('write', 'exit', 'kill', 'fork'),
        }),
        { minLength: 0, maxLength: 5 },
      );

      return ebpfArb.map((ebpfEvents) => ({ leasesData, ebpfEvents }));
    });

    fc.assert(
      fc.property(arbWithEbpf, ({ leasesData, ebpfEvents }) => {
        const segments = buildSwimSegments(leasesData, ebpfEvents);

        // For each eBPF event attached to a segment, verify it's on the correct node
        for (const seg of segments) {
          if (seg.ebpfEvents) {
            for (const evt of seg.ebpfEvents) {
              // Event namespace matches segment namespace
              expect(evt.namespace).toBe(seg.namespace);
              // Event timestamp falls within segment time range
              expect(evt.timestamp).toBeGreaterThanOrEqual(seg.start);
              expect(evt.timestamp).toBeLessThanOrEqual(seg.end);
            }
          }
        }
      }),
      { numRuns: 100 },
    );
  });
});
