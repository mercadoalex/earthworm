import React from 'react';
import './App.css';

function Footer() {
  return (
    <footer className="App-footer" style={{ background: '#222', color: '#fff', textAlign: 'center' }}>
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: '1rem', fontSize: '0.6rem', color: '#fff' }}>
        <span>Made with <span style={{ color: 'red' }}>❤️</span> by Alex</span>
        <span>
          <a href="https://github.com/mercadoalex/earthworm" target="_blank" rel="noopener noreferrer" style={{ color: '#fff', textDecoration: 'none' }}>
            https://github.com/mercadoalex/earthworm
          </a>
        </span>
      </div>
    </footer>
  );
}

export default Footer;