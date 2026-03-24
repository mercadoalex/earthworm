import * as fc from 'fast-check';
import * as yaml from 'js-yaml';
import {
  parseLeases,
  serializeToYaml,
  generateOutputFilename,
  generateTimestamp,
  LeaseItem,
  LeasesYaml,
} from './parseLeases';

// Helper: generate a valid lease item
const leaseItemArb = fc.record({
  name: fc.stringMatching(/^[a-z][a-z0-9-]{0,12}$/).filter((s) => s.length > 0),
  namespace: fc.stringMatching(/^[a-z][a-z0-9-]{0,12}$/).filter((s) => s.length > 0),
  renewTime: fc.date({
    min: new Date('2020-01-01T00:00:00Z'),
    max: new Date('2030-12-31T23:59:59Z'),
  }),
  leaseDurationSeconds: fc.integer({ min: 1, max: 3600 }),
});

// Helper: generate a valid LeasesYaml structure and its YAML string
const validLeasesYamlArb = fc
  .array(leaseItemArb, { minLength: 1, maxLength: 10 })
  .map((items) => {
    const leasesYaml: LeasesYaml = {
      items: items.map((item) => ({
        metadata: { name: item.name, namespace: item.namespace },
        spec: {
          renewTime: item.renewTime.toISOString(),
          leaseDurationSeconds: item.leaseDurationSeconds,
        },
      })),
    };
    const yamlStr = yaml.dump(leasesYaml, {
      lineWidth: -1,
      noRefs: true,
      quotingType: '"',
      forceQuotes: false,
    });
    return { leasesYaml, yamlStr };
  });

// Feature: earthworm-improvements, Property 10: Lease parser round-trip
describe('Property 10: Lease parser round-trip', () => {
  // **Validates: Requirements 10.1, 10.4, 10.5**

  it('parse → serialize → parse produces equivalent JSON', () => {
    fc.assert(
      fc.property(validLeasesYamlArb, ({ leasesYaml, yamlStr }) => {
        // First parse
        const json1 = parseLeases(yamlStr);

        // Serialize back to YAML (with original items for metadata preservation)
        const yaml2 = serializeToYaml(json1, leasesYaml.items);

        // Parse again
        const json2 = parseLeases(yaml2);

        // Should be equivalent
        expect(json2).toEqual(json1);
      }),
      { numRuns: 100 },
    );
  });

  it('parse → serialize (without originals) → parse produces equivalent JSON', () => {
    fc.assert(
      fc.property(validLeasesYamlArb, ({ yamlStr }) => {
        const json1 = parseLeases(yamlStr);
        const yaml2 = serializeToYaml(json1);
        const json2 = parseLeases(yaml2);
        expect(json2).toEqual(json1);
      }),
      { numRuns: 100 },
    );
  });
});

// Feature: earthworm-improvements, Property 11: Lease parser rejects invalid input
describe('Property 11: Lease parser rejects invalid input', () => {
  // **Validates: Requirements 10.2**

  it('throws descriptive error for invalid YAML strings', () => {
    const invalidYamlArb = fc.oneof(
      // Completely invalid YAML
      fc.constant('{{{'),
      fc.constant(':::'),
      fc.constant('\t\t\t---\n\t\tbad:\n\t\t\t- [[['),
      // Valid YAML but missing items field
      fc.constant('foo: bar'),
      fc.constant('items: not-an-array'),
      // Empty input
      fc.constant(''),
      // Null-like
      fc.constant('null'),
      // Items with missing metadata
      fc.constant('items:\n  - spec:\n      renewTime: "2025-01-01T00:00:00Z"\n      leaseDurationSeconds: 50'),
      // Items with missing namespace
      fc.constant('items:\n  - metadata:\n      name: test\n    spec:\n      renewTime: "2025-01-01T00:00:00Z"\n      leaseDurationSeconds: 50'),
      // Items with missing renewTime
      fc.constant('items:\n  - metadata:\n      name: test\n      namespace: ns\n    spec:\n      leaseDurationSeconds: 50'),
    );

    fc.assert(
      fc.property(invalidYamlArb, (input) => {
        expect(() => parseLeases(input)).toThrow();
      }),
      { numRuns: 100 },
    );
  });

  it('error messages are descriptive (contain useful context)', () => {
    const invalidInputs = [
      { input: '{{{', pattern: /parse|YAML/i },
      { input: 'foo: bar', pattern: /items/i },
      { input: '', pattern: /Malformed/i },
    ];

    for (const { input, pattern } of invalidInputs) {
      expect(() => parseLeases(input)).toThrow(pattern);
    }
  });
});

// Feature: earthworm-improvements, Property 12: Lease parser output filename format
describe('Property 12: Lease parser output filename format', () => {
  // **Validates: Requirements 10.3**

  it('filename matches leases{YYYYMMDDTHHmmss}.json pattern for any date', () => {
    const dateArb = fc.date({
      min: new Date('2000-01-01T00:00:00Z'),
      max: new Date('2099-12-31T23:59:59Z'),
    });

    fc.assert(
      fc.property(dateArb, (date) => {
        const filename = generateOutputFilename(date);
        expect(filename).toMatch(/^leases\d{8}T\d{6}\.json$/);
      }),
      { numRuns: 100 },
    );
  });

  it('timestamp portion is exactly 15 characters', () => {
    const dateArb = fc.date({
      min: new Date('2000-01-01T00:00:00Z'),
      max: new Date('2099-12-31T23:59:59Z'),
    });

    fc.assert(
      fc.property(dateArb, (date) => {
        const ts = generateTimestamp(date);
        expect(ts).toMatch(/^\d{8}T\d{6}$/);
        expect(ts.length).toBe(15);
      }),
      { numRuns: 100 },
    );
  });

  it('timestamp components match the input date', () => {
    const dateArb = fc.date({
      min: new Date('2000-01-01T00:00:00Z'),
      max: new Date('2099-12-31T23:59:59Z'),
    });

    fc.assert(
      fc.property(dateArb, (date) => {
        const ts = generateTimestamp(date);
        const year = parseInt(ts.substring(0, 4), 10);
        const month = parseInt(ts.substring(4, 6), 10);
        const day = parseInt(ts.substring(6, 8), 10);
        const hour = parseInt(ts.substring(9, 11), 10);
        const minute = parseInt(ts.substring(11, 13), 10);
        const second = parseInt(ts.substring(13, 15), 10);

        expect(year).toBe(date.getFullYear());
        expect(month).toBe(date.getMonth() + 1);
        expect(day).toBe(date.getDate());
        expect(hour).toBe(date.getHours());
        expect(minute).toBe(date.getMinutes());
        expect(second).toBe(date.getSeconds());
      }),
      { numRuns: 100 },
    );
  });
});
