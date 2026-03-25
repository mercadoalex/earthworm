import { config } from '../config';
import type { LeasesByNamespace, HistogramBin } from '../types/heartbeat';

/**
 * Computes interval frequency distribution from LeasesByNamespace.
 *
 * @param leasesData - The lease data keyed by namespace
 * @param binWidthSeconds - Width of each histogram bin in seconds (default 1)
 * @param namespaceFilter - Optional namespace to filter by
 * @returns Array of HistogramBin objects
 */
export function buildHistogramBins(
  leasesData: LeasesByNamespace | null,
  binWidthSeconds: number = 1,
  namespaceFilter?: string,
): HistogramBin[] {
  if (!leasesData) return [];

  // Collect all inter-lease intervals in seconds
  const intervals: number[] = [];
  const namespaces = namespaceFilter
    ? [namespaceFilter].filter((ns) => ns in leasesData)
    : Object.keys(leasesData);

  for (const ns of namespaces) {
    const arr = leasesData[ns];
    if (!arr || arr.length < 2) continue;
    for (let i = 1; i < arr.length; i++) {
      const intervalMs = arr[i].y - arr[i - 1].y;
      intervals.push(intervalMs / 1000); // convert to seconds
    }
  }

  if (intervals.length === 0) return [];

  // Find range
  const minInterval = Math.min(...intervals);
  const maxInterval = Math.max(...intervals);

  // Build bins
  const binStart = Math.floor(minInterval / binWidthSeconds) * binWidthSeconds;
  const binEnd = maxInterval;
  const totalIntervals = intervals.length;

  const bins: HistogramBin[] = [];

  for (let rangeStart = binStart; rangeStart <= binEnd; rangeStart += binWidthSeconds) {
    const rangeEnd = rangeStart + binWidthSeconds;

    // Count intervals in this bin
    const count = intervals.filter((v) => v >= rangeStart && v < rangeEnd).length;

    const percentage = (count / totalIntervals) * 100;

    // Classify severity based on thresholds (in seconds)
    const warningThresholdSec = config.warningGapThreshold / 1000;
    const criticalThresholdSec = config.criticalGapThreshold / 1000;

    let severity: 'normal' | 'warning' | 'critical';
    if (rangeStart >= criticalThresholdSec) {
      severity = 'critical';
    } else if (rangeStart >= warningThresholdSec) {
      severity = 'warning';
    } else {
      severity = 'normal';
    }

    bins.push({ rangeStart, rangeEnd, count, percentage, severity });
  }

  return bins;
}
