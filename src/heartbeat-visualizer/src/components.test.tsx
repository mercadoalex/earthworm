import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import App from './App';
import Footer from './Footer';
import ChartControls from './ChartControls';
import HeartbeatChart from './HeartbeatChart';
import { ViewProvider } from './contexts/ViewContext';
import type { LeasesByNamespace } from './types/heartbeat';

// Mock hooks used by HeartbeatChart
jest.mock('./hooks/useHeartbeatData', () => ({
  useHeartbeatData: () => ({
    leasesData: {
      'test-ns': [
        { x: 0, y: 1000000 },
        { x: 1, y: 1005000 },
        { x: 2, y: 1010000 },
      ],
    },
    currentHeartbeat: 1010000,
    step: 'animate',
    chartData: [
      { timestamp: 1000000, 'test-ns': 1000000 },
      { timestamp: 1005000, 'test-ns': 1005000 },
      { timestamp: 1010000, 'test-ns': 1010000 },
    ],
    namespaces: ['test-ns'],
    currentFileIdx: 0,
  }),
}));

jest.mock('./hooks/useEbpfData', () => ({
  useEbpfData: () => ({
    showEbpf: false,
    toggleEbpf: jest.fn(),
    clearEbpfData: jest.fn(),
    restoreEbpfData: jest.fn(),
    ebpfMarkers: [],
  }),
}));

jest.mock('./hooks/useWebSocket', () => ({
  useWebSocket: () => ({
    status: 'connected',
    lastMessage: null,
  }),
}));

// Mock fetch globally to prevent real network calls
beforeEach(() => {
  global.fetch = jest.fn(() =>
    Promise.resolve({
      ok: true,
      json: () => Promise.resolve([]),
    }),
  ) as jest.Mock;

  // Mock WebSocket
  (global as any).WebSocket = jest.fn(() => ({
    close: jest.fn(),
    send: jest.fn(),
    onopen: null,
    onclose: null,
    onmessage: null,
    onerror: null,
    readyState: 0,
  }));

  // Mock ResizeObserver
  (global as any).ResizeObserver = jest.fn(() => ({
    observe: jest.fn(),
    unobserve: jest.fn(),
    disconnect: jest.fn(),
  }));
});

afterEach(() => {
  jest.restoreAllMocks();
});

// --- App rendering tests ---
// Requirements: 12.7, 9.1, 9.2
describe('App component', () => {
  it('renders the header with "Heartbeat Visualizer"', () => {
    render(<App />);
    expect(screen.getByText(/Heartbeat Visualizer/i)).toBeInTheDocument();
  });

  it('renders semantic header element with role="banner"', () => {
    render(<App />);
    const header = screen.getByRole('banner');
    expect(header).toBeInTheDocument();
  });

  it('renders the App container with header and footer', () => {
    render(<App />);
    // Verify header is present
    expect(screen.getByRole('banner')).toBeInTheDocument();
    // Verify footer is present
    expect(screen.getByRole('contentinfo')).toBeInTheDocument();
    // Verify heading text
    expect(screen.getByText(/Heartbeat Visualizer/i)).toBeInTheDocument();
  });

  it('renders the footer', () => {
    render(<App />);
    expect(screen.getByRole('contentinfo')).toBeInTheDocument();
  });
});

// --- Footer rendering tests ---
describe('Footer component', () => {
  it('renders footer with role="contentinfo"', () => {
    render(<Footer />);
    const footer = screen.getByRole('contentinfo');
    expect(footer).toBeInTheDocument();
  });

  it('renders the author credit', () => {
    render(<Footer />);
    expect(screen.getByText(/Made with/i)).toBeInTheDocument();
  });

  it('renders the GitHub link', () => {
    render(<Footer />);
    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('href', 'https://github.com/mercadoalex/earthworm');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });
});


// --- ChartControls rendering tests ---
// Requirements: 12.6, 9.2
describe('ChartControls component', () => {
  const mockLeasesData: LeasesByNamespace = {
    'test-ns': [
      { x: 0, y: 1000000 },
      { x: 1, y: 1005000 },
    ],
  };

  const defaultProps = {
    noise: false,
    onNoiseToggle: jest.fn(),
    language: 'en' as string,
    onLanguageToggle: jest.fn(),
    onRestart: jest.fn(),
    timestamp: 1000000,
    leasesData: mockLeasesData,
    showEbpf: false,
    onEbpfCorrelate: jest.fn(),
    clearEbpfData: jest.fn(),
    restoreEbpfData: jest.fn(),
  };

  it('renders the chart controls navigation', () => {
    render(<ChartControls {...defaultProps} />);
    const nav = screen.getByRole('navigation', { name: /chart controls/i });
    expect(nav).toBeInTheDocument();
  });

  it('renders the sound toggle button with ARIA label', () => {
    render(<ChartControls {...defaultProps} />);
    const soundBtn = screen.getByLabelText(/enable sound|disable sound/i);
    expect(soundBtn).toBeInTheDocument();
  });

  it('renders the language toggle button with ARIA label', () => {
    render(<ChartControls {...defaultProps} />);
    const langBtn = screen.getByLabelText(/switch to spanish|switch to english/i);
    expect(langBtn).toBeInTheDocument();
  });

  it('renders the restart button with ARIA label', () => {
    render(<ChartControls {...defaultProps} />);
    const restartBtn = screen.getByLabelText(/restart heartbeat animation/i);
    expect(restartBtn).toBeInTheDocument();
  });

  it('renders the eBPF correlation button with ARIA label', () => {
    render(<ChartControls {...defaultProps} />);
    const ebpfBtn = screen.getByLabelText(/show ebpf correlation|hide ebpf correlation/i);
    expect(ebpfBtn).toBeInTheDocument();
  });

  it('calls onNoiseToggle when sound button is clicked', () => {
    const onNoiseToggle = jest.fn();
    render(<ChartControls {...defaultProps} onNoiseToggle={onNoiseToggle} />);
    const soundBtn = screen.getByLabelText(/enable sound|disable sound/i);
    fireEvent.click(soundBtn);
    expect(onNoiseToggle).toHaveBeenCalledTimes(1);
  });

  it('calls onRestart when restart button is clicked', () => {
    const onRestart = jest.fn();
    render(<ChartControls {...defaultProps} onRestart={onRestart} />);
    const restartBtn = screen.getByLabelText(/restart heartbeat animation/i);
    fireEvent.click(restartBtn);
    expect(onRestart).toHaveBeenCalledTimes(1);
  });

  it('calls onLanguageToggle when language button is clicked', () => {
    const onLanguageToggle = jest.fn();
    render(<ChartControls {...defaultProps} onLanguageToggle={onLanguageToggle} />);
    const langBtn = screen.getByLabelText(/switch to spanish/i);
    fireEvent.click(langBtn);
    expect(onLanguageToggle).toHaveBeenCalledTimes(1);
  });

  it('renders anomaly summary section', () => {
    render(<ChartControls {...defaultProps} />);
    expect(screen.getByText(/Anomaly Summary/i)).toBeInTheDocument();
  });
});


// --- HeartbeatChart rendering tests ---
// Requirements: 12.5
describe('HeartbeatChart component', () => {
  it('renders without crashing with mock data', () => {
    const { container } = render(
      <ViewProvider>
        <HeartbeatChart />
      </ViewProvider>,
    );
    expect(container).toBeTruthy();
  });

  it('renders the chart controls section', () => {
    render(
      <ViewProvider>
        <HeartbeatChart />
      </ViewProvider>,
    );
    const nav = screen.getByRole('navigation', { name: /chart controls/i });
    expect(nav).toBeInTheDocument();
  });

  it('renders the view selector tabs', () => {
    render(
      <ViewProvider>
        <HeartbeatChart />
      </ViewProvider>,
    );
    // ViewSelector renders tabs inside a nav with aria-label
    const viewNav = screen.getByRole('navigation', { name: /view selector/i });
    expect(viewNav).toBeInTheDocument();
  });

  it('does not render connection status banner when connected', () => {
    render(
      <ViewProvider>
        <HeartbeatChart />
      </ViewProvider>,
    );
    // ConnectionStatus returns null when status is 'connected'
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });
});
