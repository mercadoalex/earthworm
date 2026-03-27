import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import ClusterSelector from './ClusterSelector';
import type { ClusterConfig } from './config';

const makeClusters = (count: number): ClusterConfig[] =>
  Array.from({ length: count }, (_, i) => ({
    name: `cluster-${i}`,
    manifestUrl: `/mocking_data/leases.manifest.json`,
    ebpfManifestUrl: `/mocking_data/ebpf-leases.manifest.json`,
    datasetPath: `/mocking_data/`,
    wsEndpoint: `ws://localhost:${8080 + i}/ws/heartbeats`,
    apiBaseUrl: `http://localhost:${8080 + i}`,
  }));

describe('ClusterSelector', () => {
  it('renders nothing when only one cluster is configured', () => {
    const { container } = render(
      <ClusterSelector clusters={makeClusters(1)} selectedIndex={0} onSelect={jest.fn()} />,
    );
    expect(container.innerHTML).toBe('');
  });

  it('renders tabs when multiple clusters are configured', () => {
    render(
      <ClusterSelector clusters={makeClusters(2)} selectedIndex={0} onSelect={jest.fn()} />,
    );
    const tabs = screen.getAllByRole('tab');
    expect(tabs).toHaveLength(2);
    expect(tabs[0]).toHaveTextContent('cluster-0');
    expect(tabs[1]).toHaveTextContent('cluster-1');
  });

  it('marks the selected cluster tab as aria-selected', () => {
    render(
      <ClusterSelector clusters={makeClusters(3)} selectedIndex={1} onSelect={jest.fn()} />,
    );
    const tabs = screen.getAllByRole('tab');
    expect(tabs[0]).toHaveAttribute('aria-selected', 'false');
    expect(tabs[1]).toHaveAttribute('aria-selected', 'true');
    expect(tabs[2]).toHaveAttribute('aria-selected', 'false');
  });

  it('calls onSelect with the correct index when a tab is clicked', () => {
    const onSelect = jest.fn();
    render(
      <ClusterSelector clusters={makeClusters(3)} selectedIndex={0} onSelect={onSelect} />,
    );
    fireEvent.click(screen.getByText('cluster-2'));
    expect(onSelect).toHaveBeenCalledWith(2);
  });

  it('supports keyboard navigation with ArrowRight', () => {
    const onSelect = jest.fn();
    render(
      <ClusterSelector clusters={makeClusters(3)} selectedIndex={0} onSelect={onSelect} />,
    );
    const firstTab = screen.getAllByRole('tab')[0];
    fireEvent.keyDown(firstTab, { key: 'ArrowRight' });
    expect(onSelect).toHaveBeenCalledWith(1);
  });

  it('supports keyboard navigation with ArrowLeft (wraps around)', () => {
    const onSelect = jest.fn();
    render(
      <ClusterSelector clusters={makeClusters(3)} selectedIndex={0} onSelect={onSelect} />,
    );
    const firstTab = screen.getAllByRole('tab')[0];
    fireEvent.keyDown(firstTab, { key: 'ArrowLeft' });
    expect(onSelect).toHaveBeenCalledWith(2); // wraps to last
  });

  it('has proper ARIA labels on each tab', () => {
    render(
      <ClusterSelector clusters={makeClusters(2)} selectedIndex={0} onSelect={jest.fn()} />,
    );
    expect(screen.getByLabelText('View cluster cluster-0')).toBeInTheDocument();
    expect(screen.getByLabelText('View cluster cluster-1')).toBeInTheDocument();
  });

  it('renders a nav with aria-label "Cluster selector"', () => {
    render(
      <ClusterSelector clusters={makeClusters(2)} selectedIndex={0} onSelect={jest.fn()} />,
    );
    expect(screen.getByRole('navigation', { name: /cluster selector/i })).toBeInTheDocument();
  });
});
