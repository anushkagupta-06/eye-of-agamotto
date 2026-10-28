import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import type { Anomaly, LogRecord, PipelineComponentStatus, REDMetric, Span } from '../types/telemetry';
import { PIPELINE_STATUS, SYSTEM_PROFILES } from '../api/telemetryClient';
import {
  fetchLiveREDMetrics,
  fetchLiveTraces,
  fetchLiveLogs,
  fetchLiveAnomalies,
  emitLiveOTLPTrace,
} from '../api/liveTelemetryClient';

interface TelemetryContextType {
  tenant: string;
  timeRange: string;
  setTimeRange: (range: string) => void;
  autoRefresh: boolean;
  setAutoRefresh: (val: boolean) => void;
  searchQuery: string;
  setSearchQuery: (query: string) => void;
  apiKey: string;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
  isStale: boolean;
  mockMode: boolean;
  setMockMode: (val: boolean) => void;
  systemProfileName: string;
  redMetrics: REDMetric[];
  anomalies: Anomaly[];
  spans: Span[];
  logs: LogRecord[];
  pipelineStatus: PipelineComponentStatus[];
  stats: {
    totalSpansPerSec: number;
    totalLogsPerSec: number;
    avgP99Ms: number;
    errorRate: number;
    activeServicesCount: number;
  };
  refreshData: () => void;
  emitRealServiceTrace: (serviceName: string, operation?: string, isError?: boolean) => Promise<{ traceId: string; spanId: string }>;
}

const TelemetryContext = createContext<TelemetryContextType | undefined>(undefined);

export const TelemetryProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const tenant = 'demo';
  const apiKey = 'eoa_cffdedbba2a3d4f0ecb4706923ae02d5d4b2ac1a0c03ff44';
  const isAuthenticated = true;

  const [timeRange, setTimeRange] = useState<string>('15m');
  const [autoRefresh, setAutoRefresh] = useState<boolean>(true);
  const [searchQuery, setSearchQuery] = useState<string>('');

  const [mockMode, setMockModeState] = useState<boolean>(() => {
    return localStorage.getItem('eoa_mock_mode') === 'true';
  });

  const setMockMode = (val: boolean) => {
    setMockModeState(val);
    localStorage.setItem('eoa_mock_mode', String(val));
  };

  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [isStale, setIsStale] = useState<boolean>(false);

  const [redMetrics, setRedMetrics] = useState<REDMetric[]>([]);
  const [anomalies, setAnomalies] = useState<Anomaly[]>([]);
  const [spans, setSpans] = useState<Span[]>([]);
  const [logs, setLogs] = useState<LogRecord[]>([]);
  const [pipelineStatus] = useState<PipelineComponentStatus[]>(PIPELINE_STATUS);

  const [tick, setTick] = useState(0);

  const refreshData = useCallback(() => {
    setTick((prev) => prev + 1);
  }, []);

  const emitRealServiceTrace = async (
    serviceName: string,
    operation: string = 'POST /v1/process',
    isError: boolean = false
  ) => {
    const cleanName = serviceName.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '-');
    const res = await emitLiveOTLPTrace(cleanName, operation, isError);

    // Optimistically update local state so the service immediately shows on the UI
    const nowIso = new Date().toISOString();
    const optimisticMetric: REDMetric = {
      serviceName: cleanName,
      operation: operation,
      callsPerMin: 120,
      errorCount: isError ? 12 : 0,
      errorRate: isError ? 10.0 : 0.0,
      p50Ms: 14,
      p95Ms: 45,
      p99Ms: 85,
      status: isError ? 'CRITICAL' : 'HEALTHY',
    };

    const optimisticSpan: Span = {
      tenantId: tenant,
      traceId: res.traceId,
      spanId: res.spanId,
      parentSpanId: '',
      serviceName: cleanName,
      name: operation,
      kind: 'SERVER',
      startTime: nowIso,
      durationNs: 45000000,
      statusCode: isError ? 'ERROR' : 'OK',
      httpStatusCode: isError ? 500 : 200,
      attributes: { 'http.method': 'POST' },
      resourceAttributes: { 'service.name': cleanName },
      ingestedAt: nowIso,
    };

    setRedMetrics((prev) => [...prev.filter((m) => m.serviceName !== cleanName), optimisticMetric]);
    setSpans((prev) => [optimisticSpan, ...prev]);

    // Also trigger background refresh to sync with Query API
    setTimeout(() => {
      refreshData();
    }, 1200);

    return res;
  };

  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(() => {
      setTick((prev) => prev + 1);
    }, 5000);
    return () => clearInterval(interval);
  }, [autoRefresh]);

  // Load telemetry from Query API (or explicitly requested mock fixtures)
  useEffect(() => {
    let isMounted = true;

    const loadData = async () => {
      if (mockMode) {
        // Explicit Mock Mode requested by user
        const fixture = SYSTEM_PROFILES['GENERAL'];
        if (isMounted) {
          setRedMetrics(fixture.redMetrics);
          setAnomalies(fixture.anomalies);
          setSpans(fixture.spans);
          setLogs(fixture.logs);
          setIsLoading(false);
          setError(null);
          setIsStale(false);
        }
        return;
      }

      setIsLoading((prev) => (redMetrics.length === 0 ? true : prev));

      try {
        const [liveRED, liveSpans, liveLogs, liveAnomalies] = await Promise.all([
          fetchLiveREDMetrics(timeRange),
          fetchLiveTraces(timeRange),
          fetchLiveLogs({ range: timeRange }),
          fetchLiveAnomalies('24h'),
        ]);

        if (isMounted) {
          setRedMetrics(liveRED);
          setSpans(liveSpans);
          setLogs(liveLogs);
          setAnomalies(liveAnomalies);
          setError(null);
          setIsStale(false);
          setIsLoading(false);
        }
      } catch (err: any) {
        if (isMounted) {
          console.warn('Query API sync error:', err);
          setError(err?.message || 'Failed to connect to Query API backend');
          setIsStale(true);
          setIsLoading(false);
        }
      }
    };

    loadData();

    return () => {
      isMounted = false;
    };
  }, [tick, timeRange, mockMode]);

  // Derived high-level stats
  const totalCalls = redMetrics.reduce((acc, m) => acc + m.callsPerMin, 0);
  const totalErrors = redMetrics.reduce((acc, m) => acc + m.errorCount, 0);
  const errorRate = totalCalls > 0 ? (totalErrors / totalCalls) * 100 : 0;
  const avgP99Ms = redMetrics.length > 0 ? Math.round(redMetrics.reduce((acc, m) => acc + m.p99Ms, 0) / redMetrics.length) : 0;
  const totalSpansPerSec = totalCalls > 0 ? Math.round(totalCalls / 60) : 0;
  const totalLogsPerSec = logs.length > 0 ? Math.round(logs.length / 10) : 0;

  return (
    <TelemetryContext.Provider
      value={{
        tenant,
        timeRange,
        setTimeRange,
        autoRefresh,
        setAutoRefresh,
        searchQuery,
        setSearchQuery,
        apiKey,
        isAuthenticated,
        isLoading,
        error,
        isStale,
        mockMode,
        setMockMode,
        systemProfileName: mockMode ? 'Demo Microservices (Mock Mode)' : 'Live Telemetry (Single-Tenant)',
        redMetrics,
        anomalies,
        spans,
        logs,
        pipelineStatus,
        stats: {
          totalSpansPerSec,
          totalLogsPerSec,
          avgP99Ms,
          errorRate: Number(errorRate.toFixed(2)),
          activeServicesCount: redMetrics.length,
        },
        refreshData,
        emitRealServiceTrace,
      }}
    >
      {children}
    </TelemetryContext.Provider>
  );
};

// eslint-disable-next-line react-refresh/only-export-components
export const useTelemetry = (): TelemetryContextType => {
  const context = useContext(TelemetryContext);
  if (!context) {
    throw new Error('useTelemetry must be used within a TelemetryProvider');
  }
  return context;
};
