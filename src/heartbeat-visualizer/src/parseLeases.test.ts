import * as yaml from 'js-yaml';
import {
  parseLeases,
  serializeToYaml,
  generateOutputFilename,
  generateTimestamp,
  LeaseItem,
  LeasesYaml,
  LeasesByNamespace,
} from './parseLeases';

// --- Unit tests for parseLeases ---

describe('parseLeases', () => {
  const validYaml = `
items:
  - metadata:
      name: test-lease
      namespace: test-ns
    spec:
      renewTime: "2025-09-08T21:00:00.000Z"
      leaseDurationSeconds: 50
  - metadata:
      name: test-lease
      namespace: test-ns
    spec:
      renewTime: "2025-09-08T21:00:50.000Z"
      leaseDurationSeconds: 50
  - metadata:
      name: other-lease
      namespace: other-ns
    spec:
      renewTime: "2025-09-08T22:00:00.000Z"
      leaseDurationSeconds: 30
`;

  it('parses valid YAML into namespaced LeasePoint arrays', () => {
    const result = parseLeases(validYaml);
    expect(Object.keys(result)).toEqual(['test-ns', 'other-ns']);
    expect(result['test-ns']).toHaveLength(2);
    expect(result['other-ns']).toHaveLength(1);
    expect(result['test-ns'][0]).toEqual({ x: 0, y: new Date('2025-09-08T21:00:00.000Z').getTime() });
    expect(result['test-ns'][1]).toEqual({ x: 1, y: new Date('2025-09-08T21:00:50.000Z').getTime() });
    expect(result['other-ns'][0]).toEqual({ x: 0, y: new Date('2025-09-08T22:00:00.000Z').getTime() });
  });

  it('throws on malformed YAML', () => {
    expect(() => parseLeases('{{{')).toThrow('Failed to parse YAML');
  });

  it('throws on missing items field', () => {
    expect(() => parseLeases('foo: bar')).toThrow('missing required field "items"');
  });

  it('throws on missing metadata', () => {
    const bad = `
items:
  - spec:
      renewTime: "2025-01-01T00:00:00Z"
      leaseDurationSeconds: 50
`;
    expect(() => parseLeases(bad)).toThrow('metadata is missing');
  });

  it('throws on missing namespace', () => {
    const bad = `
items:
  - metadata:
      name: test
    spec:
      renewTime: "2025-01-01T00:00:00Z"
      leaseDurationSeconds: 50
`;
    expect(() => parseLeases(bad)).toThrow('namespace is missing or empty');
  });

  it('throws on missing renewTime', () => {
    const bad = `
items:
  - metadata:
      name: test
      namespace: ns
    spec:
      leaseDurationSeconds: 50
`;
    expect(() => parseLeases(bad)).toThrow('renewTime is missing or empty');
  });

  it('throws on null input', () => {
    // yaml.load of empty string returns undefined
    expect(() => parseLeases('')).toThrow('Malformed YAML');
  });
});

describe('serializeToYaml', () => {
  it('serializes LeasesByNamespace back to valid YAML', () => {
    const data: LeasesByNamespace = {
      'test-ns': [
        { x: 0, y: new Date('2025-09-08T21:00:00.000Z').getTime() },
        { x: 1, y: new Date('2025-09-08T21:00:50.000Z').getTime() },
      ],
    };
    const yamlStr = serializeToYaml(data);
    const parsed = yaml.load(yamlStr) as LeasesYaml;
    expect(parsed.items).toHaveLength(2);
    expect(parsed.items[0].metadata.namespace).toBe('test-ns');
    expect(parsed.items[0].spec.renewTime).toBe('2025-09-08T21:00:00.000Z');
  });

  it('round-trips: parse → serialize → parse produces equivalent JSON', () => {
    const originalYaml = `
items:
  - metadata:
      name: lease-a
      namespace: ns-a
    spec:
      renewTime: "2025-09-08T21:00:00.000Z"
      leaseDurationSeconds: 50
  - metadata:
      name: lease-a
      namespace: ns-a
    spec:
      renewTime: "2025-09-08T21:00:50.000Z"
      leaseDurationSeconds: 50
  - metadata:
      name: lease-b
      namespace: ns-b
    spec:
      renewTime: "2025-09-08T22:00:00.000Z"
      leaseDurationSeconds: 30
`;
    // Parse original
    const json1 = parseLeases(originalYaml);

    // Get original items for metadata preservation
    const origParsed = yaml.load(originalYaml) as LeasesYaml;

    // Serialize back to YAML
    const yaml2 = serializeToYaml(json1, origParsed.items);

    // Parse again
    const json2 = parseLeases(yaml2);

    // Should be equivalent
    expect(json2).toEqual(json1);
  });

  it('round-trips without original items (uses defaults)', () => {
    const data: LeasesByNamespace = {
      'my-ns': [
        { x: 0, y: new Date('2025-01-01T00:00:00.000Z').getTime() },
        { x: 1, y: new Date('2025-01-01T00:01:00.000Z').getTime() },
      ],
    };
    const yamlStr = serializeToYaml(data);
    const result = parseLeases(yamlStr);
    expect(result).toEqual(data);
  });
});

describe('generateOutputFilename', () => {
  it('generates filename in leases{YYYYMMDDTHHmmss}.json format', () => {
    const date = new Date(2025, 8, 8, 14, 30, 45); // Sep 8, 2025 14:30:45
    const filename = generateOutputFilename(date);
    expect(filename).toBe('leases20250908T143045.json');
  });

  it('pads single-digit months and days', () => {
    const date = new Date(2025, 0, 5, 3, 7, 9); // Jan 5, 2025 03:07:09
    const filename = generateOutputFilename(date);
    expect(filename).toBe('leases20250105T030709.json');
  });
});

describe('generateTimestamp', () => {
  it('matches YYYYMMDDTHHmmss pattern (15 chars)', () => {
    const ts = generateTimestamp(new Date());
    expect(ts).toMatch(/^\d{8}T\d{6}$/);
    expect(ts.length).toBe(15);
  });
});
