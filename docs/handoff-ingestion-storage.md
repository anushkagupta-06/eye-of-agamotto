# Handoff: ingestion and storage 

Everything between "an app emits telemetry" and "rows sit in ClickHouse" works end to end,
including the failure paths. Details of the queue and table contracts are in
[`docs/contracts/ingestion-contract.md`](contracts/ingestion-contract.md); this page is the practical summary.

## 1. Run it from root

Each new terminal needs `.env` loaded first (variables are per terminal):

```powershell
Get-Content .env | ForEach-Object { if ($_ -match '^\s*([^#=\s]+)=(.*)$') { [Environment]::SetEnvironmentVariable($Matches[1], $Matches[2]) } }
```

```powershell
# 1. one-time: copy .env.example to .env and set four different long random values
# 2. infrastructure (NATS + ClickHouse); the first start creates the tables and streams
docker compose -f docker-compose.infra.yml --env-file .env up -d
docker compose -f docker-compose.infra.yml logs nats-init        # should end with "streams ready"

# 3. one-time: create your own API key (shown once, only its hash is stored)
$bytes = New-Object byte[] 24; [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
$key = "eoa_" + (($bytes | ForEach-Object { $_.ToString("x2") }) -join "")
$digest = ([Security.Cryptography.SHA256]::Create().ComputeHash([Text.Encoding]::UTF8.GetBytes($key)) | ForEach-Object { $_.ToString("x2") }) -join ""
New-Item -ItemType Directory -Force .data | Out-Null
[IO.File]::AppendAllText("$PWD\.data\api-keys.txt", "demo $digest`n"); $key

# 4. gateway and writer, each in its own terminal (.env loaded in both)
$env:GATEWAY_API_KEYS_FILE = "$PWD\.data\api-keys.txt"
cd services; go run ./ingestion-gateway/cmd/gateway
cd services; go run ./writer/cmd/writer
```

Point an instrumented service at the gateway with the standard OpenTelemetry variables:

```
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318        (HTTP/protobuf; gRPC is on :4317)
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_EXPORTER_OTLP_HEADERS=x-api-key=<your key>
OTEL_SERVICE_NAME=checkout                                (becomes service_name)
```

| What | Address |
|---|---|
| Gateway OTLP gRPC / HTTP | `:4317` / `:4318` (`/v1/traces`, `/v1/logs`) |
| Gateway ops / Writer ops | `:8080` / `:8081` (`/livez`, `/readyz`, `/metrics`) |
| NATS client / monitoring | `127.0.0.1:4222` / `:8222` (token auth, `NATS_AUTH_TOKEN`) |
| ClickHouse HTTP / native | `127.0.0.1:8123` / `:9000` |

ClickHouse users: **`query_api`** is read-only with resource limits (password `CH_READER_PASSWORD`); use it for
everything that only reads. `writer` can insert (`CH_WRITER_PASSWORD`). `admin` is for schema work only.

## 2. Tables (database `observability`)

| Table | Written by | Notes |
|---|---|---|
| `spans` | writer | one row per span, sorted by tenant, service, start time |
| `logs` | writer | full-text token index on `body`, bloom indexes on tags |
| `trace_index` | automatic | trace id to time window, used for trace-by-id |
| `latency_1m` | automatic | per service and operation per minute: calls, errors, latency sketch |
| `edge_events` | **processor** | proposed: one row per caller-to-callee call |
| `service_edges_1m` | automatic | per edge per minute, built from `edge_events` |
| `anomalies` | **processor** | proposed: one row per detected anomaly |

Edges cannot be derived inside ClickHouse at insert time (the
parent span can arrive in a different batch), so the processor derives them: a `SERVER` span whose parent is a span of
a *different* service gives the edge `parent.service -> child.service`; insert it into `edge_events` and the rollup
fills itself.

## 3. Rules for reading the data

1. **Always filter `tenant_id` and a time range.** Both drive partition and index pruning. Queries without a time
   bound scan the full retention (7 days raw data, 30 days rollups).
2. **Duplicates are possible** (at-least-once delivery). For one trace use `ORDER BY ingested_at DESC LIMIT 1 BY
   span_id` as below. For aggregates read the rollup tables; they are deduplicated at insert time.
3. **Time is `DateTime64(9)`**, nanoseconds. `duration_ns` is nanoseconds. Ids are lowercase hex (trace 32 chars,
   span 16 chars); a root span has `parent_span_id = ''`.
4. **Always send an explicit `LIMIT`.** The `query_api` user silently truncates at 100,000 rows and aborts queries
   after 10 seconds.
5. **Use query parameters** (`{name:Type}`), never string concatenation. Regex search (`match`) cannot use the index:
   combine it with tenant, service and a short time range.
6. Rollup percentiles: `quantilesTDigestMerge(0.5, 0.95, 0.99)(duration_q)` returns `[p50, p95, p99]` in ns.

## 4. Example queries

Written against the schema but **not yet executed**; run each once and tell me if any fails. Quick CLI check:

```powershell
docker compose -f docker-compose.infra.yml exec clickhouse clickhouse-client --user query_api --password $env:CH_READER_PASSWORD --param_tenant=demo -q "SELECT count() FROM observability.spans WHERE tenant_id = {tenant:String}"
```

**Trace by id (flamegraph).** Two steps: find the time window, then fetch the spans.

```sql
SELECT min(first_seen) AS t0, max(last_seen) AS t1
FROM trace_index
WHERE tenant_id = {tenant:String} AND trace_id = {trace:String};

SELECT span_id, parent_span_id, service_name, name, kind, start_time, duration_ns,
       status_code, status_message, http_status_code, attributes
FROM spans
WHERE tenant_id = {tenant:String} AND trace_id = {trace:String}
  AND start_time BETWEEN {t0:DateTime64(9)} AND {t1:DateTime64(9)}
ORDER BY ingested_at DESC
LIMIT 1 BY span_id;
```

Build the tree from `parent_span_id`, not from timestamps (clock skew between hosts). A span whose parent is missing
belongs under a synthetic root.

**Slowest and failing traces of a service (click a node in the service map).**

```sql
SELECT trace_id, max(duration_ns) AS slowest_span_ns,
       countIf(status_code = 'ERROR') AS error_spans, min(start_time) AS started
FROM spans
WHERE tenant_id = {tenant:String} AND service_name = {service:String}
  AND start_time >= now() - INTERVAL 15 MINUTE
GROUP BY trace_id
ORDER BY slowest_span_ns DESC          -- failing instead: add HAVING error_spans > 0, ORDER BY started DESC
LIMIT 20;
```

**Logs of one trace ("View Logs" from a span).**

```sql
SELECT timestamp, service_name, severity_text, body, span_id
FROM logs
WHERE tenant_id = {tenant:String} AND trace_id = {trace:String}
  AND timestamp BETWEEN {t0:DateTime64(9)} - INTERVAL 1 MINUTE AND {t1:DateTime64(9)} + INTERVAL 5 MINUTE
ORDER BY timestamp
LIMIT 1000;
```

**Log explorer: tags, severity, full text.**

```sql
SELECT timestamp, service_name, severity_text, body, trace_id
FROM logs
WHERE tenant_id = {tenant:String}
  AND timestamp >= now() - INTERVAL 15 MINUTE
  AND service_name = {service:String}
  AND severity_number >= 17                    -- 17 = ERROR and above
  AND hasToken(body, {word:String})            -- indexed whole-word search, case-sensitive
  AND attributes['pod'] = {pod:String}         -- any tag, no schema change needed
ORDER BY timestamp DESC
LIMIT 200;
```

**Live tail** is served from NATS, not ClickHouse. Create an *ephemeral* consumer on `telemetry.logs` with
`DeliverNew` (never reuse the `writer-*` or `proc-*` durables), filter by the `X-Tenant-Id` message header, and decode
the OTLP payload.

**RED metrics of a service (health tiles, anomaly baseline).**

```sql
SELECT minute, sum(calls) AS calls, sum(errors) AS errors,
       quantilesTDigestMerge(0.5, 0.95, 0.99)(duration_q) AS p50_p95_p99_ns
FROM latency_1m
WHERE tenant_id = {tenant:String} AND service_name = {service:String}
  AND kind IN ('SERVER', 'CONSUMER')           -- entry spans only, so calls are not double counted
  AND minute >= now() - INTERVAL 1 HOUR
GROUP BY minute
ORDER BY minute;
```

Rate is `calls / 60` per second, error rate is `errors / calls`. For a baseline, run the same query over a longer
window (for example 24 hours) and compare against the latest minutes.

**Service map edges.**

```sql
SELECT caller, callee, sum(calls) AS calls, sum(errors) AS errors,
       quantilesTDigestMerge(0.5, 0.95, 0.99)(duration_q) AS p50_p95_p99_ns
FROM service_edges_1m
WHERE tenant_id = {tenant:String} AND minute >= now() - INTERVAL 15 MINUTE
GROUP BY caller, callee;
```

## 5. For the stream processor

- Consume `telemetry.traces` and `telemetry.logs` with **durable consumers** `proc-traces` and `proc-logs`
  (pull, explicit ack, unlimited `max_deliver`), so every message independently of the writer.
- Payload is a standard OTLP protobuf (`ExportTraceServiceRequest` / `ExportLogsServiceRequest`); the tenant is in the
  `X-Tenant-Id` header. Delivery is at-least-once, so make it idempotent.
- The OTLP-to-row conversion lives in `services/writer/internal/transform`. Because it is `internal`, outside modules cannot
  import it. If you want it, say so and I will move `model` and `transform` to a shared package.
- For write access: use the `writer` ClickHouse user for `edge_events` and `anomalies` for now.


## 6. Open decisions

1. In processing pipeline confirm or change `edge_events` and `anomalies`.
2. Shared decoding code: keep it private to the writer, or move it to a shared package for the processor?
3. A dedicated `processor` ClickHouse user instead of reusing `writer` (small hardening task).
4. future improvement: instrumentation scope name and version, span events, span links.
