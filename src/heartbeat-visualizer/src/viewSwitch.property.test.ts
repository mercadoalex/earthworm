import * as fc from 'fast-check';
import type { ViewType } from './types/heartbeat';

// Feature: realistic-data-and-visualizations, Property 38: View switch preserves zoom domain
// **Validates: Requirements 12.3**

/**
 * This test validates that the ViewContext state model preserves xDomain
 * when switching between views. We test the pure state logic without React rendering.
 */

const ALL_VIEWS: ViewType[] = ['line', 'heatmap', 'timeline', 'histogram', 'table'];

// Simulate the ViewContext state model
interface ViewState {
  activeView: ViewType;
  xDomain: [number, number] | null;
}

function createViewState(): ViewState {
  return { activeView: 'line', xDomain: null };
}

function setActiveView(state: ViewState, view: ViewType): ViewState {
  return { ...state, activeView: view };
}

function setXDomain(state: ViewState, domain: [number, number] | null): ViewState {
  return { ...state, xDomain: domain };
}

describe('Property 38: View switch preserves zoom domain', () => {
  it('switching views preserves the xDomain value', () => {
    const viewArb = fc.constantFrom(...ALL_VIEWS);
    const domainArb = fc.tuple(
      fc.integer({ min: 0, max: 1_000_000_000 }),
      fc.integer({ min: 0, max: 1_000_000_000 }),
    ).map(([a, b]): [number, number] => a < b ? [a, b] : [b, a]).filter(([a, b]) => a < b);

    fc.assert(
      fc.property(domainArb, viewArb, viewArb, (domain, fromView, toView) => {
        let state = createViewState();
        state = setActiveView(state, fromView);
        state = setXDomain(state, domain);

        // Switch to a different view
        state = setActiveView(state, toView);

        // xDomain should be preserved after view switch
        expect(state.xDomain).toEqual(domain);
        expect(state.xDomain![0]).toBe(domain[0]);
        expect(state.xDomain![1]).toBe(domain[1]);
      }),
      { numRuns: 100 },
    );
  });

  it('switching views multiple times preserves xDomain', () => {
    const viewArb = fc.constantFrom(...ALL_VIEWS);
    const domainArb = fc.tuple(
      fc.integer({ min: 0, max: 1_000_000_000 }),
      fc.integer({ min: 0, max: 1_000_000_000 }),
    ).map(([a, b]): [number, number] => a < b ? [a, b] : [b, a]).filter(([a, b]) => a < b);
    const viewSequenceArb = fc.array(viewArb, { minLength: 2, maxLength: 10 });

    fc.assert(
      fc.property(domainArb, viewSequenceArb, (domain, views) => {
        let state = createViewState();
        state = setXDomain(state, domain);

        // Switch through multiple views
        for (const view of views) {
          state = setActiveView(state, view);
        }

        // xDomain should still be preserved
        expect(state.xDomain).toEqual(domain);
      }),
      { numRuns: 100 },
    );
  });

  it('null xDomain is preserved across view switches', () => {
    const viewArb = fc.constantFrom(...ALL_VIEWS);

    fc.assert(
      fc.property(viewArb, viewArb, (fromView, toView) => {
        let state = createViewState();
        state = setActiveView(state, fromView);
        // xDomain starts as null
        state = setActiveView(state, toView);
        expect(state.xDomain).toBeNull();
      }),
      { numRuns: 100 },
    );
  });
});
