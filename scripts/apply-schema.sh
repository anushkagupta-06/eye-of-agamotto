#!/usr/bin/env bash
# Applies every infra/clickhouse/init/*.sql, in order, to a RUNNING ClickHouse container.
# The image only runs init scripts on an empty data volume, so use this after schema changes.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
if [[ -f .env ]]; then set -a; . ./.env; set +a; fi
: "${CH_ADMIN_PASSWORD:?set CH_ADMIN_PASSWORD in .env}"
for f in infra/clickhouse/init/*.sql; do
  echo "applying $f"
  docker compose -f docker-compose.infra.yml exec -T clickhouse \
    clickhouse-client --user admin --password "$CH_ADMIN_PASSWORD" --multiquery < "$f"
done
echo "schema applied"
