import type { Anomaly, LogRecord, REDMetric, Span, ServiceEdge } from '../types/telemetry';

const QUERY_API_BASE = '/api/query/api/v1';
const GATEWAY_BASE = '/api/gateway/v1';
const DEFAULT_API_KEY = 'eoa_cffdedbba2a3d4f0ecb4706923ae02d5d4b2ac1a0c03ff44';

function timeRangeToWindow(range?: string): { from?: string; to?: string } {
  if (!range) return {};
  const now = new Date();
  let durationMs = 15 * 60 * 1000;
  if (range === '1h') durationMs = 60 * 60 * 1000;
  else if (range === '24h') durationMs = 24 * 60 * 60 * 1000;
  else if (range === '7d') durationMs = 7 * 24 * 60 * 60 * 1000;
  const from = new Date(now.getTime() - durationMs).toISOString();
  const to = now.toISOString();
  return { from, to };
}

/**
 * Fetch real-time RED Metrics from QueryAPI / ClickHouse.
 */
export async function fetchLiveREDMetrics(range?: string): Promise<REDMetric[]> {
  const { from, to } = timeRangeToWindow(range);
  const params = new URLSearchParams();
  if (from) params.set('from', from);
  if (to) params.set('to', to);

  const queryStr = params.toString();
  const url = `${QUERY_API_BASE}/metrics/red${queryStr ? '?' + queryStr : ''}`;
  const res = await fetch(url, { headers: { Accept: 'application/json' } });
  if (!res.ok) {
    throw new Error(`RED Metrics query failed with status ${res.status}`);
  }
  const json = await res.json();
  if (Array.isArray(json?.data)) {
    return json.data;
  }
  return [];
}

export interface TraceSearchFilters {
  service?: string;
  status?: string;
  operation?: string;
  minDurationNs?: number;
  range?: string;
  limit?: number;
  offset?: number;
}

export interface TraceSummaryItem {
  traceId: string;
  rootService: string;
  rootOperation: string;
  spanCount: number;
  durationMs: number;
  hasError: boolean;
  startTime: string;
}

/**
 * Fetch trace summaries from QueryAPI.
 */
export async function fetchLiveTraceSummaries(filters?: TraceSearchFilters): Promise<TraceSummaryItem[]> {
  const params = new URLSearchParams();
  const { from, to } = timeRangeToWindow(filters?.range);
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  if (filters?.service && filters.service !== 'ALL') params.set('service', filters.service);
  if (filters?.status && filters.status !== 'ALL') params.set('status', filters.status);
  if (filters?.operation) params.set('operation', filters.operation);
  if (filters?.minDurationNs) params.set('min_duration', String(filters.minDurationNs));
  params.set('limit', String(filters?.limit || 50));
  if (filters?.offset) params.set('offset', String(filters.offset));

  const res = await fetch(`${QUERY_API_BASE}/traces?${params.toString()}`, {
    headers: { Accept: 'application/json' },
  });
  if (!res.ok) {
    throw new Error(`Traces search query failed with status ${res.status}`);
  }
  const json = await res.json();
  if (Array.isArray(json?.data)) {
    return json.data.map((item: any) => ({
      traceId: item.traceId,
      rootService: item.rootService || 'unknown',
      rootOperation: item.rootOperation || 'unknown',
      spanCount: item.spanCount || 1,
      durationMs: Math.round((item.durationNs || 0) / 1000000),
      hasError: Boolean(item.hasError),
      startTime: item.startTime,
    }));
  }
  return [];
}

/**
 * Fetch detailed spans for a specific traceId.
 */
export async function fetchTraceDetail(traceId: string): Promise<Span[]> {
  if (!traceId) return [];
  const res = await fetch(`${QUERY_API_BASE}/traces/${encodeURIComponent(traceId)}`, {
    headers: { Accept: 'application/json' },
  });
  if (!res.ok) {
    if (res.status === 404) return [];
    throw new Error(`Trace lookup failed with status ${res.status}`);
  }
  const json = await res.json();
  if (Array.isArray(json?.spans)) {
    return json.spans;
  }
  return [];
}

/**
 * Fetch spans for active traces in the current time window.
 */
export async function fetchLiveTraces(range?: string): Promise<Span[]> {
  try {
    const summaries = await fetchLiveTraceSummaries({ range, limit: 15 });
    if (summaries.length === 0) return [];

    const detailedSpans: Span[] = [];
    for (const summary of summaries.slice(0, 10)) {
      try {
        const spans = await fetchTraceDetail(summary.traceId);
        detailedSpans.push(...spans);
      } catch {
        // Continue fetching other traces
      }
    }
    return detailedSpans;
  } catch (err) {
    console.warn('Failed to fetch live traces:', err);
    throw err;
  }
}

export interface LogSearchFilters {
  query?: string;
  service?: string;
  severity?: string;
  traceId?: string;
  range?: string;
  limit?: number;
  offset?: number;
}

/**
 * Fetch real-time Logs from QueryAPI / ClickHouse.
 */
export async function fetchLiveLogs(filters?: LogSearchFilters): Promise<LogRecord[]> {
  const params = new URLSearchParams();
  const { from, to } = timeRangeToWindow(filters?.range);
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  if (filters?.query) params.set('query', filters.query);
  if (filters?.service && filters.service !== 'ALL') params.set('service', filters.service);
  if (filters?.severity && filters.severity !== 'ALL') params.set('severity', filters.severity);
  if (filters?.traceId) params.set('trace_id', filters.traceId);
  params.set('limit', String(filters?.limit || 100));
  if (filters?.offset) params.set('offset', String(filters.offset));

  const res = await fetch(`${QUERY_API_BASE}/logs?${params.toString()}`, {
    headers: { Accept: 'application/json' },
  });
  if (!res.ok) {
    throw new Error(`Logs search query failed with status ${res.status}`);
  }
  const json = await res.json();
  if (Array.isArray(json?.data)) {
    return json.data;
  }
  return [];
}

/**
 * Fetch real-time Anomalies from QueryAPI / ClickHouse.
 */
export async function fetchLiveAnomalies(range: string = '24h'): Promise<Anomaly[]> {
  const res = await fetch(`${QUERY_API_BASE}/anomalies?range=${encodeURIComponent(range)}`, {
    headers: { Accept: 'application/json' },
  });
  if (!res.ok) {
    throw new Error(`Anomalies query failed with status ${res.status}`);
  }
  const json = await res.json();
  if (Array.isArray(json?.data)) {
    return json.data;
  }
  return [];
}

/**
 * Fetch real-time Service Topology from QueryAPI.
 */
export async function fetchLiveTopology(range?: string): Promise<ServiceEdge[]> {
  const params = new URLSearchParams();
  if (range) params.set('range', range);
  const queryStr = params.toString();
  const url = `${QUERY_API_BASE}/topology${queryStr ? '?' + queryStr : ''}`;

  const res = await fetch(url, { headers: { Accept: 'application/json' } });
  if (!res.ok) {
    throw new Error(`Topology query failed with status ${res.status}`);
  }
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
  return [];
}

/**
 * Emit a real OpenTelemetry Trace via Ingestion Gateway HTTP /v1/traces endpoint.
 * This triggers the full pipeline: Gateway -> NATS JetStream -> ClickHouse Writer & Stream Processor.
 */
export async function emitLiveOTLPTrace(
  serviceName: string,
  operation: string = 'POST /v1/process',
  isError: boolean = false
): Promise<{ traceId: string; spanId: string }> {
  const cleanService = serviceName.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '-');
  const traceId = Array.from({ length: 32 }, () => Math.floor(Math.random() * 16).toString(16)).join('');
  const spanId = Array.from({ length: 16 }, () => Math.floor(Math.random() * 16).toString(16)).join('');
  const nowUnixNano = Date.now() * 1000000;
  const durationNano = 45 * 1000000; // 45ms

  const otlpPayload = {
    resourceSpans: [
      {
        resource: {
          attributes: [
            { key: 'service.name', value: { stringValue: cleanService } },
            { key: 'deployment.environment', value: { stringValue: 'production' } },
          ],
        },
        scopeSpans: [
          {
            scope: { name: 'eoa-dashboard-ingest' },
            spans: [
              {
                traceId,
                spanId,
                name: operation,
                kind: 2, // SPAN_KIND_SERVER
                startTimeUnixNano: String(nowUnixNano),
                endTimeUnixNano: String(nowUnixNano + durationNano),
                attributes: [
                  { key: 'http.method', value: { stringValue: 'POST' } },
                  { key: 'http.status_code', value: { intValue: isError ? 500 : 200 } },
                ],
                status: {
                  code: isError ? 2 : 1, // 2 = STATUS_CODE_ERROR, 1 = STATUS_CODE_OK
                  message: isError ? 'Internal Server Error' : '',
                },
              },
            ],
          },
        ],
      },
    ],
  };

  const res = await fetch(`${GATEWAY_BASE}/traces`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-EOA-API-Key': DEFAULT_API_KEY,
    },
    body: JSON.stringify(otlpPayload),
  });

  if (!res.ok) {
    throw new Error(`Ingestion gateway rejected trace [HTTP ${res.status}]`);
  }

  return { traceId, spanId };
}
