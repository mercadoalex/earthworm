import { useEffect, useState, useCallback } from 'react';
import { config } from '../config';
import type { EbpfEvent, EbpfMarker, ChartDataPoint } from '../types/heartbeat';

export interface UseEbpfDataReturn {
  ebpfData: EbpfEvent[];
  showEbpf: boolean;
  toggleEbpf: () => void;
  clearEbpfData: () => void;
  restoreEbpfData: () => void;
  getEbpfMarkers: (chartData: ChartDataPoint[], namespaces: string[]) => EbpfMarker[];
}

export function useEbpfData(
  currentFileIdx: number,
  step: string,
  ebpfManifestUrlOverride?: string,
  datasetPathOverride?: string,
): UseEbpfDataReturn {
  const ebpfManifestUrl = ebpfManifestUrlOverride ?? config.ebpfManifestUrl;
  const datasetPath = datasetPathOverride ?? config.datasetPath;

  const [ebpfManifest, setEbpfManifest] = useState<string[]>([]);
  const [ebpfData, setEbpfData] = useState<EbpfEvent[]>([]);
  const [showEbpf, setShowEbpf] = useState(true);
  const [cachedEbpfData, setCachedEbpfData] = useState<EbpfEvent[]>([]);

  // Fetch eBPF manifest on mount or when URL changes
  useEffect(() => {
    fetch(ebpfManifestUrl)
      .then((res) => res.json())
      .then((files: string[]) => setEbpfManifest(files))
      .catch(() => setEbpfManifest([]));
  }, [ebpfManifestUrl]);

  // Load eBPF data when syncing
  useEffect(() => {
    if (step !== 'sync' || ebpfManifest.length === 0) return;
    const ebpfFile = ebpfManifest[currentFileIdx] || ebpfManifest[0];
    fetch(`${datasetPath}${ebpfFile}`)
      .then((res) => res.json())
      .then((data: EbpfEvent[] | unknown) => {
        const arr = Array.isArray(data) ? data : [];
        setEbpfData(arr);
        setCachedEbpfData(arr);
      })
      .catch(() => {
        setEbpfData([]);
        setCachedEbpfData([]);
      });
    setShowEbpf(true);
  }, [step, currentFileIdx, ebpfManifest, datasetPath]);

  const toggleEbpf = useCallback(() => {
    setShowEbpf((v) => !v);
  }, []);

  const clearEbpfData = useCallback(() => {
    setEbpfData([]);
  }, []);

  const restoreEbpfData = useCallback(() => {
    setEbpfData(cachedEbpfData);
  }, [cachedEbpfData]);

  const getEbpfMarkers = useCallback(
    (chartData: ChartDataPoint[], namespaces: string[]): EbpfMarker[] => {
      if (!showEbpf || !ebpfData || ebpfData.length === 0) return [];
      const markers: EbpfMarker[] = [];
      chartData.forEach((point) => {
        namespaces.forEach((ns) => {
          const matchingEvents = ebpfData.filter(
            (event) =>
              event.namespace === ns &&
              Math.abs(event.timestamp - (point.timestamp || 0)) < 60000
          );
          matchingEvents.forEach((event, i) => {
            markers.push({
              x: point.timestamp || 0,
              y: (point[ns] as number) || 0,
              namespace: ns,
              event,
              offset: (i - Math.floor(matchingEvents.length / 2)) * 12,
            });
          });
        });
      });
      return markers;
    },
    [showEbpf, ebpfData]
  );

  return {
    ebpfData,
    showEbpf,
    toggleEbpf,
    clearEbpfData,
    restoreEbpfData,
    getEbpfMarkers,
  };
}
