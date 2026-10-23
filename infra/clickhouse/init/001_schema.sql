-- Eye of Agamotto: ClickHouse storage contract.
-- Owner: ingestion/storage (feature/ingestion). Tables marked [processing] are written/read
-- by the stream processor and are PROPOSED: Anushka to confirm before freeze.
-- All statements are idempotent (IF NOT EXISTS). Retention is edited via ALTER ... MODIFY TTL.

CREATE DATABASE IF NOT EXISTS observability;

-- ---------------------------------------------------------------- spans
CREATE TABLE IF NOT EXISTS observability.spans
(
    tenant_id           LowCardinality(String),
    trace_id            String CODEC(ZSTD(1)),            -- 32 lowercase hex chars
    span_id             String CODEC(ZSTD(1)),            -- 16 lowercase hex chars
    parent_span_id      String CODEC(ZSTD(1)),            -- '' for root spans
    service_name        LowCardinality(String),
    name                LowCardinality(String),
    kind                Enum8('UNSPECIFIED' = 0, 'INTERNAL' = 1, 'SERVER' = 2, 'CLIENT' = 3, 'PRODUCER' = 4, 'CONSUMER' = 5),
    start_time          DateTime64(9) CODEC(Delta(8), ZSTD(1)),
    duration_ns         UInt64 CODEC(T64, ZSTD(1)),
    status_code         Enum8('UNSET' = 0, 'OK' = 1, 'ERROR' = 2),
    status_message      String CODEC(ZSTD(1)),
    http_status_code    UInt16,                           -- 0 when not an HTTP span
    attributes          Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    resource_attributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ingested_at         DateTime DEFAULT now(),

    INDEX idx_trace_id  trace_id                      TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_attr_key  mapKeys(attributes)           TYPE bloom_filter(0.01)  GRANULARITY 1,
    INDEX idx_attr_val  mapValues(attributes)         TYPE bloom_filter(0.01)  GRANULARITY 1,
    INDEX idx_res_key   mapKeys(resource_attributes)  TYPE bloom_filter(0.01)  GRANULARITY 1,
    INDEX idx_res_val   mapValues(resource_attributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_duration  duration_ns                   TYPE minmax              GRANULARITY 1
)
ENGINE = ReplacingMergeTree(ingested_at)
PARTITION BY toDate(start_time)
ORDER BY (tenant_id, service_name, start_time, trace_id, span_id)
TTL toDateTime(start_time) + INTERVAL 7 DAY DELETE
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- Trace-by-ID lookup. spans is sorted by service/time, so trace_id is NOT a primary-key prefix.
-- This tiny table maps trace_id -> time window; query API reads it, then queries spans with a
-- start_time bound so partitions and granules are pruned.
CREATE TABLE IF NOT EXISTS observability.trace_index
(
    tenant_id  LowCardinality(String),
    trace_id   String,
    first_seen SimpleAggregateFunction(min, DateTime64(9)),
    last_seen  SimpleAggregateFunction(max, DateTime64(9)),
    last_day   SimpleAggregateFunction(max, Date)
)
ENGINE = AggregatingMergeTree
ORDER BY (tenant_id, trace_id)
TTL last_day + INTERVAL 7 DAY DELETE;

CREATE MATERIALIZED VIEW IF NOT EXISTS observability.trace_index_mv
TO observability.trace_index AS
SELECT tenant_id, trace_id,
       min(start_time)          AS first_seen,
       max(start_time)          AS last_seen,
       max(toDate(start_time))  AS last_day
FROM observability.spans
GROUP BY tenant_id, trace_id;

-- ---------------------------------------------------------------- logs
CREATE TABLE IF NOT EXISTS observability.logs
(
    tenant_id           LowCardinality(String),
    timestamp           DateTime64(9) CODEC(Delta(8), ZSTD(1)),
    service_name        LowCardinality(String),
    severity_text       LowCardinality(String),
    severity_number     UInt8,                            -- OTLP 1..24, 0 = unspecified
    body                String CODEC(ZSTD(3)),
    trace_id            String CODEC(ZSTD(1)),            -- '' when not correlated
    span_id             String CODEC(ZSTD(1)),
    attributes          Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    resource_attributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ingested_at         DateTime DEFAULT now(),

    INDEX idx_body     body                     TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 1,
    INDEX idx_trace_id trace_id                 TYPE bloom_filter(0.001)     GRANULARITY 1,
    INDEX idx_attr_key mapKeys(attributes)      TYPE bloom_filter(0.01)      GRANULARITY 1,
    INDEX idx_attr_val mapValues(attributes)    TYPE bloom_filter(0.01)      GRANULARITY 1
)
ENGINE = MergeTree
PARTITION BY toDate(timestamp)
ORDER BY (tenant_id, service_name, timestamp)
TTL toDateTime(timestamp) + INTERVAL 7 DAY DELETE
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- ---------------------------------------------------------------- rollups
-- Per-service/operation RED rollup, maintained incrementally on every spans insert.
-- kind is part of the key so the consumer decides which kinds count (usually SERVER/CONSUMER).
CREATE TABLE IF NOT EXISTS observability.latency_1m
(
    tenant_id    LowCardinality(String),
    service_name LowCardinality(String),
    operation    LowCardinality(String),
    kind         Enum8('UNSPECIFIED' = 0, 'INTERNAL' = 1, 'SERVER' = 2, 'CLIENT' = 3, 'PRODUCER' = 4, 'CONSUMER' = 5),
    minute       DateTime,
    calls        SimpleAggregateFunction(sum, UInt64),
    errors       SimpleAggregateFunction(sum, UInt64),
    duration_q   AggregateFunction(quantilesTDigest(0.5, 0.95, 0.99), UInt64)
)
ENGINE = AggregatingMergeTree
PARTITION BY toDate(minute)
ORDER BY (tenant_id, service_name, operation, kind, minute)
TTL minute + INTERVAL 30 DAY DELETE
SETTINGS ttl_only_drop_parts = 1;

CREATE MATERIALIZED VIEW IF NOT EXISTS observability.latency_1m_mv
TO observability.latency_1m AS
SELECT tenant_id,
       service_name,
       name                                     AS operation,
       kind,
       toStartOfMinute(toDateTime(start_time))  AS minute,
       count()                                  AS calls,
       countIf(status_code = 'ERROR')           AS errors,
       quantilesTDigestState(0.5, 0.95, 0.99)(duration_ns) AS duration_q
FROM observability.spans
GROUP BY tenant_id, service_name, operation, kind, minute;

-- [processing, PROPOSED] A caller->callee edge needs a child span joined to its parent span,
-- which may arrive in a different insert block, so it cannot be a materialized view on spans.
-- The stream processor derives edges and inserts observations here; the MV rolls them up.
CREATE TABLE IF NOT EXISTS observability.edge_events
(
    tenant_id   LowCardinality(String),
    caller      LowCardinality(String),
    callee      LowCardinality(String),
    ts          DateTime64(3),
    duration_ns UInt64,
    is_error    UInt8
)
ENGINE = MergeTree
PARTITION BY toDate(ts)
ORDER BY (tenant_id, caller, callee, ts)
TTL toDateTime(ts) + INTERVAL 2 DAY DELETE
SETTINGS ttl_only_drop_parts = 1;

CREATE TABLE IF NOT EXISTS observability.service_edges_1m
(
    tenant_id  LowCardinality(String),
    caller     LowCardinality(String),
    callee     LowCardinality(String),
    minute     DateTime,
    calls      SimpleAggregateFunction(sum, UInt64),
    errors     SimpleAggregateFunction(sum, UInt64),
    duration_q AggregateFunction(quantilesTDigest(0.5, 0.95, 0.99), UInt64)
)
ENGINE = AggregatingMergeTree
PARTITION BY toDate(minute)
ORDER BY (tenant_id, caller, callee, minute)
TTL minute + INTERVAL 30 DAY DELETE
SETTINGS ttl_only_drop_parts = 1;

CREATE MATERIALIZED VIEW IF NOT EXISTS observability.service_edges_1m_mv
TO observability.service_edges_1m AS
SELECT tenant_id, caller, callee,
       toStartOfMinute(toDateTime(ts)) AS minute,
       count()                         AS calls,
       sum(is_error)                   AS errors,
       quantilesTDigestState(0.5, 0.95, 0.99)(duration_ns) AS duration_q
FROM observability.edge_events
GROUP BY tenant_id, caller, callee, minute;

-- [processing, PROPOSED]
CREATE TABLE IF NOT EXISTS observability.anomalies
(
    tenant_id       LowCardinality(String),
    id              UUID,
    service_name    LowCardinality(String),
    operation       LowCardinality(String),
    metric          LowCardinality(String),               -- e.g. p99_latency, error_rate
    window_start    DateTime,
    window_end      DateTime,
    baseline_value  Float64,
    observed_value  Float64,
    z_score         Float64,
    root_cause_service LowCardinality(String),
    evidence        String CODEC(ZSTD(1)),                -- JSON, explain_anomaly payload
    status          Enum8('OPEN' = 0, 'RESOLVED' = 1),
    updated_at      DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toDate(window_start)
ORDER BY (tenant_id, id)
TTL window_start + INTERVAL 30 DAY DELETE
SETTINGS ttl_only_drop_parts = 1;
