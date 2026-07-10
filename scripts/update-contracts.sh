#!/usr/bin/env bash
# scripts/update-contracts.sh — scarica gli OpenAPI ufficiali Atlassian.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p docs/contracts

command -v curl >/dev/null 2>&1 || { echo "curl required" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq required" >&2; exit 1; }

CURL_OPTS=(-fsSL --retry 3 --retry-delay 2 --max-time 120)

# download_spec <url> <dest> <min_paths>
# Scarica su file temporaneo, valida il contenuto, poi sposta atomicamente.
download_spec() {
  local url="$1" dest="$2" min_paths="$3"
  local tmp
  tmp="$(mktemp)"

  if ! curl "${CURL_OPTS[@]}" "$url" -o "$tmp"; then
    rm -f "$tmp"
    echo "ERROR: download failed for $url" >&2
    exit 1
  fi

  if ! jq -e ".paths | length > $min_paths" "$tmp" >/dev/null 2>&1; then
    rm -f "$tmp"
    echo "ERROR: $url did not return a valid OpenAPI spec with > $min_paths paths" >&2
    exit 1
  fi

  mv "$tmp" "$dest"
}

download_spec "https://developer.atlassian.com/cloud/jira/platform/swagger-v3.v3.json" \
  docs/contracts/jira-platform-v3.json 300
download_spec "https://developer.atlassian.com/cloud/jira/software/swagger.v3.json" \
  docs/contracts/jira-agile-1.0.json 40

echo "Platform paths: $(jq '.paths | length' docs/contracts/jira-platform-v3.json)"
echo "Agile paths:    $(jq '.paths | length' docs/contracts/jira-agile-1.0.json)"
