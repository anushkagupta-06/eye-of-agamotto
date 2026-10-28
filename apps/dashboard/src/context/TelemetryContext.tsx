import React, { createContext, useContext, useState, useEffect } from 'react';
import type { Anomaly, LogRecord, PipelineComponentStatus, REDMetric, Span } from '../types/telemetry';
import { PIPELINE_STATUS, SYSTEM_PROFILES, type SystemProfileType } from '../api/telemetryClient';

interface TelemetryContextType {
  tenant: string;
  setTenant: (tenant: string) => void;
  systemProfile: SystemProfileType;
  setSystemProfile: (profile: SystemProfileType) => void;
  systemProfileName: string;
  systemProfileDescription: string;
  timeRange: string;
  setTimeRange: (range: string) => void;
  autoRefresh: boolean;
  setAutoRefresh: (val: boolean) => void;
  searchQuery: string;
  setSearchQuery: (query: string) => void;
  apiKey: string;
  isAuthenticated: boolean;
  login: (tenantId: string, key: string) => boolean;
  logout: () => void;
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
  addCustomServiceTelemetry: (serviceName: string) => void;
}

const TelemetryContext = createContext<TelemetryContextType | undefined>(undefined);

export const TelemetryProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [tenant, setTenant] = useState<string>(() => {
    return localStorage.getItem('eoa_auth_tenant') || 'demo';
  });
  const [apiKey, setApiKey] = useState<string>(() => {
    return localStorage.getItem('eoa_auth_key') || 'eoa_cffdedbba2a3d4f0ecb4706923ae02d5d4b2ac1a0c03ff44';
  });
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(() => {
    return localStorage.getItem('eoa_is_authenticated') === 'true';
  });

  const [systemProfile, setSystemProfileState] = useState<SystemProfileType>(() => {
    return (localStorage.getItem('eoa_system_profile') as SystemProfileType) || 'GENERAL';
  });

  const [timeRange, setTimeRange] = useState<string>('15m');
  const [autoRefresh, setAutoRefresh] = useState<boolean>(true);
  const [searchQuery, setSearchQuery] = useState<string>('');

  const [redMetrics, setRedMetrics] = useState<REDMetric[]>(SYSTEM_PROFILES[systemProfile].redMetrics);
  const [anomalies, setAnomalies] = useState<Anomaly[]>(SYSTEM_PROFILES[systemProfile].anomalies);
  const [spans, setSpans] = useState<Span[]>(SYSTEM_PROFILES[systemProfile].spans);
  const [logs, setLogs] = useState<LogRecord[]>(SYSTEM_PROFILES[systemProfile].logs);
  const [pipelineStatus] = useState<PipelineComponentStatus[]>(PIPELINE_STATUS);

  const [tick, setTick] = useState(0);

  const setSystemProfile = (profile: SystemProfileType) => {
    setSystemProfileState(profile);
    localStorage.setItem('eoa_system_profile', profile);
    const data = SYSTEM_PROFILES[profile];
    setRedMetrics(data.redMetrics);
    setAnomalies(data.anomalies);
    setSpans(data.spans);
    setLogs(data.logs);
  };

  const addCustomServiceTelemetry = (customName: string) => {
    const cleanName = customName.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '-');
    if (!cleanName) return;

    const newMetric: REDMetric = {
      serviceName: cleanName,
      operation: 'POST /v1/process',
      callsPerMin: 12500,
      errorCount: 12,
      errorRate: 0.1,
      p50Ms: 12,
      p95Ms: 45,
      p99Ms: 90,
      status: 'HEALTHY',
    };

    const gatewayService = redMetrics.find((m) => /gateway|ingress|router/i.test(m.serviceName))?.serviceName || 'api-gateway';
    const traceId = 'c' + Math.random().toString(16).slice(2, 17) + Math.random().toString(16).slice(2, 17);
    const rootSpanId = 'r' + Math.random().toString(16).slice(2, 17);
    const childSpanId = 's' + Math.random().toString(16).slice(2, 17);

    const now = new Date();
    const childTime = new Date(now.getTime() + 10);

    const rootSpan: Span = {
      tenantId: tenant,
      traceId,
      spanId: rootSpanId,
      parentSpanId: '',
      serviceName: gatewayService,
      name: `PROXY /api/v1/${cleanName}`,
      kind: 'SERVER',
      startTime: now.toISOString(),
      durationNs: 65000000,
      statusCode: 'OK',
      httpStatusCode: 200,
      attributes: { 'http.route': `/api/v1/${cleanName}` },
      resourceAttributes: { 'service.name': gatewayService },
      ingestedAt: now.toISOString(),
    };

    const childSpan: Span = {
      tenantId: tenant,
      traceId,
      spanId: childSpanId,
      parentSpanId: rootSpanId,
      serviceName: cleanName,
      name: 'POST /v1/process',
      kind: 'SERVER',
      startTime: childTime.toISOString(),
      durationNs: 45000000,
      statusCode: 'OK',
      httpStatusCode: 200,
      attributes: { 'custom.registered': 'true' },
      resourceAttributes: { 'service.name': cleanName, 'environment': 'custom' },
      ingestedAt: childTime.toISOString(),
    };

    const newLog: LogRecord = {
      tenantId: tenant,
      timestamp: now.toISOString(),
      serviceName: cleanName,
      severityText: 'INFO',
      severityNumber: 9,
      body: `Custom microservice ${cleanName} registered and actively emitting OpenTelemetry spans`,
      traceId: traceId,
      spanId: childSpanId,
      attributes: { 'status': 'READY' },
      resourceAttributes: { 'service.name': cleanName },
    };

    setRedMetrics((prev) => [...prev.filter((m) => m.serviceName !== cleanName), newMetric]);
    setSpans((prev) => [childSpan, rootSpan, ...prev]);
    setLogs((prev) => [newLog, ...prev]);
  };

  const login = (tenantId: string, key: string): boolean => {
    if (
      (tenantId.trim().toLowerCase() === 'demo' && key.trim() === 'eoa_cffdedbba2a3d4f0ecb4706923ae02d5d4b2ac1a0c03ff44') ||
      (tenantId.trim().length > 0 && key.startsWith('eoa_'))
    ) {
      setTenant(tenantId.trim());
      setApiKey(key.trim());
      setIsAuthenticated(true);
      localStorage.setItem('eoa_is_authenticated', 'true');
      localStorage.setItem('eoa_auth_tenant', tenantId.trim());
      localStorage.setItem('eoa_auth_key', key.trim());
      return true;
    }
    return false;
  };

  const logout = () => {
    setIsAuthenticated(false);
    localStorage.removeItem('eoa_is_authenticated');
  };

  const refreshData = () => {
    setTick((prev) => prev + 1);
  };

  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(() => {
      setTick((prev) => prev + 1);
    }, 5000);
    return () => clearInterval(interval);
  }, [autoRefresh]);

  // Derived high-level stats
  const totalCalls = redMetrics.reduce((acc, m) => acc + m.callsPerMin, 0);
  const totalErrors = redMetrics.reduce((acc, m) => acc + m.errorCount, 0);
  const errorRate = totalCalls > 0 ? (totalErrors / totalCalls) * 100 : 0;
  const avgP99Ms = Math.round(redMetrics.reduce((acc, m) => acc + m.p99Ms, 0) / redMetrics.length);
  const totalSpansPerSec = Math.round(totalCalls / 60) + (tick % 7);
  const totalLogsPerSec = Math.round(totalSpansPerSec * 1.8) + (tick % 11);

  return (
    <TelemetryContext.Provider
      value={{
        tenant,
        setTenant,
        systemProfile,
        setSystemProfile,
        systemProfileName: SYSTEM_PROFILES[systemProfile].name,
        systemProfileDescription: SYSTEM_PROFILES[systemProfile].description,
        timeRange,
        setTimeRange,
        autoRefresh,
        setAutoRefresh,
        searchQuery,
        setSearchQuery,
        apiKey,
        isAuthenticated,
        login,
        logout,
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
        addCustomServiceTelemetry,
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
