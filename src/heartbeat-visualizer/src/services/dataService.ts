import { config } from '../config';
import type {
  HeartbeatEvent,
  LeasesByNamespace,
  ChartDataPoint,
} from '../types/heartbeat';

/**
 * Fetch heartbeats from the API, optionally filtered by time range.
 */
export async function fetchHeartbeats(from?: Date, to?: Date): Promise<HeartbeatEvent[]> {
  const params = new URLSearchParams();
  if (from) params.set('from', from.toISOString());
  if (to) params.set('to', to.toISOString());

  const query = params.toString();
  const url = `${config.apiBaseUrl}/api/heartbeats${query ? `?${query}` : ''}`;

  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`Failed to fetch heartbeats: ${res.status}`);
  }
  return res.json();
}

/**
 * Post a heartbeat event to the API.
 */
export async function postHeartbeat(event: HeartbeatEvent): Promise<void> {
  const res = await fetch(`${config.apiBaseUrl}/api/heartbeat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(event),
  });
  if (!res.ok) {
    throw new Error(`Failed to post heartbeat: ${res.status}`);
  }
}

/**
 * Transform raw lease data into chart-ready data points.
 * Each point has an index, a timestamp, and a value per namespace.
 */
export function transformLeasesForChart(
  data: LeasesByNamespace,
  maxPoints: number
): ChartDataPoint[] {
  const namespaces = Object.keys(data);
  const chartData: ChartDataPoint[] = [];

  for (let i = 0; i < maxPoints; i++) {
    const point: ChartDataPoint = { index: i };
    for (const ns of namespaces) {
      if (data[ns][i] && data[ns][i].y) {
        point.timestamp = data[ns][i].y;
        break;
      }
    }
    namespaces.forEach((ns) => {
      if (data[ns][i]) {
        point[ns] = data[ns][i].x;
      }
    });
    chartData.push(point);
  }

  return chartData;
}
