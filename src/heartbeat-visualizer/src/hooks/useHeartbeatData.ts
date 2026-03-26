import { useEffect, useState, useCallback, useMemo } from 'react';
import { config } from '../config';
import { transformLeasesForChart } from '../services/dataService';
import type { LeasesByNamespace, ChartDataPoint } from '../types/heartbeat';

export type HeartbeatStep = 'sync' | 'animate' | 'pause' | 'nodata';

export interface UseHeartbeatDataReturn {
  manifest: string[];
  currentFileIdx: number;
  leasesData: LeasesByNamespace | null;
  currentHeartbeat: number;
  step: HeartbeatStep;
  chartData: ChartDataPoint[];
  namespaces: string[];
  restart: () => void;
}

export function useHeartbeatData(
  manifestUrlOverride?: string,
  datasetPathOverride?: string,
): UseHeartbeatDataReturn {
  const manifestUrl = manifestUrlOverride ?? config.manifestUrl;
  const datasetPath = datasetPathOverride ?? config.datasetPath;

  const [manifest, setManifest] = useState<string[]>([]);
  const [currentFileIdx, setCurrentFileIdx] = useState(0);
  const [leasesData, setLeasesData] = useState<LeasesByNamespace | null>(null);
  const [currentHeartbeat, setCurrentHeartbeat] = useState(0);
  const [step, setStep] = useState<HeartbeatStep>('sync');

  // 1. Fetch heartbeat manifest on mount or when URL changes
  useEffect(() => {
    setCurrentFileIdx(0);
    setCurrentHeartbeat(0);
    setStep('sync');
    fetch(manifestUrl)
      .then((res) => res.json())
      .then((files: string[]) => {
        const sorted = files.sort((a, b) => {
          if (a === 'leases.json') return -1;
          if (b === 'leases.json') return 1;
          return a.localeCompare(b);
        });
        setManifest(sorted);
        if (!files || files.length === 0) setStep('nodata');
      })
      .catch(() => setStep('nodata'));
  }, [manifestUrl]);

  // 2. When in 'sync' step, load heartbeat data
  useEffect(() => {
    if (step !== 'sync' || manifest.length === 0) return;
    setCurrentHeartbeat(0);
    const file = manifest[currentFileIdx];
    fetch(`${datasetPath}${file}`)
      .then((res) => res.json())
      .then((data: LeasesByNamespace) => {
        setLeasesData(data);
        setTimeout(() => setStep('animate'), 1000);
      })
      .catch(() => {
        setLeasesData(null);
        setStep('nodata');
      });
  }, [step, manifest, currentFileIdx, datasetPath]);

  // 3. Animate heartbeats
  useEffect(() => {
    if (step !== 'animate' || !leasesData) return;
    const totalHeartbeats = Math.max(
      ...Object.values(leasesData).map((nsArr) => nsArr.length)
    );
    if (currentHeartbeat < totalHeartbeats - 1) {
      const timer = setTimeout(() => {
        setCurrentHeartbeat((hb) => hb + 1);
      }, config.heartbeatInterval);
      return () => clearTimeout(timer);
    } else {
      const timer = setTimeout(() => setStep('pause'), 2000);
      return () => clearTimeout(timer);
    }
  }, [step, leasesData, currentHeartbeat]);

  // 4. In 'pause' step, go to next dataset or finish
  useEffect(() => {
    if (step !== 'pause') return;
    if (currentFileIdx < manifest.length - 1) {
      const timer = setTimeout(() => {
        setCurrentFileIdx((idx) => idx + 1);
        setStep('sync');
      }, 1000);
      return () => clearTimeout(timer);
    } else {
      const timer = setTimeout(() => setStep('nodata'), 1200);
      return () => clearTimeout(timer);
    }
  }, [step, currentFileIdx, manifest.length]);

  // Prepare chart data (memoized to avoid recomputation on unrelated re-renders)
  const namespaces = useMemo(
    () => (leasesData ? Object.keys(leasesData) : []),
    [leasesData],
  );
  const chartData = useMemo(
    () => (leasesData ? transformLeasesForChart(leasesData, currentHeartbeat + 1) : []),
    [leasesData, currentHeartbeat],
  );

  const restart = useCallback(() => {
    setCurrentFileIdx(0);
    setCurrentHeartbeat(0);
    setStep('sync');
  }, []);

  return {
    manifest,
    currentFileIdx,
    leasesData,
    currentHeartbeat,
    step,
    chartData,
    namespaces,
    restart,
  };
}
