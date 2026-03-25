import { config } from '../config';
import type { LeasesByNamespace, HeatmapCell } from '../types/heartbeat';

/**
 * Transforms LeasesByNamespace into a HeatmapCell grid.
 * Each cell represents a (node, timeBucket) pair with heartbeat count and status.
 *
 * @param leasesData - The lease data keyed by namespace (namespace contains node data as arrays)
 * @param bucketSizeMs - Time bucket width in milliseconds (default 30000 = 30s)
 * @returns Array of HeatmapCell objects
 */
export function buildHeatmapData(
  leasesData: LeasesByNamespace | null,
  bucketSizeMs: number = 30000,
): HeatmapCell[] {
  if (!leasesData) return [];

  const namespaces = Object.keys(leasesData);
  if (namespaces.length === 0) return [];

  // Find global time range across all namespaces
  let globalMin = Infinity;
  let globalMax = -Infinity;
  for (const ns of namespaces) {
    const arr = leasesData[ns];
    if (!arr || arr.length === 0) continue;
    for (const pt of arr) {
      if (pt.y < globalMin) globalMin = pt.y;
      if (pt.y > globalMax) globalMax = pt.y;
    }
  }

  if (!isFinite(globalMin) || !isFinite(globalMax)) return [];

  // Build time buckets
  const bucketStart = Math.floor(globalMin / bucketSizeMs) * bucketSizeMs;
  const bucketEnd = globalMax;

  const cells: HeatmapCell[] = [];

  for (const ns of namespaces) {
    const arr = leasesData[ns];
    if (!arr || arr.length === 0) continue;

    // Each namespace is treated as a "node" row in the heatmap
    const nodeName = ns;

    for (let t = bucketStart; t <= bucketEnd; t += bucketSizeMs) {
      const tEnd = t + bucketSizeMs;

      // Count heartbeats in this bucket
      let heartbeatCount = 0;
      for (const pt of arr) {
        if (pt.y >= t && pt.y < tEnd) {
          heartbeatCount++;
        }
      }

      // Determine status based on heartbeat count and expected rate
      const expectedHeartbeats = bucketSizeMs / config.heartbeatInterval;
      let status: 'ready' | 'warning' | 'critical';
      if (heartbeatCount === 0) {
        status = 'critical';
      } else if (heartbeatCount < expectedHeartbeats * 0.5) {
        status = 'warning';
      } else {
        status = 'ready';
      }

      cells.push({
        nodeName,
        namespace: ns,
        timeBucket: t,
        timeBucketEnd: tEnd,
        status,
        heartbeatCount,
      });
    }
  }

  return cells;
}
