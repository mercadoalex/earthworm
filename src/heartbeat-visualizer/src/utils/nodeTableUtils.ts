import { config } from '../config';
import type { LeasesByNamespace, NodeSummary } from '../types/heartbeat';

/**
 * Extracts per-node summary with last 20 intervals from LeasesByNamespace.
 *
 * @param leasesData - The lease data keyed by namespace
 * @returns Array of NodeSummary objects
 */
export function buildNodeSummaries(
  leasesData: LeasesByNamespace | null,
): NodeSummary[] {
  if (!leasesData) return [];

  const summaries: NodeSummary[] = [];

  for (const ns of Object.keys(leasesData)) {
    const arr = leasesData[ns];
    if (!arr || arr.length === 0) continue;

    const nodeName = ns;
    const lastHeartbeat = arr[arr.length - 1].y;

    // Compute intervals
    const allIntervals: number[] = [];
    for (let i = 1; i < arr.length; i++) {
      allIntervals.push(arr[i].y - arr[i - 1].y);
    }

    // Take last 20 intervals
    const recentIntervals = allIntervals.slice(-20);

    // Determine current status from most recent interval
    let currentStatus: string;
    if (recentIntervals.length === 0) {
      currentStatus = 'Unknown';
    } else {
      const lastInterval = recentIntervals[recentIntervals.length - 1];
      if (lastInterval > config.criticalGapThreshold) {
        currentStatus = 'NotReady';
      } else if (lastInterval > config.warningGapThreshold) {
        currentStatus = 'Warning';
      } else {
        currentStatus = 'Ready';
      }
    }

    summaries.push({
      nodeName,
      namespace: ns,
      currentStatus,
      lastHeartbeat,
      recentIntervals,
    });
  }

  return summaries;
}
