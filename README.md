# The Eye of Agamotto (Eye of Agamotto)
> **High-Throughput Distributed Observability, Flamegraph Visualizer, and AI-Native Root-Cause Diagnosis Platform**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=flat&logo=react)](https://react.dev)
[![ClickHouse](https://img.shields.io/badge/ClickHouse-24.8-FFCC01?style=flat&logo=clickhouse)](https://clickhouse.com)
[![NATS JetStream](https://img.shields.io/badge/NATS-JetStream-27AAE1?style=flat&logo=nats.io)](https://nats.io)
[![OpenTelemetry](https://img.shields.io/badge/OTel-v1.30+-F5A800?style=flat&logo=opentelemetry)](https://opentelemetry.io)
[![Model Context Protocol](https://img.shields.io/badge/MCP-Standard-purple?style=flat)](https://modelcontextprotocol.io)

---

## 1. Problem Statement & Overview

Modern production software architectures are split across ephemeral microservices running on containers and serverless environments. When a latency spike or cascading failure strikes (e.g., *"checkout is degraded"*), software engineers struggle to:
1. Identify the origin service causing the failure vs. downstream victims.
2. Correlate millions of high-cardinality logs and spans across ephemeral containers that may no longer exist.
3. Trace asynchronous RPC boundaries and parent-child execution paths without heavy database queries.

**The Eye of Agamotto** is a self-hosted, cloud-native observability platform engineered for sub-second distributed trace exploration, automated service topology discovery, streaming log aggregation, and LLM-powered root-cause analysis through the **Model Context Protocol (MCP)**.

---

## 2. System Architecture

```
                          [ Instrumented Microservices ]
                         (OTel SDK: HTTP/4318 & gRPC/4317)
                                        │
                                        ▼
                           [ Ingestion Gateway ]
                 (API-Key Auth · Token Bucket Rate Limiter)
                                        │
                                        ▼
                              [ NATS JetStream ]
                (Durable subjects: telemetry.traces & telemetry.logs)
                     ┌──────────────────┴──────────────────┐
                     ▼                                     ▼
             [ Writer Service ]                   [ Stream Processor ]
         (Bulk Columnar Insertion)            (Parent-Child Cross-Boundary Join)
                     │                                     │
                     │  observability.spans                │  observability.edge_events
                     │  observability.logs                 │  observability.service_edges_1m
                     ▼                                     ▼
        ┌─────────────────────────────────────────────────────────────┐
        │                 ClickHouse Columnar Storage                 │
        │        (Token-Bloom Filters, T-Digest Latency Rollups)      │
        └──────────────────────────────┬──────────────────────────────┘
                                       │
                                       ▼
                              [ Query API Server ]
                 (ClickHouse RO Pool · NATS Live Tail SSE · Cache)
                     ┌─────────────────┴─────────────────┐
                     ▼                                   ▼
          [ React 19 Web Dashboard ]             [ MCP Server ]
       - Interactive D3 Flamegraph          - search_traces / get_trace
       - React Flow Dependency Map          - query_logs / get_service_topology
       - Live Log Explorer (SSE)            - list_anomalies / explain_anomaly
       - Root-Cause Diagnostic AI           - Stdio & SSE Transport for LLMs
```

---

## 3. Abstract Feature Comparison: Implemented vs. Future

| Proposed Capability in Abstract | Status in Codebase | Implementation Details / Code Location |
| :--- | :---: | :--- |
| **OTel Ingestion Gateway** (gRPC 4317 & HTTP 4318) | **Completed** | Full OTLP protobuf & JSON support in [`services/ingestion-gateway`](file:///services/ingestion-gateway/). |
| **API Key Authentication & Tenant Routing** | **Completed** | SHA-256 constant-time lookup in [`internal/auth/keystore.go`](file:///services/ingestion-gateway/internal/auth/keystore.go). |
| **Backpressure & Rate Limiting** | **Completed** | Token bucket algorithm returning HTTP `429` and gRPC `RESOURCE_EXHAUSTED` in [`internal/ratelimit`](file:///services/ingestion-gateway/internal/ratelimit/). |
| **Durable Messaging Buffer** | **Completed** | NATS JetStream `TELEMETRY` stream with independent consumer groups (`writer-traces`, `writer-logs`, `proc-traces`). |
| **Columnar Telemetry Storage** | **Completed** | ClickHouse schema with Token-Bloom filters (`body`), Bloom filters (`trace_id`, `attributes`), and ReplacingMergeTree in [`001_schema.sql`](file:///infra/clickhouse/init/001_schema.sql). |
| **Service-to-Service Edge Extraction** | **Completed** | Stream processor derives caller $\to$ callee edges from parent-child client/server spans in [`services/stream-processor/edge_events.go`](file:///services/stream-processor/edge_events.go). |
| **RED Metrics Rollup** | **Completed** | T-Digest quantiles (P50, P95, P99) and call/error aggregation in `latency_1m` / `service_edges_1m` and [`services/stream-processor/red_metrics.go`](file:///services/stream-processor/red_metrics.go). |
| **Query API & Live Log Tail** | **Completed** | Go Query API exposing traces, logs, topology, RED metrics, and SSE live tail in [`services/query-api`](file:///services/query-api/). |
| **Interactive Distributed Trace Flamegraph** | **Completed** | Zoomable D3 waterfall timeline with clock-skew detection & clamping in [`TraceFlamegraphD3.tsx`](file:///apps/dashboard/src/components/traces/TraceFlamegraphD3.tsx). |
| **Service Dependency Topology Map** | **Completed** | Directed DAG visualization via `@xyflow/react` with real-time error colors and latency tags in [`TopologyPage.tsx`](file:///apps/dashboard/src/pages/TopologyPage.tsx). |
| **Log Explorer with Trace Correlation** | **Completed** | Full-text, regex, level, and span/trace-id filters with 1-click drill-down in [`LogsPage.tsx`](file:///apps/dashboard/src/pages/LogsPage.tsx). |
| **Model Context Protocol (MCP) Server** | **Completed** | 6 standard tools (`search_traces`, `get_trace`, `query_logs`, `get_service_topology`, `list_anomalies`, `explain_anomaly`) over stdio and SSE in [`services/mcp-server`](file:///services/mcp-server/). |
| **Automated Root-Cause Correlation (Continuous Daemon)** | **Completed** | Continuous streaming anomaly detection ($Z$-score $\ge 3.0$), root-cause attribution, and lifecycle tracking (`OPEN`/`RESOLVED`) in [`services/stream-processor`](file:///services/stream-processor/). |
| **Built-in 5-Microservice Demo Workload & Fault Injector** | **Roadmap** | Benchmark script (`smoke-ingest.sh`) exists; containerized multi-service mock cluster + k6 chaos injector in roadmap. |
| **Virtualized Infinite Scroll (100k+ rows)** | **Roadmap** | TanStack Virtual installed in `package.json`; currently utilizes server-side pagination with real-time SSE. |
| **Multi-Tenant User Login & RBAC UI** | **Roadmap** | Simulated dev auth currently active; OIDC/JWT login UI deferred to production deployment roadmap. |

---

## 4. Key Components Implemented

### 1. Ingestion Gateway (`services/ingestion-gateway`)
- Accepts standard OpenTelemetry traces and logs on ports `4317` (gRPC) and `4318` (HTTP).
- Performs constant-time SHA-256 API key verification mapped to tenants.
- Enforces rate limits using a token bucket: returns `HTTP 429 Too Many Requests` or gRPC `Code(8) RESOURCE_EXHAUSTED`.
- Batches incoming payloads and pushes them onto NATS JetStream `telemetry.traces` and `telemetry.logs`.

### 2. Writer Service (`services/writer`)
- Pulls from NATS JetStream using durable consumer `writer-traces` and `writer-logs`.
- Decodes OTLP protobuf and JSON batches without memory allocations on hot paths.
- Performs bulk columnar inserts into ClickHouse (`observability.spans` and `observability.logs`).

### 3. Stream Processor (`services/stream-processor`)
- Concurrently reads `telemetry.traces` via durable consumer `proc-traces` with stream sequence deduplication.
- **Late-Arrival Safe Edge Correlation**: Correlates caller $\to$ callee edges across split NATS messages and out-of-order span deliveries with TTL-based buffering and deduplication (`edge_correlator.go`).
- Streams derived edges into `observability.edge_events`, continuously aggregated by ClickHouse materialized views (`observability.service_edges_1m_mv`).
- **Continuous Anomaly Production**: Evaluates rolling latency baselines, computes $Z$-scores ($\ge 3.0$), generates deterministic UUIDs for idempotency under redelivery, attributes root-cause spans, and writes `OPEN` and `RESOLVED` incidents to `observability.anomalies` (`anomaly_detector.go`).
- Computes sliding-window Rate, Error, and Duration percentiles (P50, P95, P99).

### 4. Query API (`services/query-api`)
- High-concurrency Go REST API with a read-only ClickHouse connection pool (`query_api` user).
- **Trace Engine**: Fast trace lookup via `observability.trace_index` for partition pruning.
- **Log Engine**: Token-bloom search over millions of log lines with trace-id correlation.
- **Live Tail**: Server-Sent Events (SSE) streaming live logs straight from NATS JetStream.
- **Topology & RED Metrics**: Real-time aggregated calls, error rates, and duration quantiles.

### 5. Model Context Protocol (MCP) Server (`services/mcp-server`)
- Exposes observability data to AI assistants (Claude, Antigravity, Cursor, Gemini):
  - `search_traces`: Search spans by service, operation, status, duration, and time range.
  - `get_trace`: Fetch all spans for a specific `trace_id`.
  - `query_logs`: Query logs with full-text search, regex, and trace correlation.
  - `get_service_topology`: Extract the live service dependency graph with RED metrics.
  - `list_anomalies`: List recent latency spikes and error rate deviations.
  - `explain_anomaly`: Correlate an anomaly to candidate root-cause spans.
- Dual-mode operation: standard input/output (`stdio`) or HTTP/SSE (port `8083`).

### 6. React 19 Web Dashboard (`apps/dashboard`)
- **D3 Distributed Flamegraph**: Interactive SVG/Canvas waterfall with parent-child nesting, dynamic time ruler, duration badges, and clock-skew correction.
- **Execution DAG View**: Graph-based trace visualization highlighting the critical latency path.
- **Service Dependency Graph**: Interactive topology powered by `@xyflow/react` showing node health and edge latency labels.
- **Log Explorer**: Live filtering with severity pills, search regex, and Live Tail toggle.
- **AI Copilot Panel**: Interactive chat with Gemini / MCP to inspect active incidents and suggest immediate code fixes.

---

## 5. Future Roadmap & Upcoming Features

The following items from the design specification are scheduled for upcoming development phases:

1. **Continuous Anomaly Detection Daemon**:
   - Standalone background worker running sliding baseline comparisons ($Z$-score $\ge 3.0$) and automatically populating `observability.anomalies`.
2. **Containerized 5-Service Demo & Chaos Injector**:
   - Pre-packaged demo workload with 5 simulated microservices (`frontend`, `auth`, `order`, `payment`, `notification`).
   - Scriptable k6 load generator with fault injection triggers (cascading database timeouts, payment gateway latency spikes, and 500 error cascades).
3. **100k+ Row TanStack Virtualization**:
   - Activating `@tanstack/react-virtual` for seamless smooth infinite scrolling of massive log outputs.
4. **Embedded Offline Engine (DuckDB Fallback)**:
   - Single-node embedded fallback for lightweight local debugging without running full ClickHouse.
5. **Full Multi-Tenant RBAC & OIDC Auth**:
   - Integration with OAuth2/OIDC providers (Google Workspace, GitHub, Okta) and team permission controls.

---

## 6. Getting Started

### Prerequisites
- Docker & Docker Compose
- Go 1.22+
- Node.js 20+ and npm

### 1. Start Infrastructure & Query API Platform Service
```bash
# Copy and configure environment variables
cp .env.example .env

# Launch ClickHouse, NATS JetStream, and Query API with healthchecks
docker compose -f docker-compose.infra.yml up -d

# Verify Query API readiness and dependency connectivity
curl -s http://127.0.0.1:8082/readyz
# Response: {"status":"READY","dependencies":{"clickhouse":"UP","nats":"UP"},"tenant":"demo"}
```

### 2. Run Telemetry Processing & Ingestion Services
```bash
# Run Ingestion Gateway (receives OTLP traces & logs on 4317/4318)
cd services/ingestion-gateway
go run ./cmd/gateway

# Run Writer Service (bulk inserts telemetry to ClickHouse)
cd services/writer
go run ./cmd/writer

# Run Stream Processor (derives topology edges & RED metrics)
cd services/stream-processor
go run .
```

### 3. Run MCP Server (Optional, for AI integration)
```bash
cd services/mcp-server
go run ./cmd/mcp-server -mode sse -port 8083
```

### 4. Run Frontend Dashboard
```bash
cd apps/dashboard
npm install
npm run dev
```
Open [http://localhost:5173](http://localhost:5173) in your browser.

---

## 7. License
Apache License 2.0. See [LICENSE](LICENSE) for details.
