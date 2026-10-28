export type SpanKind = 'UNSPECIFIED' | 'INTERNAL' | 'SERVER' | 'CLIENT' | 'PRODUCER' | 'CONSUMER';
export type StatusCode = 'UNSET' | 'OK' | 'ERROR';

export interface Span {
  tenantId: string;
  traceId: string;
  spanId: string;
  parentSpanId: string;
  serviceName: string;
  name: string;
  kind: SpanKind;
  startTime: string; // ISO or nanos
  durationNs: number;
  statusCode: StatusCode;
  statusMessage?: string;
  httpStatusCode?: number;
  attributes: Record<string, string>;
  resourceAttributes: Record<string, string>;
  ingestedAt: string;
}

export interface LogRecord {
  tenantId: string;
  timestamp: string;
  serviceName: string;
  severityText: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR' | 'FATAL';
  severityNumber: number;
  body: string;
  traceId?: string;
  spanId?: string;
  attributes: Record<string, string>;
  resourceAttributes: Record<string, string>;
}

export interface REDMetric {
  serviceName: string;
  operation: string;
  callsPerMin: number;
  errorCount: number;
  errorRate: number; // percentage
  p50Ms: number;
  p95Ms: number;
  p99Ms: number;
  status: 'HEALTHY' | 'DEGRADED' | 'CRITICAL';
}

export interface ServiceEdge {
  caller: string;
  callee: string;
  calls: number;
  errors: number;
  p95LatencyMs: number;
}

export interface Anomaly {
  id: string;
  tenantId: string;
  serviceName: string;
  operation: string;
  metric: string;
  windowStart: string;
  windowEnd: string;
  baselineValue: number;
  observedValue: number;
  zScore: number;
  rootCauseService: string;
  evidence: string;
  status: 'OPEN' | 'RESOLVED';
  severity: 'CRITICAL' | 'WARNING';
}

export interface PipelineComponentStatus {
  name: string;
  status: 'OPERATIONAL' | 'DEGRADED' | 'DOWN';
  latency: string;
  details: string;
  endpoint?: string;
}
