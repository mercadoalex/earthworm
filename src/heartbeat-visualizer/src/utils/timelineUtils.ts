import { config } from '../config';
import type { LeasesByNamespace, SwimSegment, EbpfEvent } from '../types/heartbeat';

/**
 * Transforms LeasesByNamespace into contiguous SwimSegment arrays per node.
 * Each segment represents a contiguous period of Ready, NotReady, or Unknown status.
 *
 * @param leasesData - The lease data keyed by namespace
 * @param ebpfEvents - Optional eBPF events to attach to gap segments
 * @returns Array of SwimSegment objects
 */
export function buildSwimSegments(
  leasesData: LeasesByNamespace | null,
  ebpfEvents?: EbpfEvent[],
): SwimSegment[] {
  if (!leasesData) return [];

  const segments: SwimSegment[] = [];

  for (const ns of Object.keys(leasesData)) {
    const arr = leasesData[ns];
    if (!arr || arr.length < 2) continue;

    const nodeName = ns;

    for (let i = 1; i < arr.length; i++) {
      const start = arr[i - 1].y;
      const end = arr[i].y;
      const gap = end - start;

      let status: 'Ready' | 'NotReady' | 'Unknown';
      if (gap > config.criticalGapThreshold) {
        status = 'NotReady';
      } else if (gap > config.warningGapThreshold) {
        status = 'Unknown';
      } else {
        status = 'Ready';
      }

      // Attach correlated eBPF events to non-Ready segments
      let attachedEbpf: EbpfEvent[] | undefined;
      if (status !== 'Ready' && ebpfEvents) {
        attachedEbpf = ebpfEvents.filter(
          (e) => e.namespace === ns && e.timestamp >= start && e.timestamp <= end,
        );
        if (attachedEbpf.length === 0) attachedEbpf = undefined;
      }

      segments.push({
        nodeName,
        namespace: ns,
        start,
        end,
        status,
        ebpfEvents: attachedEbpf,
      });
    }
  }

  return segments;
}
