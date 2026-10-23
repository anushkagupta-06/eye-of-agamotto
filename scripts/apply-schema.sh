#!/usr/bin/env bash
# Re-applies infra/clickhouse/init/001_schema.sql to a RUNNING ClickHouse container.
# Needed after you change the schema, because the image only runs init scripts on an empty volume.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
if [[ -f .env ]]; then set -a; . ./.env; set +a; fi
: "${CH_ADMIN_PASSWORD:?set CH_ADMIN_PASSWORD in .env}"
docker compose -f docker-compose.infra.yml exec -T clickhouse \
  clickhouse-client --user admin --password "$CH_ADMIN_PASSWORD" --multiquery \
  < infra/clickhouse/init/001_schema.sql
echo "schema applied"
