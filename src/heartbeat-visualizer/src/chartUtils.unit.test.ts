import { hasWarning, hasDeath } from './utils/chartUtils';
import { config } from './config';
import type { LeasePoint } from './types/heartbeat';

// Unit tests for hasWarning and hasDeath utility functions
// **Validates: Requirements 12.1, 12.2**

describe('hasWarning — unit tests', () => {
  const WARNING = config.warningGapThreshold; // 10000
  const CRITICAL = config.criticalGapThreshold; // 40000

  it('returns false for empty array', () => {
    expect(hasWarning([])).toBe(false);
  });

  it('returns false for single-element array', () => {
    expect(hasWarning([{ x: 0, y: 5000 }])).toBe(false);
  });

  it('returns false when gap is below warning threshold', () => {
    const points: LeasePoint[] = [
      { x: 0, y: 100000 },
      { x: 1, y: 100000 + WARNING }, // gap = 10000, not > 10000
    ];
    expect(hasWarning(points)).toBe(false);
  });

  it('returns true when gap is within warning range (just above warning threshold)', () => {
    const points: LeasePoint[] = [
      { x: 0, y: 100000 },
      { x: 1, y: 100000 + WARNING + 1 }, // gap = 10001, in (10000, 40000)
    ];
    expect(hasWarning(points)).toBe(true);
  });

  it('returns true when gap is in the middle of warning range', () => {
    const points: LeasePoint[] = [
      { x: 0, y: 100000 },
      { x: 1, y: 100000 + 25000 }, // gap = 25000, in (10000, 40000)
    ];
    expect(hasWarning(points)).toBe(true);
  });

  it('returns true when gap is just below critical threshold', () => {
    const points: LeasePoint[] = [
      { x: 0, y: 100000 },
      { x: 1, y: 100000 + CRITICAL - 1 }, // gap = 39999, in (10000, 40000)
    ];
    expect(hasWarning(points)).toBe(true);
  });

  it('returns false when gap equals critical threshold (not strictly less)', () => {
    const points: LeasePoint[] = [
      { x: 0, y: 100000 },
      { x: 1, y: 100000 + CRITICAL }, // gap = 40000, not < 40000
    ];
    expect(hasWarning(points)).toBe(false);
  });

  it('returns false when gap is above critical threshold', () => {
    const points: LeasePoint[] = [
      { x: 0, y: 100000 },
      { x: 1, y: 100000 + CRITICAL + 5000 }, // gap = 45000, above critical
    ];
    expect(hasWarning(points)).toBe(false);
  });

  it('returns true when at least one gap in multi-point array is in warning range', () => {
    const points: LeasePoint[] = [
      { x: 0, y: 100000 },
      { x: 1, y: 105000 },  // gap = 5000, below warning
      { x: 2, y: 130000 },  // gap = 25000, in warning range
      { x: 3, y: 135000 },  // gap = 5000, below warning
    ];
    expect(hasWarning(points)).toBe(true);
  });

  it('returns false when all gaps are below warning threshold', () => {
    const points: LeasePoint[] = [
      { x: 0, y: 100000 },
      { x: 1, y: 105000 },  // gap = 5000
      { x: 2, y: 108000 },  // gap = 3000
      { x: 3, y: 110000 },  // gap = 2000
    ];
    expect(hasWarning(points)).toBe(false);
  });
});

describe('hasDeath — unit tests', () => {
  const CRITICAL = config.criticalGapThreshold; // 40000

  it('returns false for empty array', () => {
    expect(hasDeath([])).toBe(false);
  });

  it('returns false for single-element array', () => {
    expect(hasDeath([{ x: 0, y: 5000 }])).toBe(false);
  });

  it('returns false when gap is below critical threshold', () => {
    const points: LeasePoint[] = [
      { x: 0, y: 100000 },
      { x: 1, y: 100000 + 30000 }, // gap = 30000, below 40000
    ];
    expect(hasDeath(points)).toBe(false);
  });

  it('returns false when gap equals critical threshold exactly', () => {
    const points: LeasePoint[] = [
      { x: 0, y: 100000 },
      { x: 1, y: 100000 + CRITICAL }, // gap = 40000, not > 40000
    ];
    expect(hasDeath(points)).toBe(false);
  });

  it('returns true when gap is just above critical threshold', () => {
    const points: LeasePoint[] = [
      { x: 0, y: 100000 },
      { x: 1, y: 100000 + CRITICAL + 1 }, // gap = 40001
    ];
    expect(hasDeath(points)).toBe(true);
  });

  it('returns true when gap is well above critical threshold', () => {
    const points: LeasePoint[] = [
      { x: 0, y: 100000 },
      { x: 1, y: 200000 }, // gap = 100000
    ];
    expect(hasDeath(points)).toBe(true);
  });

  it('returns true when at least one gap in multi-point array exceeds critical', () => {
    const points: LeasePoint[] = [
      { x: 0, y: 100000 },
      { x: 1, y: 105000 },  // gap = 5000, normal
      { x: 2, y: 155000 },  // gap = 50000, death
      { x: 3, y: 160000 },  // gap = 5000, normal
    ];
    expect(hasDeath(points)).toBe(true);
  });

  it('returns false when all gaps are within warning range but below critical', () => {
    const points: LeasePoint[] = [
      { x: 0, y: 100000 },
      { x: 1, y: 125000 },  // gap = 25000, warning but not death
      { x: 2, y: 150000 },  // gap = 25000, warning but not death
    ];
    expect(hasDeath(points)).toBe(false);
  });
});
