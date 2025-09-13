import logo from './logo.svg';
import './App.css';
import HeartbeatChart from './HeartbeatChart';
import Footer from './Footer';

function App() {
  return (
    <div className="App">
      <header className="App-header">
        <h1>Heartbeat Visualizer</h1>
        <HeartbeatChart />
      </header>
      <Footer />
    </div>
  );
}

export default App;
