import { render, screen } from '@testing-library/react';
import App from './App';

test('renders heartbeat visualizer heading', () => {
  render(<App />);
  const heading = screen.getByText(/Heartbeat Visualizer/i);
  expect(heading).toBeInTheDocument();
});
