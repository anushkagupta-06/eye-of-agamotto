#!/usr/bin/env bash
# Generates realistic multi-service OTLP traces and logs into Eye of Agamotto.
# Simulates microservices: api-gateway -> auth-service, order-service -> payment-service, inventory-service
set -euo pipefail

URL="${EOA_GATEWAY_URL:-http://127.0.0.1:4318}"
KEY="${EOA_API_KEY:-eoa_cffdedbba2a3d4f0ecb4706923ae02d5d4b2ac1a0c03ff44}"
COUNT="${1:-10}" # Number of trace batches to send (pass a number or "loop")

generate_hex() {
  local len=$1
  od -vN "$((len/2))" -An -tx1 /dev/urandom | tr -d ' \n'
}

echo "🚀 Sending $COUNT synthetic microservice trace batches to $URL..."

send_batch() {
  local now_sec=$(date +%s)
  local now_nano="${now_sec}000000000"
  local trace_id=$(generate_hex 32)
  local span_gw=$(generate_hex 16)
  local span_auth=$(generate_hex 16)
  local span_order=$(generate_hex 16)
  local span_pay=$(generate_hex 16)

  # 1. API Gateway Span
  local payload='{
    "resourceSpans": [
      {
        "resource": {
          "attributes": [
            {"key": "service.name", "value": {"stringValue": "api-gateway"}},
            {"key": "deployment.environment", "value": {"stringValue": "production"}}
          ]
        },
        "scopeSpans": [{
          "spans": [{
            "traceId": "'$trace_id'",
            "spanId": "'$span_gw'",
            "name": "POST /checkout",
            "kind": 2,
            "startTimeUnixNano": "'$now_nano'",
            "endTimeUnixNano": "'$((now_sec + 1))'000000000",
            "status": {"code": 1}
          }]
        }]
      },
      {
        "resource": {
          "attributes": [
            {"key": "service.name", "value": {"stringValue": "auth-service"}},
            {"key": "deployment.environment", "value": {"stringValue": "production"}}
          ]
        },
        "scopeSpans": [{
          "spans": [{
            "traceId": "'$trace_id'",
            "spanId": "'$span_auth'",
            "parentSpanId": "'$span_gw'",
            "name": "POST /v1/verify-token",
            "kind": 2,
            "startTimeUnixNano": "'$now_nano'",
            "endTimeUnixNano": "'$((now_sec + 1))'000000000",
            "status": {"code": 1}
          }]
        }]
      },
      {
        "resource": {
          "attributes": [
            {"key": "service.name", "value": {"stringValue": "order-service"}},
            {"key": "deployment.environment", "value": {"stringValue": "production"}}
          ]
        },
        "scopeSpans": [{
          "spans": [{
            "traceId": "'$trace_id'",
            "spanId": "'$span_order'",
            "parentSpanId": "'$span_gw'",
            "name": "POST /orders/create",
            "kind": 2,
            "startTimeUnixNano": "'$now_nano'",
            "endTimeUnixNano": "'$((now_sec + 1))'000000000",
            "status": {"code": 1}
          }]
        }]
      },
      {
        "resource": {
          "attributes": [
            {"key": "service.name", "value": {"stringValue": "payment-service"}},
            {"key": "deployment.environment", "value": {"stringValue": "production"}}
          ]
        },
        "scopeSpans": [{
          "spans": [{
            "traceId": "'$trace_id'",
            "spanId": "'$span_pay'",
            "parentSpanId": "'$span_order'",
            "name": "POST /payments/charge",
            "kind": 2,
            "startTimeUnixNano": "'$now_nano'",
            "endTimeUnixNano": "'$((now_sec + 1))'000000000",
            "status": {"code": 1}
          }]
        }]
      }
    ]
  }'

  curl -s -X POST "$URL/v1/traces" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $KEY" \
    -d "$payload" > /dev/null

  # Also send a companion log
  local log_payload='{
    "resourceLogs": [{
      "resource": {
        "attributes": [
          {"key": "service.name", "value": {"stringValue": "payment-service"}}
        ]
      },
      "scopeLogs": [{
        "logRecords": [{
          "timeUnixNano": "'$now_nano'",
          "severityText": "INFO",
          "severityNumber": 9,
          "body": {"stringValue": "Transaction settled successfully for trace '$trace_id'"},
          "traceId": "'$trace_id'",
          "spanId": "'$span_pay'"
        }]
      }]
    }]
  }'

  curl -s -X POST "$URL/v1/logs" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $KEY" \
    -d "$log_payload" > /dev/null

  echo "  ✓ Emitted distributed trace $trace_id (api-gateway -> auth -> order -> payment)"
}

if [ "$COUNT" = "loop" ]; then
  echo "🔁 Running continuous traffic loop (press Ctrl+C to stop)..."
  while true; do
    send_batch
    sleep 2
  done
else
  for i in $(seq 1 "$COUNT"); do
    send_batch
    sleep 0.5
  done
fi

echo "✨ Done! Check your Eye of Agamotto dashboard at http://localhost:5173"
