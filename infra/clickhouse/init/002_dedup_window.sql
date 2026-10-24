-- The writer inserts with insert_deduplication_token so a retried identical batch is dropped.
-- Non-replicated MergeTree tables only honour that when a deduplication window is configured
-- (the default is 0 = off), including the tables fed by materialized views.
-- Idempotent: safe to run repeatedly, on fresh and existing volumes.
ALTER TABLE observability.spans       MODIFY SETTING non_replicated_deduplication_window = 1000;
ALTER TABLE observability.logs        MODIFY SETTING non_replicated_deduplication_window = 1000;
ALTER TABLE observability.trace_index MODIFY SETTING non_replicated_deduplication_window = 1000;
ALTER TABLE observability.latency_1m  MODIFY SETTING non_replicated_deduplication_window = 1000;
