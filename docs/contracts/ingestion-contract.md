# Ingestion and storage contract (v1, owner: feature/ingestion)

Frozen once Anushka and Meenakshi sign off. Changes go through a PR touching this file.

## 1. NATS JetStream

| Item | Value |
|---|---|
| Stream `TELEMETRY` | subjects `telemetry.traces`, `telemetry.logs`; file storage; limits retention; 24h / 10 GiB; `discard: new` |
| Stream `TELEMETRY_DLQ` | subjects `telemetry.dlq.>`; 7d / 1 GiB; `discard: old` |
| Duplicate window | 120 s (publisher sets `Nats-Msg-Id`) |
| Max message size | 4 MiB (gateway splits larger batches) |

**Payload:** standard OTLP protobuf: `ExportTraceServiceRequest` on `telemetry.traces`,
`ExportLogsServiceRequest` on `telemetry.logs`. A message may contain several merged client requests
(several `ResourceSpans` / `ResourceLogs`).

**Headers (set by gateway):**
- `Nats-Msg-Id`: `<tenant_id>:<sha256 of payload, hex>`
- `X-Tenant-Id`: tenant resolved from the API key
- `X-Received-At`: RFC3339Nano gateway receive time
- `Content-Type`: `application/x-protobuf`

**Consumers:** each service creates its own durable pull consumer (`CreateOrUpdateConsumer`), explicit ack,
`ack_wait` 30 s, `max_deliver` 10.
- writer: `writer-traces`, `writer-logs` (same durable name across replicas = work sharing)
- processing: `proc-traces`, `proc-logs` (separate durables, so both see every message)

**Dead letter:** a message that cannot be decoded or hits `max_deliver` is republished by its consumer to
`telemetry.dlq.<traces|logs>` with headers `X-Error`, `X-Original-Subject`, `X-Delivery-Count`, then acked.

**Delivery semantics:** at-least-once. Consumers must be idempotent.

## 2. ClickHouse (`infra/clickhouse/init/001_schema.sql`)

Written by the writer: `spans`, `logs`.
Derived by materialized views (no application writes): `trace_index`, `latency_1m`, `service_edges_1m`.
Proposed for processing: `edge_events` (insert), `anomalies` (insert/read).

**OTLP to row mapping (writer):**
- `service_name` from resource attribute `service.name`, else `unknown_service`.
- `trace_id`/`span_id`/`parent_span_id` lowercase hex; empty string when absent or all zeros.
- `start_time`/`timestamp` in nanoseconds; `duration_ns = end - start`, clamped at 0.
- Attribute values are stringified (ints, bools, doubles as canonical text; arrays/maps as JSON).
- `http_status_code` from `http.response.status_code`, else `http.status_code`, else 0.
- Logs with `timestamp = 0` use `observed_time`, else the message's `X-Received-At`.

**Reading guidance (query API / processor):**
- `spans` is a ReplacingMergeTree: dedup happens at merge time, so for exact reads of one trace use
  `argMax(..., ingested_at)` grouped by `(trace_id, span_id)` or `FINAL`. Aggregates read the rollup tables.
- Trace by ID: read `trace_index` for `first_seen`/`last_seen`, then query `spans` with a `start_time` bound.
- Rollups: `quantilesTDigestMerge(0.5, 0.95, 0.99)(duration_q)`, `sum(calls)`, `sum(errors)`.
- Query API connects as `query_api` (read-only, resource limits). Writer connects as `writer`.
