#!/usr/bin/env bash
# Generates an API key for a tenant and appends its SHA-256 digest to the key file.
# The plaintext key is printed once and never stored. Usage: scripts/gen-api-key.sh <tenant_id> [keyfile]
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
tenant="${1:?usage: gen-api-key.sh <tenant_id> [keyfile]}"
file="${2:-$ROOT/.data/api-keys.txt}"
[[ "$tenant" =~ ^[a-z0-9][a-z0-9_-]{0,63}$ ]] || { echo "tenant_id must match ^[a-z0-9][a-z0-9_-]{0,63}$" >&2; exit 1; }

key="eoa_$(openssl rand -hex 24)"
if command -v sha256sum >/dev/null; then digest="$(printf %s "$key" | sha256sum | cut -d' ' -f1)"
else digest="$(printf %s "$key" | shasum -a 256 | cut -d' ' -f1)"; fi

mkdir -p "$(dirname "$file")"
umask 077
echo "$tenant $digest" >> "$file"
echo "tenant:  $tenant"
echo "API key: $key   (shown once; send as 'X-API-Key' or 'Authorization: Bearer')"
echo "digest appended to $file (send SIGHUP to a running gateway to reload)"
