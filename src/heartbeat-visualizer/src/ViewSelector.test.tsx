import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import ViewSelector from './ViewSelector';
import { ViewProvider, useViewContext } from './contexts/ViewContext';

// Helper to render ViewSelector within its required provider
function renderWithProvider() {
  return render(
    <ViewProvider>
      <ViewSelector />
    </ViewProvider>,
  );
}

describe('ViewSelector', () => {
  it('renders all 5 view options', () => {
    renderWithProvider();
    expect(screen.getByRole('tab', { name: /line chart/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /heatmap/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /timeline/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /histogram/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /table/i })).toBeInTheDocument();
  });

  it('defaults to Line Chart as the active tab', () => {
    renderWithProvider();
    const lineTab = screen.getByRole('tab', { name: /line chart/i });
    expect(lineTab).toHaveAttribute('aria-selected', 'true');
  });

  it('switches active view when a tab is clicked', () => {
    renderWithProvider();
    const heatmapTab = screen.getByRole('tab', { name: /heatmap/i });
    fireEvent.click(heatmapTab);
    expect(heatmapTab).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: /line chart/i })).toHaveAttribute('aria-selected', 'false');
  });
});

// Helper component to read context values
const ContextReader: React.FC = () => {
  const { activeView, xDomain } = useViewContext();
  return (
    <div>
      <span data-testid="active-view">{activeView}</span>
      <span data-testid="x-domain">{xDomain ? `${xDomain[0]},${xDomain[1]}` : 'null'}</span>
    </div>
  );
};

describe('ViewContext', () => {
  it('defaults activeView to "line"', () => {
    render(
      <ViewProvider>
        <ContextReader />
      </ViewProvider>,
    );
    expect(screen.getByTestId('active-view')).toHaveTextContent('line');
  });

  it('defaults xDomain to null', () => {
    render(
      <ViewProvider>
        <ContextReader />
      </ViewProvider>,
    );
    expect(screen.getByTestId('x-domain')).toHaveTextContent('null');
  });

  it('throws when useViewContext is used outside ViewProvider', () => {
    // Suppress console.error for this test
    const spy = jest.spyOn(console, 'error').mockImplementation(() => {});
    expect(() => render(<ContextReader />)).toThrow(
      'useViewContext must be used within a ViewProvider',
    );
    spy.mockRestore();
  });
});
