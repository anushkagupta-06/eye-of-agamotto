import type { Anomaly, LogRecord, REDMetric, Span, ServiceEdge } from '../types/telemetry';

const QUERY_API_BASE = '/api/query/api/v1';

/**
 * Fetch real-time RED Metrics from QueryAPI / ClickHouse.
 */
export async function fetchLiveREDMetrics(): Promise<REDMetric[]> {
  try {
    const res = await fetch(`${QUERY_API_BASE}/metrics/red`, {
      headers: { 'Accept': 'application/json' },
    });
    if (res.ok) {
      const json = await res.json();
      if (Array.isArray(json?.data)) {
        return json.data;
      }
    }
  } catch (err) {
    // Backend offline or unreachable
  }
  return [];
}

/**
 * Fetch real-time Traces from QueryAPI / ClickHouse.
 */
export async function fetchLiveTraces(): Promise<Span[]> {
  try {
    const res = await fetch(`${QUERY_API_BASE}/traces?limit=50`, {
      headers: { 'Accept': 'application/json' },
    });
    if (res.ok) {
      const json = await res.json();
      if (Array.isArray(json?.data) && json.data.length > 0) {
        // Fetch detailed spans for the most active traces
        const spansList: Span[] = [];
        for (const summary of json.data.slice(0, 10)) {
          try {
            const traceRes = await fetch(`${QUERY_API_BASE}/traces/${summary.traceId}`);
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
        return spansList;
      }
      return [];
    }
  } catch (err) {
    // Backend offline
  }
  return [];
}

/**
 * Fetch real-time Logs from QueryAPI / ClickHouse.
 */
export async function fetchLiveLogs(): Promise<LogRecord[]> {
  try {
    const res = await fetch(`${QUERY_API_BASE}/logs?limit=100`, {
      headers: { 'Accept': 'application/json' },
    });
    if (res.ok) {
      const json = await res.json();
      if (Array.isArray(json?.data)) {
        return json.data;
      }
    }
  } catch (err) {
    // Backend offline
  }
  return [];
}

/**
 * Fetch real-time Anomalies from QueryAPI / ClickHouse.
 */
export async function fetchLiveAnomalies(): Promise<Anomaly[]> {
  try {
    const res = await fetch(`${QUERY_API_BASE}/anomalies?range=24h`, {
      headers: { 'Accept': 'application/json' },
    });
    if (res.ok) {
      const json = await res.json();
      if (Array.isArray(json?.data)) {
        return json.data;
      }
    }
  } catch (err) {
    // Backend offline
  }
  return [];
}

/**
 * Fetch real-time Service Topology from QueryAPI.
 */
export async function fetchLiveTopology(): Promise<ServiceEdge[]> {
  try {
    const res = await fetch(`${QUERY_API_BASE}/topology`, {
      headers: { 'Accept': 'application/json' },
    });
    if (res.ok) {
      const json = await res.json();
      if (Array.isArray(json?.edges)) {
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
