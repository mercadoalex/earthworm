import { config } from '../config';
import type { LeasePoint, LeasesByNamespace, Anomaly, NodeAnomaly } from '../types/heartbeat';

/**
 * Returns the segment color based on the gap between two consecutive lease points.
 * - Green for normal intervals (gap <= warningGapThreshold)
 * - Yellow/warning for gaps between warning and critical thresholds
 * - Red/critical for gaps exceeding critical threshold (with multi-interval logic)
 */
export function getSegmentColor(data: LeasePoint[], index: number): string {
  if (index === 0 || !data[index] || !data[index - 1]) {
    return config.colors.healthy;
  }

  const prev = data[index - 1].y;
  const curr = data[index].y;
  const interval = (curr - prev) / 1000; // seconds

  if (interval > config.criticalGapThreshold / 1000) {
    // Check previous intervals for multi-interval critical logic
    let criticalCount = 1;
    if (index > 1 && data[index - 2]) {
      const prevInterval = (data[index - 1].y - data[index - 2].y) / 1000;
      if (prevInterval > config.criticalGapThreshold / 1000) criticalCount++;
    }
    if (index > 2 && data[index - 3]) {
      const prevPrevInterval = (data[index - 2].y - data[index - 3].y) / 1000;
      if (prevPrevInterval > config.criticalGapThreshold / 1000) criticalCount++;
    }
    return criticalCount >= 2 ? config.colors.critical : config.colors.warning;
  }

  return config.colors.healthy;
}

/**
 * Returns true if any gap between consecutive y values is in the warning range
 * (> warningGapThreshold and < criticalGapThreshold).
 */
export function hasWarning(data: LeasePoint[]): boolean {
  if (!data || data.length < 2) return false;
  for (let i = 1; i < data.length; i++) {
    const gap = data[i].y - data[i - 1].y;
    if (gap > config.warningGapThreshold && gap < config.criticalGapThreshold) return true;
  }
  return false;
}

/**
 * Returns true if any gap between consecutive y values exceeds the critical threshold (death).
 */
export function hasDeath(data: LeasePoint[]): boolean {
  if (!data || data.length < 2) return false;
  for (let i = 1; i < data.length; i++) {
    const gap = data[i].y - data[i - 1].y;
    if (gap > config.criticalGapThreshold) return true;
  }
  return false;
}

/**
 * Detects anomalies (gaps in the warning range) across all namespaces.
 */
export function getAnomalies(leasesData: LeasesByNamespace | null): Anomaly[] {
  if (!leasesData) return [];
  const anomalies: Anomaly[] = [];
  Object.entries(leasesData).forEach(([ns, arr]) => {
    if (!arr || arr.length < 2) return;
    for (let i = 1; i < arr.length; i++) {
      const gap = arr[i].y - arr[i - 1].y;
      if (gap > config.warningGapThreshold && gap < config.criticalGapThreshold) {
        anomalies.push({
          namespace: ns,
          index: i,
          gap,
          from: arr[i - 1].y,
          to: arr[i].y,
        });
      }
    }
  });
  return anomalies;
}

/**
 * Detects anomalies with per-node information.
 * Returns NodeAnomaly[] which extends Anomaly with nodeName.
 */
export function getNodeAnomalies(leasesData: LeasesByNamespace | null): NodeAnomaly[] {
  if (!leasesData) return [];
  const anomalies: NodeAnomaly[] = [];
  Object.entries(leasesData).forEach(([ns, arr]) => {
    if (!arr || arr.length < 2) return;
    for (let i = 1; i < arr.length; i++) {
      const gap = arr[i].y - arr[i - 1].y;
      if (gap > config.warningGapThreshold && gap < config.criticalGapThreshold) {
        anomalies.push({
          nodeName: ns,
          namespace: ns,
          index: i,
          gap,
          from: arr[i - 1].y,
          to: arr[i].y,
        });
      }
    }
  });
  return anomalies;
}

/**
 * Unified status-to-color mapping used by all views.
 * Green for ready, yellow for warning, red for critical.
 */
export function getStatusColor(status: string): string {
  switch (status.toLowerCase()) {
    case 'ready':
      return config.colors.healthy;
    case 'warning':
    case 'unknown':
      return config.colors.warning;
    case 'critical':
    case 'notready':
    case 'not_ready':
      return config.colors.critical;
    default:
      return config.colors.healthy;
  }
}

/**
 * Formats a timestamp (ms) as "Monday, April 1st, 2025".
 */
export function formatFullDate(ms: number | null): string {
  if (!ms) return '';
  const date = new Date(ms);
  const dayName = date.toLocaleDateString('en-US', { weekday: 'long' });
  const monthName = date.toLocaleDateString('en-US', { month: 'long' });
  const day = date.getDate();
  const ordinal = (n: number): string => {
    if (n > 3 && n < 21) return 'th';
    switch (n % 10) {
      case 1: return 'st';
      case 2: return 'nd';
      case 3: return 'rd';
      default: return 'th';
    }
  };
  const year = date.getFullYear();
  return `${dayName}, ${monthName} ${day}${ordinal(day)}, ${year}`;
}
