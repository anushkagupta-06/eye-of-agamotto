#!/usr/bin/env bash
# Sends one OTLP/HTTP JSON span and one log to a running gateway. Usage: EOA_API_KEY=... scripts/smoke-ingest.sh
set -euo pipefail
URL="${EOA_GATEWAY_URL:-http://127.0.0.1:4318}"
KEY="${EOA_API_KEY:?set EOA_API_KEY}"
now="$(date +%s)000000000"
trace="5b8efff798038103d269b633813fc60c"; span="eee19b7ec3c1b174"

post() { # path body
  curl -sS -o /dev/stderr -w "%{http_code}\n" -X POST "$URL$1" \
    -H "Content-Type: application/json" -H "X-API-Key: $KEY" -d "$2"
}

echo "traces:"; post /v1/traces '{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"smoke-test"}}]},"scopeSpans":[{"spans":[{"traceId":"'$trace'","spanId":"'$span'","name":"GET /smoke","kind":2,"startTimeUnixNano":"'$now'","endTimeUnixNano":"'$now'"}]}]}]}'
echo "logs:";   post /v1/logs   '{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"smoke-test"}}]},"scopeLogs":[{"logRecords":[{"timeUnixNano":"'$now'","severityText":"INFO","body":{"stringValue":"hello from smoke test"},"traceId":"'$trace'","spanId":"'$span'"}]}]}]}'
echo "partial-success check (bad trace id, expect HTTP 200 with rejectedSpans=1):"
post /v1/traces '{"resourceSpans":[{"scopeSpans":[{"spans":[{"traceId":"abcd","spanId":"'$span'","name":"bad","startTimeUnixNano":"'$now'"}]}]}]}'
