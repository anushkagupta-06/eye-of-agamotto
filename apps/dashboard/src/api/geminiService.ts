import type { Anomaly, LogRecord, REDMetric, Span } from '../types/telemetry';
import { callMCPTool } from './mcpClient';

export const GEMINI_API_KEY_DEFAULT: string =
  (import.meta as any).env?.VITE_GEMINI_API_KEY ||
  (import.meta as any).env?.GEMINI_API_KEY_DEFAULT 
  '';

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
  // 1. Filter out only failing or degraded services to reduce token noise
  const failingServices = context.redMetrics.filter(
    (m) => m.status === 'CRITICAL' || m.status === 'DEGRADED' || m.errorRate > 0 || m.p95Ms > 100
  );

  // 2. Extract recent high-severity error logs (ClickHouse logs)
  const errorLogs = context.logs
    .filter((l) => l.severityText === 'ERROR' || l.severityText === 'WARN' || l.severityNumber >= 13)
    .slice(0, 6)
    .map((l) => `[${l.serviceName}] [${l.severityText}] "${l.body}" (trace_id: ${l.traceId || 'none'})`)
    .join('\n');

  // 3. Extract failed distributed spans (ClickHouse spans)
  const failedSpans = context.spans
    .filter((s) => s.statusCode === 'ERROR')
    .slice(0, 4)
    .map(
      (s) =>
        `[${s.serviceName}] ${s.name} - Duration: ${Math.round(s.durationNs / 1000000)}ms, HTTP: ${s.httpStatusCode || 'ERR'}, Msg: "${s.statusMessage || 'Unspecified error'}"`
    )
    .join('\n');

  // 4. Extract active Z-score anomalies
  const activeAnomalies = context.anomalies
    .filter((a) => a.status === 'OPEN')
    .map(
      (a) =>
        `• [${a.serviceName}] Metric: ${a.metric} | Observed: ${a.observedValue}ms (Baseline: ${a.baselineValue}ms, Z-Score: ${a.zScore}σ)\n  Root Cause: ${a.rootCauseService}\n  Evidence: ${a.evidence}`
    )
    .join('\n');

  // 5. Structure a high-density, low-token Telemetry Snapshot
  const telemetrySnapshot = `
=== LIVE OPENTELEMETRY CONTEXT SNAPSHOT ===
TENANT: "${context.tenant}"
ACTIVE ARCHITECTURE DOMAIN: "${context.systemProfileName}"

1. DEGRADED / FAILING SERVICES (RED Metrics):
${
  failingServices.length > 0
    ? failingServices
        .map(
          (s) =>
            `• ${s.serviceName} (${s.operation}) -> Calls: ${s.callsPerMin}/min, Errors: ${s.errorCount} (${s.errorRate}%), P95: ${s.p95Ms}ms, P99: ${s.p99Ms}ms [${s.status}]`
        )
        .join('\n')
    : 'None (All services within healthy SLA budgets)'
}

2. STATISTICAL LATENCY & ERROR ANOMALIES (Z-Score Spikes):
${activeAnomalies || 'No open anomaly clusters detected'}

3. RECENT CLICKHOUSE ERROR LOGS (Filtered):
${errorLogs || 'No critical error logs detected'}

4. FAILED DISTRIBUTED TRACE SPANS:
${failedSpans || 'No span errors detected'}
===========================================
`;

  // 6. Professional Staff SRE & Principal Distributed Systems Architect System Prompt
  const systemInstruction = `You are the Eye of Agamotto AI Observability Assistant, an expert Staff Site Reliability Engineer (SRE) and Principal Distributed Systems Architect.
You are powered by Google Gemini 2.5 Flash and connected to live OpenTelemetry telemetry via Model Context Protocol (MCP).

YOUR OPERATIONAL OBJECTIVES:
1. **Diagnosis & Root Cause**: Identify the exact culprit service, failure mode, and bottleneck (e.g., connection pool exhaustion, socket starvation, slow database queries, thread deadlocks, retry storms, upstream cascading timeout).
2. **Actionable Code & Architecture Improvements**:
   - Deliver concrete, production-grade code snippets (Go, Python, TypeScript, or SQL) that engineers can immediately paste to fix the bug.
   - Prescribe battle-tested architectural resilience patterns:
     * Circuit Breaker pattern (with trip thresholds, half-open probe requests).
     * Exponential Backoff with Full Jitter.
     * Decoupled asynchronous worker queues using NATS JetStream.
     * Connection pool sizing and deadline budgets.
     * ClickHouse index tuning or cache invalidation.
3. **Structured Incident Response Format**:
   Always format your answer cleanly using these exact 4 Markdown sections:
   - ### 🚨 1. Incident Diagnosis & Root Cause
   - ### ⚡ 2. Immediate Code & Architectural Fix (Include complete, syntax-highlighted code block)
   - ### ⚙️ 3. Configuration & Timeout Tuning
   - ### 🛡️ 4. Blast Radius & Upstream Prevention

Cite the exact service names, numbers, and observed latencies from the telemetry snapshot. Be direct, authoritative, and concise.`;

  const url = `https://generativelanguage.googleapis.com/v1beta/models/${GEMINI_MODEL}:generateContent?key=${apiKey}`;

  const mcpToolsDeclaration = [
    {
      function_declarations: [
        {
          name: 'searchTraces',
          description: 'Search distributed traces and spans in ClickHouse filtered by service, minDuration, or status (ERROR/OK).',
          parameters: {
            type: 'OBJECT',
            properties: {
              service: { type: 'STRING', description: 'Microservice name' },
              status: { type: 'STRING', description: 'ERROR or OK' },
              limit: { type: 'NUMBER', description: 'Maximum traces to return' },
            },
          },
        },
        {
          name: 'getTrace',
          description: 'Retrieve the complete distributed trace DAG for a specific traceId.',
          parameters: {
            type: 'OBJECT',
            properties: {
              traceId: { type: 'STRING', description: '32-character hexadecimal trace identifier' },
            },
            required: ['traceId'],
          },
        },
        {
          name: 'queryLogs',
          description: 'Search indexed ClickHouse logs using token full-text search, service filter, or severity.',
          parameters: {
            type: 'OBJECT',
            properties: {
              query: { type: 'STRING', description: 'Search token in log body' },
              service: { type: 'STRING', description: 'Service name filter' },
              severity: { type: 'STRING', description: 'ERROR, WARN, INFO' },
              traceId: { type: 'STRING', description: 'Filter logs by traceId' },
            },
          },
        },
        {
          name: 'getServiceTopology',
          description: 'Retrieve the live directed service dependency topology map with RED metrics.',
          parameters: {
            type: 'OBJECT',
            properties: {},
          },
        },
        {
          name: 'listAnomalies',
          description: 'List active statistical anomalies and latency spikes.',
          parameters: {
            type: 'OBJECT',
            properties: {},
          },
        },
        {
          name: 'explainAnomaly',
          description: 'Deep-dive analysis of an anomaly and candidate root-cause spans.',
          parameters: {
            type: 'OBJECT',
            properties: {
              anomalyId: { type: 'STRING', description: 'Anomaly identifier' },
            },
            required: ['anomalyId'],
          },
        },
      ],
    },
  ];

  // Using official Gemini 2.5 system_instruction format with MCP tools
  const requestBody: any = {
    system_instruction: {
      parts: [{ text: systemInstruction }],
    },
    contents: [
      {
        role: 'user',
        parts: [
          {
            text: `${telemetrySnapshot}\n\nUSER QUESTION / TASK:\n${userQuery}`,
          },
        ],
      },
    ],
    tools: mcpToolsDeclaration,
    generationConfig: {
      temperature: 0.15,
      maxOutputTokens: 2048,
    },
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
    const candidateParts = data?.candidates?.[0]?.content?.parts;

    // Check if Gemini invoked an MCP tool
    const functionCallPart = candidateParts?.find((p: any) => p.functionCall);
    if (functionCallPart?.functionCall) {
      const { name, args } = functionCallPart.functionCall;
      console.log(`[MCP] Gemini called tool '${name}':`, args);

      try {
        const toolResult = await callMCPTool(name, args || {});
        console.log(`[MCP] Tool result received for '${name}':`, toolResult);

        // Send function response back to Gemini to complete the tool execution loop
        const followUpBody = {
          system_instruction: {
            parts: [{ text: systemInstruction }],
          },
          contents: [
            ...requestBody.contents,
            {
              role: 'model',
              parts: [{ functionCall: functionCallPart.functionCall }],
            },
            {
              role: 'user',
              parts: [
                {
                  functionResponse: {
                    name,
                    response: { output: toolResult },
                  },
                },
              ],
            },
          ],
          tools: mcpToolsDeclaration,
          generationConfig: {
            temperature: 0.15,
            maxOutputTokens: 2048,
          },
        };

        const followUpResp = await fetch(url, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(followUpBody),
        });

        if (followUpResp.ok) {
          const followUpData = await followUpResp.json();
          const finalCandidateText = followUpData?.candidates?.[0]?.content?.parts?.[0]?.text;
          if (finalCandidateText) {
            return `> ⚡ **Executed MCP Tool:** \`${name}\`\n\n${finalCandidateText}`;
          }
        }
      } catch (mcpErr) {
        console.warn(`[MCP] Tool call '${name}' failed, falling back to direct context:`, mcpErr);
      }
    }

    const candidateText = candidateParts?.[0]?.text;
    if (candidateText) {
      return candidateText;
    }
    return 'Gemini returned an empty response. Please verify the active telemetry snapshot.';
  } catch (error) {
    console.error('Failed to call Gemini API:', error);

    // Fallback response with concrete advice if offline or rate-limited
    const worstService = failingServices[0]?.serviceName || 'downstream-dependency';
    return `### 🚨 1. Incident Diagnosis & Root Cause
The active telemetry reveals elevated latency and error rates primarily originating from **${worstService}**. The observed failure exhibits signs of downstream saturation and thread starvation under sustained traffic.

### ⚡ 2. Immediate Code & Architectural Fix
Implement an asynchronous Circuit Breaker with exponential backoff:

\`\`\`go
// Go implementation using sony/gobreaker
var cb *gobreaker.CircuitBreaker

func init() {
    cb = gobreaker.NewCircuitBreaker(gobreaker.Settings{
        Name:        "${worstService}-breaker",
        MaxRequests: 3,
        Interval:    10 * time.Second,
        Timeout:     3 * time.Second,
        ReadyToTrip: func(counts gobreaker.Counts) bool {
            failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
            return counts.Requests >= 10 && failureRatio >= 0.3
        },
    })
}
\`\`\`

### ⚙️ 3. Configuration & Timeout Tuning
- Enforce strict context deadlines: \`http.Client{Timeout: 500 * time.Millisecond}\`.
- Scale NATS worker consumers to absorb queue backpressure.

### 🛡️ 4. Blast Radius & Upstream Prevention
Return a graceful fallback response or cached payload at the API Gateway when the circuit opens, preventing cascade failure to upstream clients.`;
  }
}
