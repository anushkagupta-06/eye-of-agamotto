import type { Anomaly, LogRecord, REDMetric, Span, ServiceEdge } from '../types/telemetry';
import { SYSTEM_PROFILES, type SystemProfileType } from './telemetryClient';

const QUERY_API_BASE = '/api/query/api/v1';

/**
 * Fetch real-time RED Metrics from QueryAPI / ClickHouse, falling back to profile defaults.
 */
export async function fetchLiveREDMetrics(tenant: string, profile: SystemProfileType): Promise<REDMetric[]> {
  try {
    const res = await fetch(`${QUERY_API_BASE}/metrics/red?tenant=${encodeURIComponent(tenant)}`, {
      headers: { 'Accept': 'application/json' },
    });
    if (res.ok) {
      const json = await res.json();
      if (Array.isArray(json?.data) && json.data.length > 0) {
        return json.data;
      }
    }
  } catch (err) {
    // Backend offline or unreachable; fall back gracefully
  }
  return SYSTEM_PROFILES[profile]?.redMetrics || [];
}

/**
 * Fetch real-time Traces from QueryAPI / ClickHouse.
 */
export async function fetchLiveTraces(tenant: string, profile: SystemProfileType): Promise<Span[]> {
  try {
    const res = await fetch(`${QUERY_API_BASE}/traces?tenant=${encodeURIComponent(tenant)}&limit=50`, {
      headers: { 'Accept': 'application/json' },
    });
    if (res.ok) {
      const json = await res.json();
      if (Array.isArray(json?.data) && json.data.length > 0) {
        // Fetch detailed spans for the most active traces
        const spansList: Span[] = [];
        for (const summary of json.data.slice(0, 5)) {
          try {
            const traceRes = await fetch(`${QUERY_API_BASE}/traces/${summary.traceId}?tenant=${encodeURIComponent(tenant)}`);
            if (traceRes.ok) {
              const traceData = await traceRes.json();
              if (Array.isArray(traceData?.spans)) {
                spansList.push(...traceData.spans);
              }
            }
          } catch {
            // ignore individual trace failure
          }
        }
        if (spansList.length > 0) {
          return spansList;
        }
      }
    }
  } catch (err) {
    // Backend offline; fall back gracefully
  }
  return SYSTEM_PROFILES[profile]?.spans || [];
}

/**
 * Fetch real-time Logs from QueryAPI / ClickHouse.
 */
export async function fetchLiveLogs(tenant: string, profile: SystemProfileType): Promise<LogRecord[]> {
  try {
    const res = await fetch(`${QUERY_API_BASE}/logs?tenant=${encodeURIComponent(tenant)}&limit=100`, {
      headers: { 'Accept': 'application/json' },
    });
    if (res.ok) {
      const json = await res.json();
      if (Array.isArray(json?.data) && json.data.length > 0) {
        return json.data;
      }
    }
  } catch (err) {
    // Backend offline; fall back gracefully
  }
  return SYSTEM_PROFILES[profile]?.logs || [];
}

/**
 * Fetch real-time Anomalies from QueryAPI / ClickHouse.
 */
export async function fetchLiveAnomalies(tenant: string, profile: SystemProfileType): Promise<Anomaly[]> {
  try {
    const res = await fetch(`${QUERY_API_BASE}/anomalies?tenant=${encodeURIComponent(tenant)}&range=24h`, {
      headers: { 'Accept': 'application/json' },
    });
    if (res.ok) {
      const json = await res.json();
      if (Array.isArray(json?.data) && json.data.length > 0) {
        return json.data;
      }
    }
  } catch (err) {
    // Backend offline; fall back gracefully
  }
  return SYSTEM_PROFILES[profile]?.anomalies || [];
}

/**
 * Fetch real-time Service Topology from QueryAPI.
 */
export async function fetchLiveTopology(tenant: string): Promise<ServiceEdge[]> {
  try {
    const res = await fetch(`${QUERY_API_BASE}/topology?tenant=${encodeURIComponent(tenant)}`, {
      headers: { 'Accept': 'application/json' },
    });
    if (res.ok) {
      const json = await res.json();
      if (Array.isArray(json?.edges) && json.edges.length > 0) {
        return json.edges.map((e: any) => ({
          caller: e.caller,
          callee: e.callee,
          calls: e.callCount,
          errors: e.errorCount,
          p95LatencyMs: e.p95Ms,
        }));
      }
    }
  } catch (err) {
    // Fall back
  }
  return [];
}
