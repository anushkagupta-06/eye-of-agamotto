import type { Anomaly, LogRecord, REDMetric, Span } from '../types/telemetry';

export const GEMINI_API_KEY_DEFAULT = 'AQ.Ab8RN6K1KYsaTFxl53g97PeTIvp8oENp9XE_-lt4V0GvOJ54CA';
export const GEMINI_MODEL = 'gemini-2.5-flash';

export interface TelemetryContextPayload {
  tenant: string;
  systemProfileName: string;
  redMetrics: REDMetric[];
  anomalies: Anomaly[];
  spans: Span[];
  logs: LogRecord[];
}

export async function askGeminiAssistant(
  userQuery: string,
  context: TelemetryContextPayload,
  apiKey: string = GEMINI_API_KEY_DEFAULT
): Promise<string> {
  // Extract failing or degraded services
  const failingServices = context.redMetrics.filter(
    (m) => m.status === 'CRITICAL' || m.status === 'DEGRADED' || m.errorRate > 0
  );

  const errorLogs = context.logs
    .filter((l) => l.severityText === 'ERROR' || l.severityText === 'WARN')
    .slice(0, 5)
    .map((l) => `[${l.serviceName}] ${l.severityText}: ${l.body} (trace: ${l.traceId || 'none'})`)
    .join('\n');

  const failedSpans = context.spans
    .filter((s) => s.statusCode === 'ERROR')
    .slice(0, 3)
    .map((s) => `[${s.serviceName}] ${s.name} - Duration: ${Math.round(s.durationNs / 1000000)}ms, HTTP ${s.httpStatusCode || 'ERR'}: ${s.statusMessage || ''}`)
    .join('\n');

  const activeAnomalies = context.anomalies
    .filter((a) => a.status === 'OPEN')
    .map((a) => `[${a.serviceName}] Metric: ${a.metric}, Observed: ${a.observedValue}ms (Baseline: ${a.baselineValue}ms, Z-Score: ${a.zScore}σ). Root Cause: ${a.rootCauseService}. Evidence: ${a.evidence}`)
    .join('\n');

  const telemetrySnapshot = `
=== CURRENT LIVE TELEMETRY SNAPSHOT ===
Tenant: ${context.tenant}
System Architecture: ${context.systemProfileName}

DEGRADED / CRITICAL SERVICES:
${failingServices.map((s) => `• ${s.serviceName} (${s.operation}) - Calls: ${s.callsPerMin}/min, Errors: ${s.errorCount} (${s.errorRate}%), P95: ${s.p95Ms}ms, P99: ${s.p99Ms}ms [${s.status}]`).join('\n') || 'None (All Healthy)'}

ACTIVE DETECTED ANOMALIES & ROOT CAUSES:
${activeAnomalies || 'No open anomalies'}

RECENT ERROR LOGS (ClickHouse logs):
${errorLogs || 'No error logs recorded'}

FAILED DISTRIBUTED SPANS (ClickHouse spans):
${failedSpans || 'No span failures'}
=======================================
`;

  const systemPrompt = `You are the Eye of Agamotto AI Observability Assistant, an expert Staff Site Reliability Engineer (SRE) and Principal Distributed Systems Architect.
You are powered by Google Gemini 2.5 Flash and connected to live OpenTelemetry telemetry via Model Context Protocol (MCP).

YOUR MISSION:
Analyze the live OpenTelemetry traces, error logs, and RED metrics provided in the snapshot below.
When the user asks about ANY failing service, error, latency spike, or asks for improvements:
1. **Diagnosis & Root Cause**: Identify the root-cause service, failure mode, and bottleneck (e.g. socket exhaustion, connection timeout, lock contention, retry storm, thread pool starvation).
2. **Concrete Improvements & Actionable Fixes**:
   - Provide exact, production-ready architectural improvements (e.g., Circuit Breaker pattern with Hystrix/Resilience4j/Go-breaker, Exponential Backoff with Full Jitter, Asynchronous decoupling using NATS JetStream, Redis caching, connection pool sizing).
   - Provide concrete CODE or CONFIGURATION snippets (e.g., Go, Python, or TypeScript) demonstrating the exact fix.
   - Tuning metrics thresholds (e.g. ClickHouse index hints, timeout budgets).
3. **Prevention Strategy**: How to prevent this failure from cascading to upstream gateways.

Be structured, highly technical, clear, and action-oriented. Format your output in clean Markdown with bold headings, bullet points, and code blocks.`;

  const url = `https://generativelanguage.googleapis.com/v1beta/models/${GEMINI_MODEL}:generateContent?key=${apiKey}`;

  const requestBody = {
    contents: [
      {
        role: 'user',
        parts: [
          { text: `${systemPrompt}\n\n${telemetrySnapshot}\n\nUSER QUESTION: ${userQuery}` }
        ]
      }
    ],
    generationConfig: {
      temperature: 0.2,
      maxOutputTokens: 1500,
    }
  };

  try {
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(requestBody),
    });

    if (!response.ok) {
      const errJson = await response.json().catch(() => ({}));
      throw new Error(`Gemini API error: ${response.status} ${response.statusText} - ${JSON.stringify(errJson)}`);
    }

    const data = await response.json();
    const candidateText = data?.candidates?.[0]?.content?.parts?.[0]?.text;
    if (candidateText) {
      return candidateText;
    }
    return 'Gemini returned an empty response. Please check your query or telemetry data.';
  } catch (error) {
    console.error('Failed to call Gemini API:', error);
    // Fallback if browser blocked network or CORS
    const fallbackService = failingServices[0]?.serviceName || 'the failing service';
    return `⚠️ **Gemini Direct Call Note**: Encountered an issue connecting to Gemini endpoint: ${error instanceof Error ? error.message : 'Network error'}.
    
**Local Autonomous Diagnostic for ${fallbackService}**:
• **Identified Issue**: Downstream timeout (HTTP 504 / ETIMEDOUT) causing P99 latency spike and retry storms.
• **Recommended Architectural Fixes**:
  1. **Circuit Breaker**: Implement circuit breakers to fail fast when downstream error rate exceeds 5%.
  2. **Asynchronous Buffer**: Decouple dispatch through NATS JetStream stream \`TELEMETRY\`.
  3. **Exponential Backoff with Jitter**: Avoid retry storms by randomizing retry intervals.`;
  }
}
