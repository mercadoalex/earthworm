import React, { createContext, useContext, useState, type ReactNode } from 'react';
import type { ViewType } from '../types/heartbeat';

export interface ViewContextValue {
  activeView: ViewType;
  setActiveView: (view: ViewType) => void;
  xDomain: [number, number] | null;
  setXDomain: (domain: [number, number] | null) => void;
}

const ViewContext = createContext<ViewContextValue | undefined>(undefined);

export const ViewProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [activeView, setActiveView] = useState<ViewType>('line');
  const [xDomain, setXDomain] = useState<[number, number] | null>(null);

  return (
    <ViewContext.Provider value={{ activeView, setActiveView, xDomain, setXDomain }}>
      {children}
    </ViewContext.Provider>
  );
};

export function useViewContext(): ViewContextValue {
  const ctx = useContext(ViewContext);
  if (!ctx) {
    throw new Error('useViewContext must be used within a ViewProvider');
  }
  return ctx;
}

export default ViewContext;
