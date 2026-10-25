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
`ack_wait` 60 s, unlimited `max_deliver` (an unreachable database must never cause messages to be
abandoned; poison messages are removed explicitly, see below).
- writer: `writer-traces`, `writer-logs` (same durable name across replicas = work sharing)
- processing: `proc-traces`, `proc-logs` (separate durables, so both see every message)

**Dead letter:** a message that cannot be decoded, has no `X-Tenant-Id`, or is rejected by the database for
data reasons (isolated by bisecting the batch) is republished by its consumer to `telemetry.dlq.<traces|logs>`
with headers `X-Error`, `X-Stage` (`decode` or `insert`), `X-Original-Subject`, `X-Delivery-Count`,
`X-Tenant-Id`, and then terminated (`Term`) so it is not redelivered. The payload is the original, unmodified
message, so it can be replayed after a fix. Database outages never dead-letter: those retry with backoff.

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

## 3. Writer behaviour (services/writer)

- **Batching:** each worker fetches up to 32 messages (or whatever arrives within 1 s) and performs one bulk
  insert for all of them. Workers per signal: 2 by default.
- **Ack after insert:** messages are acknowledged only once the insert succeeded.
- **Retry:** a failed insert negatively acknowledges the whole batch with a delay that doubles per delivery
  (1 s, 2 s, 4 s ... capped at 30 s). Data stays in the stream (24 h retention) while ClickHouse is down.
- **Deduplication:** each insert carries `insert_deduplication_token = <signal>-<hash of stream sequences>`, so
  redelivering the same messages together is dropped by ClickHouse (see `002_dedup_window.sql`). If messages
  are redelivered in a different grouping, `spans` is deduplicated at merge time by `ReplacingMergeTree`;
  `logs` has no natural key, so rare duplicate log rows are possible after a crash. This is the accepted
  at-least-once trade-off.
- **Timestamps:** a log with neither `time_unix_nano` nor `observed_time_unix_nano` gets the gateway's
  `X-Received-At`.
- **Attribute encoding:** scalars as text, arrays and maps as JSON, bytes as base64. Instrumentation scope
  name/version, span events and span links are not stored in v1.
