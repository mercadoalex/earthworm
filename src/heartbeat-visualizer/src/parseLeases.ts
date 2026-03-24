// This script parses leases.yaml, splits Lease objects by namespace,
// converts them into [{x, y}, ...] format, and saves the result as leases+timestamp.json for frontend use.

import * as fs from 'fs';
import * as yaml from 'js-yaml';
import * as path from 'path';

/** Raw Kubernetes Lease metadata from YAML */
export interface LeaseMetadata {
  name: string;
  namespace: string;
}

/** Raw Kubernetes Lease spec from YAML */
export interface LeaseSpec {
  renewTime: string;
  leaseDurationSeconds: number;
}

/** A single Kubernetes Lease item from YAML */
export interface LeaseItem {
  metadata: LeaseMetadata;
  spec: LeaseSpec;
}

/** Top-level structure of leases.yaml */
export interface LeasesYaml {
  items: LeaseItem[];
}

/** A single data point for the chart: x = index, y = timestamp ms */
export interface LeasePoint {
  x: number;
  y: number;
}

/** Parsed leases grouped by namespace */
export interface LeasesByNamespace {
  [namespace: string]: LeasePoint[];
}

/**
 * Validates that a parsed YAML object is a valid LeasesYaml structure.
 * Throws descriptive errors for missing or malformed fields.
 */
function validateLeasesYaml(data: unknown): asserts data is LeasesYaml {
  if (data === null || data === undefined || typeof data !== 'object') {
    throw new Error('Malformed YAML: expected an object with an "items" array');
  }

  const obj = data as Record<string, unknown>;
  if (!Array.isArray(obj.items)) {
    throw new Error('Malformed YAML: missing required field "items" (expected an array)');
  }

  for (let i = 0; i < obj.items.length; i++) {
    const item = obj.items[i] as Record<string, unknown>;
    if (!item || typeof item !== 'object') {
      throw new Error(`Malformed YAML: items[${i}] is not an object`);
    }

    // Validate metadata
    if (!item.metadata || typeof item.metadata !== 'object') {
      throw new Error(`Malformed YAML: items[${i}].metadata is missing or not an object`);
    }
    const meta = item.metadata as Record<string, unknown>;
    if (typeof meta.name !== 'string' || meta.name.length === 0) {
      throw new Error(`Malformed YAML: items[${i}].metadata.name is missing or empty`);
    }
    if (typeof meta.namespace !== 'string' || meta.namespace.length === 0) {
      throw new Error(`Malformed YAML: items[${i}].metadata.namespace is missing or empty`);
    }

    // Validate spec
    if (!item.spec || typeof item.spec !== 'object') {
      throw new Error(`Malformed YAML: items[${i}].spec is missing or not an object`);
    }
    const spec = item.spec as Record<string, unknown>;
    if (typeof spec.renewTime !== 'string' || spec.renewTime.length === 0) {
      throw new Error(`Malformed YAML: items[${i}].spec.renewTime is missing or empty`);
    }
    if (typeof spec.leaseDurationSeconds !== 'number') {
      throw new Error(`Malformed YAML: items[${i}].spec.leaseDurationSeconds is missing or not a number`);
    }
  }
}

/**
 * Parses a YAML string containing Kubernetes Lease items and returns
 * lease data grouped by namespace in {x: index, y: timestampMs} format.
 */
export function parseLeases(yamlContent: string): LeasesByNamespace {
  let parsed: unknown;
  try {
    parsed = yaml.load(yamlContent);
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    throw new Error(`Failed to parse YAML: ${msg}`);
  }

  validateLeasesYaml(parsed);

  // Group items by namespace
  const grouped: Record<string, LeaseItem[]> = {};
  for (const item of parsed.items) {
    const ns = item.metadata.namespace;
    if (!grouped[ns]) grouped[ns] = [];
    grouped[ns].push(item);
  }

  // Convert each namespace's items to LeasePoint[]
  const result: LeasesByNamespace = {};
  for (const ns of Object.keys(grouped)) {
    result[ns] = grouped[ns].map((lease, idx) => ({
      x: idx,
      y: new Date(lease.spec.renewTime).getTime(),
    }));
  }

  return result;
}

/**
 * Generates a timestamp string in YYYYMMDDTHHmmss format.
 */
export function generateTimestamp(date: Date = new Date()): string {
  const pad = (n: number): string => n.toString().padStart(2, '0');
  return (
    date.getFullYear().toString() +
    pad(date.getMonth() + 1) +
    pad(date.getDate()) +
    'T' +
    pad(date.getHours()) +
    pad(date.getMinutes()) +
    pad(date.getSeconds())
  );
}

/**
 * Generates the output filename for parsed leases.
 */
export function generateOutputFilename(date: Date = new Date()): string {
  return `leases${generateTimestamp(date)}.json`;
}

/**
 * Serializes a LeasesByNamespace structure back into a valid YAML Lease list string.
 *
 * When originalItems are provided, metadata.name and spec.leaseDurationSeconds
 * are preserved from the original data. Otherwise, defaults are used
 * (name = "lease-{namespace}-{index}", leaseDurationSeconds = 50).
 *
 * Round-trip guarantee: parse(yaml) → json → serialize(json, originalItems) → yaml2 → parse(yaml2)
 * produces an equivalent LeasesByNamespace.
 */
export function serializeToYaml(
  data: LeasesByNamespace,
  originalItems?: LeaseItem[],
): string {
  // Build a lookup from original items: namespace → LeaseItem[] (in order)
  const originalByNs: Record<string, LeaseItem[]> = {};
  if (originalItems) {
    for (const item of originalItems) {
      const ns = item.metadata.namespace;
      if (!originalByNs[ns]) originalByNs[ns] = [];
      originalByNs[ns].push(item);
    }
  }

  const items: LeaseItem[] = [];
  for (const ns of Object.keys(data)) {
    const points = data[ns];
    const origNsItems = originalByNs[ns] || [];

    for (let i = 0; i < points.length; i++) {
      const point = points[i];
      const origItem = origNsItems[i];

      const name = origItem
        ? origItem.metadata.name
        : `lease-${ns}-${i}`;
      const leaseDurationSeconds = origItem
        ? origItem.spec.leaseDurationSeconds
        : 50;

      items.push({
        metadata: { name, namespace: ns },
        spec: {
          renewTime: new Date(point.y).toISOString(),
          leaseDurationSeconds,
        },
      });
    }
  }

  const leasesYaml: LeasesYaml = { items };
  return yaml.dump(leasesYaml, {
    lineWidth: -1,
    noRefs: true,
    quotingType: '"',
    forceQuotes: false,
  });
}

// --- CLI entry point ---
// Only run when executed directly (not when imported)
if (require.main === module) {
  const leasesPath = path.resolve(__dirname, '../../leases.yaml');

  console.log('Looking for leases.yaml at:', leasesPath);
  if (!fs.existsSync(leasesPath)) {
    console.error('Error: leases.yaml not found at ' + leasesPath);
    process.exit(1);
  }

  let yamlContent: string;
  try {
    yamlContent = fs.readFileSync(leasesPath, 'utf8');
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    console.error(`Error reading file: ${msg}`);
    process.exit(1);
  }

  let leasesByNamespace: LeasesByNamespace;
  try {
    leasesByNamespace = parseLeases(yamlContent);
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    console.error(`Error: ${msg}`);
    process.exit(1);
  }

  // Log grouped counts
  console.log('Total namespaces:', Object.keys(leasesByNamespace).length);
  for (const ns of Object.keys(leasesByNamespace)) {
    console.log(`Namespace: ${ns}, Points: ${leasesByNamespace[ns].length}`);
  }

  // Write output
  const outFile = path.join(__dirname, generateOutputFilename());
  fs.writeFileSync(outFile, JSON.stringify(leasesByNamespace, null, 2));
  console.log('leases JSON written to', outFile);
  console.log('leases JSON has been created successfully.');
}
