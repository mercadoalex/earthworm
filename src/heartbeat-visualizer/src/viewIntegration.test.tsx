import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import { ViewProvider, useViewContext } from './contexts/ViewContext';
import ViewSelector from './ViewSelector';
import HeatmapView from './views/HeatmapView';
import TimelineView from './views/TimelineView';
import HistogramView from './views/HistogramView';
import NodeTable from './views/NodeTable';
import AnomalyBadge from './components/AnomalyBadge';
import type { LeasesByNamespace } from './types/heartbeat';

// Feature: realistic-data-and-visualizations
// Task 14.6: Unit test — default view is 'line', all 5 views render without crashing

// Mock ResizeObserver for Recharts ResponsiveContainer
beforeAll(() => {
  class MockResizeObserver {
    observe = jest.fn();
    unobserve = jest.fn();
    disconnect = jest.fn();
  }
  (global as any).ResizeObserver = MockResizeObserver;
});

// Sample data for rendering tests
const sampleLeasesData: LeasesByNamespace = {
  'kube-system': [
    { x: 0, y: 1700000000000 },
    { x: 1, y: 1700000010000 },
    { x: 2, y: 1700000020000 },
    { x: 3, y: 1700000030000 },
  ],
  'production': [
    { x: 0, y: 1700000000000 },
    { x: 1, y: 1700000011000 },
    { x: 2, y: 1700000021000 },
    { x: 3, y: 1700000031000 },
  ],
};

// Helper to read context
const ContextReader: React.FC = () => {
  const { activeView } = useViewContext();
  return <span data-testid="current-view">{activeView}</span>;
};

describe('View Integration — Task 14.6', () => {
  it('default view is "line"', () => {
    render(
      <ViewProvider>
        <ContextReader />
      </ViewProvider>,
    );
    expect(screen.getByTestId('current-view')).toHaveTextContent('line');
  });

  it('ViewSelector defaults to Line Chart selected', () => {
    render(
      <ViewProvider>
        <ViewSelector />
      </ViewProvider>,
    );
    const lineTab = screen.getByRole('tab', { name: /line chart/i });
    expect(lineTab).toHaveAttribute('aria-selected', 'true');
  });

  it('HeatmapView renders without crashing with sample data', () => {
    const { container } = render(
      <HeatmapView leasesData={sampleLeasesData} width={600} />,
    );
    expect(container).toBeTruthy();
    expect(screen.getByLabelText('Heatmap view')).toBeInTheDocument();
  });

  it('TimelineView renders without crashing with sample data', () => {
    const { container } = render(
      <TimelineView leasesData={sampleLeasesData} width={600} />,
    );
    expect(container).toBeTruthy();
    expect(screen.getByLabelText('Timeline view')).toBeInTheDocument();
  });

  it('HistogramView renders without crashing with sample data', () => {
    const { container } = render(
      <HistogramView leasesData={sampleLeasesData} />,
    );
    expect(container).toBeTruthy();
    expect(screen.getByLabelText('Histogram view')).toBeInTheDocument();
  });

  it('NodeTable renders without crashing with sample data', () => {
    const { container } = render(
      <NodeTable leasesData={sampleLeasesData} />,
    );
    expect(container).toBeTruthy();
    expect(screen.getByLabelText('Node table')).toBeInTheDocument();
  });

  it('AnomalyBadge renders without crashing with sample data', () => {
    const { container } = render(
      <AnomalyBadge leasesData={sampleLeasesData} />,
    );
    expect(container).toBeTruthy();
    expect(screen.getByTestId('anomaly-badge')).toBeInTheDocument();
  });

  it('all 5 view tabs can be selected without errors', () => {
    render(
      <ViewProvider>
        <ViewSelector />
        <ContextReader />
      </ViewProvider>,
    );

    // Click each tab and verify it becomes active
    const tabs = ['Line Chart', 'Heatmap', 'Timeline', 'Histogram', 'Table'];
    const expectedViews = ['line', 'heatmap', 'timeline', 'histogram', 'table'];

    tabs.forEach((tabName, idx) => {
      const tab = screen.getByRole('tab', { name: new RegExp(tabName, 'i') });
      fireEvent.click(tab);
      expect(tab).toHaveAttribute('aria-selected', 'true');
      expect(screen.getByTestId('current-view')).toHaveTextContent(expectedViews[idx]);
    });
  });
});
