import { config } from '../config';

/**
 * Returns a threshold-based color for a single heartbeat interval segment.
 *
 * @param intervalMs - The interval in milliseconds
 * @returns Color string: green for normal, yellow for warning, red for critical
 */
export function getSparklineColor(intervalMs: number): string {
  if (intervalMs > config.criticalGapThreshold) {
    return config.colors.critical;
  }
  if (intervalMs > config.warningGapThreshold) {
    return config.colors.warning;
  }
  return config.colors.healthy;
}
