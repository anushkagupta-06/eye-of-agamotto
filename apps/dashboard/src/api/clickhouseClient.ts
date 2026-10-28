/**
 * ClickHouse Direct REST Client
 * Connects the Eye of Agamotto frontend directly to ClickHouse (via Vite proxy /api/clickhouse).
 * Implements the exact query contracts defined in docs/handoff-ingestion-storage.md.
 */

import type { LogRecord, REDMetric, Span } from '../types/telemetry';

const CLICKHOUSE_BASE_URL = '/api/clickhouse';

export interface ClickHouseQueryResult<T> {
  meta: Array<{ name: string; type: string }>;
  data: T[];
  rows: number;
}

/**
 * Checks if the ClickHouse server is running and reachable.
 */
export async function pingClickHouse(): Promise<boolean> {
  try {
    const res = await fetch(`${CLICKHOUSE_BASE_URL}/ping`, {
      method: 'GET',
      headers: { 'Accept': 'text/plain' },
    });
    if (!res.ok) return false;
    const text = await res.text();
    return text.trim() === 'Ok.';
  } catch {
    return false;
  }
}

/**
 * Executes a read-only SQL query against ClickHouse using the query_api profile.
 */
export async function executeClickHouseQuery<T>(sql: string): Promise<T[]> {
  const url = `${CLICKHOUSE_BASE_URL}/?query=${encodeURIComponent(sql + ' FORMAT JSON')}`;
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      'Accept': 'application/json',
    },
  });

  if (!response.ok) {
    const errText = await response.text();
    throw new Error(`ClickHouse query failed [HTTP ${response.status}]: ${errText}`);
  }

  const json: ClickHouseQueryResult<T> = await response.json();
  return json.data || [];
}

/**
 * Fetch spans for a specific trace ID (Flamegraph query from Naina's handoff)
 */
export async function fetchTraceSpansFromClickHouse(tenantId: string, traceId: string): Promise<Span[]> {
  const sql = `
    SELECT
      tenant_id,
      trace_id,
      span_id,
      parent_span_id,
      service_name,
      name,
      kind,
      toString(start_time) AS start_time,
      duration_ns,
      status_code,
      status_message,
      http_status_code,
      attributes,
      resource_attributes,
      toString(ingested_at) AS ingested_at
    FROM observability.spans
    WHERE tenant_id = '${tenantId}' AND trace_id = '${traceId}'
    ORDER BY ingested_at DESC
    LIMIT 1 BY span_id
  `;

  interface RawSpan {
    tenant_id: string;
    trace_id: string;
    span_id: string;
    parent_span_id: string;
    service_name: string;
    name: string;
    kind: string;
    start_time: string;
    duration_ns: number;
    status_code: string;
    status_message: string;
    http_status_code: number;
    attributes: Record<string, string>;
    resource_attributes: Record<string, string>;
    ingested_at: string;
  }

  const rows = await executeClickHouseQuery<RawSpan>(sql);
  return rows.map((r) => ({
    tenantId: r.tenant_id,
    traceId: r.trace_id,
    spanId: r.span_id,
    parentSpanId: r.parent_span_id || '',
    serviceName: r.service_name,
    name: r.name,
    kind: (r.kind || 'SERVER') as Span['kind'],
    startTime: r.start_time,
    durationNs: Number(r.duration_ns),
    statusCode: (r.status_code || 'OK') as Span['statusCode'],
    statusMessage: r.status_message,
    httpStatusCode: r.http_status_code,
    attributes: r.attributes || {},
    resourceAttributes: r.resource_attributes || {},
    ingestedAt: r.ingested_at,
  }));
}

/**
 * Fetch logs with full-text search and time pruning (Logs query from Naina's handoff)
 */
export async function fetchLogsFromClickHouse(
  tenantId: string,
  options?: {
    service?: string;
    keyword?: string;
    severityNumberMin?: number;
    limit?: number;
  }
): Promise<LogRecord[]> {
  const limit = options?.limit || 100;
  const whereClauses: string[] = [
    `tenant_id = '${tenantId}'`,
    `timestamp >= now() - INTERVAL 1 HOUR`,
  ];

  if (options?.service && options.service !== 'ALL') {
    whereClauses.push(`service_name = '${options.service}'`);
  }
  if (options?.severityNumberMin) {
    whereClauses.push(`severity_number >= ${options.severityNumberMin}`);
  }
  if (options?.keyword && options.keyword.trim().length > 0) {
    whereClauses.push(`hasToken(body, '${options.keyword.replace(/'/g, "\\'")}')`);
  }

  const sql = `
    SELECT
      tenant_id,
      toString(timestamp) AS timestamp,
      service_name,
      severity_text,
      severity_number,
      body,
      trace_id,
      span_id,
      attributes,
      resource_attributes
    FROM observability.logs
    WHERE ${whereClauses.join(' AND ')}
    ORDER BY timestamp DESC
    LIMIT ${limit}
  `;

  interface RawLog {
    tenant_id: string;
    timestamp: string;
    service_name: string;
    severity_text: string;
    severity_number: number;
    body: string;
    trace_id: string;
    span_id: string;
    attributes: Record<string, string>;
    resource_attributes: Record<string, string>;
  }

  const rows = await executeClickHouseQuery<RawLog>(sql);
  return rows.map((r) => ({
    tenantId: r.tenant_id,
    timestamp: r.timestamp,
    serviceName: r.service_name,
    severityText: (r.severity_text || 'INFO') as LogRecord['severityText'],
    severityNumber: r.severity_number || 9,
    body: r.body,
    traceId: r.trace_id,
    spanId: r.span_id,
    attributes: r.attributes || {},
    resourceAttributes: r.resource_attributes || {},
  }));
}

/**
 * Fetch RED metrics rolled up from ClickHouse latency_1m table
 */
export async function fetchREDMetricsFromClickHouse(tenantId: string): Promise<REDMetric[]> {
  const sql = `
    SELECT
      service_name,
      operation,
      sum(calls) AS calls_per_min,
      sum(errors) AS error_count,
      round(sum(errors) * 100.0 / nullIf(sum(calls), 0), 2) AS error_rate,
      round(quantilesTDigestMerge(0.5, 0.95, 0.99)(duration_q)[1] / 1000000, 1) AS p50_ms,
      round(quantilesTDigestMerge(0.5, 0.95, 0.99)(duration_q)[2] / 1000000, 1) AS p95_ms,
      round(quantilesTDigestMerge(0.5, 0.95, 0.99)(duration_q)[3] / 1000000, 1) AS p99_ms
    FROM observability.latency_1m
    WHERE tenant_id = '${tenantId}' AND minute >= now() - INTERVAL 15 MINUTE
    GROUP BY service_name, operation
    ORDER BY calls_per_min DESC
    LIMIT 20
  `;

  interface RawMetric {
    service_name: string;
    operation: string;
    calls_per_min: number;
    error_count: number;
    error_rate: number;
    p50_ms: number;
    p95_ms: number;
    p99_ms: number;
  }

  const rows = await executeClickHouseQuery<RawMetric>(sql);
  return rows.map((r) => {
    const errorRate = Number(r.error_rate) || 0;
    const p95 = Number(r.p95_ms) || 0;
    const status: REDMetric['status'] = errorRate > 2.0 || p95 > 250 ? 'CRITICAL' : (errorRate > 0.5 || p95 > 100 ? 'DEGRADED' : 'HEALTHY');
    return {
      serviceName: r.service_name,
      operation: r.operation,
      callsPerMin: Number(r.calls_per_min) || 0,
      errorCount: Number(r.error_count) || 0,
      errorRate: errorRate,
      p50Ms: Number(r.p50_ms) || 0,
      p95Ms: p95,
      p99Ms: Number(r.p99_ms) || 0,
      status,
    };
  });
}
