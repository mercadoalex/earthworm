import React, { useState } from 'react';
import './App.css';
import HeartbeatChart from './HeartbeatChart';
import Footer from './Footer';
import ClusterSelector from './ClusterSelector';
import { clusters } from './config';
import { ViewProvider } from './contexts/ViewContext';

const App: React.FC = () => {
  const [selectedClusterIdx, setSelectedClusterIdx] = useState(0);
  const selectedCluster = clusters[selectedClusterIdx];

  return (
    <ViewProvider>
      <div className="App">
        <header className="App-header" role="banner">
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px', justifyContent: 'center' }}>
            <img
              src={`${process.env.PUBLIC_URL}/logo_transparente.png`}
              alt="Earthworm Logo"
              style={{ height: '48px', width: 'auto' }}
            />
            <h1 style={{ margin: 0 }}>Earthworm Observability</h1>
          </div>
          <ClusterSelector
            clusters={clusters}
            selectedIndex={selectedClusterIdx}
            onSelect={setSelectedClusterIdx}
          />
        </header>
        <main className="App-main" role="main">
          <HeartbeatChart cluster={selectedCluster} />
        </main>
        <Footer />
      </div>
    </ViewProvider>
  );
};

export default App;
