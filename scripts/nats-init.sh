#!/bin/sh
# Idempotently creates/updates JetStream streams. Runs inside the natsio/nats-box container.
# The nats CLI has no --token flag, so the token travels as URL userinfo (nats://TOKEN@host).
# Keep NATS_AUTH_TOKEN URL-safe (letters, digits, - _ .), e.g. output of `openssl rand -hex 24`.
set -eu
: "${NATS_AUTH_TOKEN:?NATS_AUTH_TOKEN is required}"
NATS_HOST="${NATS_HOST:-nats:4222}"
SERVER="nats://${NATS_AUTH_TOKEN}@${NATS_HOST}"

for cfg in /streams/*.json; do
  name="$(sed -n 's/.*"name": *"\([^"]*\)".*/\1/p' "$cfg" | head -n1)"
  echo "applying $(basename "$cfg") ($name)"
  if nats --server "$SERVER" stream info "$name" >/dev/null 2>&1; then
    nats --server "$SERVER" stream edit --force --config "$cfg" "$name"
  else
    nats --server "$SERVER" stream add --config "$cfg"
  fi
done
echo "streams ready"