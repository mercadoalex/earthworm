import React from 'react';
import './App.css';

const Footer: React.FC = () => {
  return (
    <footer className="App-footer" role="contentinfo" style={{ background: '#222', color: '#e0e0e0', textAlign: 'center' }}>
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: '1rem', fontSize: '0.6rem', color: '#e0e0e0' }}>
        <span>Made with <span style={{ color: 'red' }}>❤️</span> by Alex</span>
        <span>
          <a href="https://github.com/mercadoalex/earthworm" target="_blank" rel="noopener noreferrer" style={{ color: '#90caf9', textDecoration: 'none' }}>
            https://github.com/mercadoalex/earthworm
          </a>
        </span>
      </div>
    </footer>
  );
};

export default Footer;
