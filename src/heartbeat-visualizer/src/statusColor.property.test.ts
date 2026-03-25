import * as fc from 'fast-check';
import { getStatusColor } from './utils/chartUtils';
import { config } from './config';

// Feature: realistic-data-and-visualizations, Property 25: Status-to-color mapping consistency
// **Validates: Requirements 7.2, 8.2**

const READY_STATUSES = ['ready', 'Ready', 'READY'];
const WARNING_STATUSES = ['warning', 'Warning', 'unknown', 'Unknown'];
const CRITICAL_STATUSES = ['critical', 'Critical', 'notready', 'NotReady', 'not_ready'];

describe('Property 25: Status-to-color mapping consistency', () => {
  it('same status always maps to the same color across multiple calls', () => {
    const statusArb = fc.constantFrom(
      'ready', 'Ready', 'warning', 'Warning', 'unknown', 'Unknown',
      'critical', 'Critical', 'notready', 'NotReady', 'not_ready',
    );

    fc.assert(
      fc.property(statusArb, fc.integer({ min: 2, max: 20 }), (status, callCount) => {
        const firstColor = getStatusColor(status);
        for (let i = 1; i < callCount; i++) {
          expect(getStatusColor(status)).toBe(firstColor);
        }
      }),
      { numRuns: 100 },
    );
  });

  it('ready statuses always map to healthy/green color', () => {
    const readyArb = fc.constantFrom(...READY_STATUSES);

    fc.assert(
      fc.property(readyArb, (status) => {
        expect(getStatusColor(status)).toBe(config.colors.healthy);
      }),
      { numRuns: 100 },
    );
  });

  it('warning/unknown statuses always map to warning/yellow color', () => {
    const warningArb = fc.constantFrom(...WARNING_STATUSES);

    fc.assert(
      fc.property(warningArb, (status) => {
        expect(getStatusColor(status)).toBe(config.colors.warning);
      }),
      { numRuns: 100 },
    );
  });

  it('critical/notready statuses always map to critical/red color', () => {
    const criticalArb = fc.constantFrom(...CRITICAL_STATUSES);

    fc.assert(
      fc.property(criticalArb, (status) => {
        expect(getStatusColor(status)).toBe(config.colors.critical);
      }),
      { numRuns: 100 },
    );
  });

  it('different status categories never map to the same color', () => {
    const readyArb = fc.constantFrom(...READY_STATUSES);
    const criticalArb = fc.constantFrom(...CRITICAL_STATUSES);

    fc.assert(
      fc.property(readyArb, criticalArb, (readyStatus, criticalStatus) => {
        const readyColor = getStatusColor(readyStatus);
        const criticalColor = getStatusColor(criticalStatus);
        expect(readyColor).not.toBe(criticalColor);
      }),
      { numRuns: 100 },
    );
  });
});
