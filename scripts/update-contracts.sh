#!/usr/bin/env bash
# scripts/update-contracts.sh — scarica gli OpenAPI ufficiali Atlassian.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p docs/contracts

curl -fsSL "https://developer.atlassian.com/cloud/jira/platform/swagger-v3.v3.json" \
  -o docs/contracts/jira-platform-v3.json
curl -fsSL "https://developer.atlassian.com/cloud/jira/software/swagger.v3.json" \
  -o docs/contracts/jira-agile-1.0.json

echo "Platform paths: $(jq '.paths | length' docs/contracts/jira-platform-v3.json)"
echo "Agile paths:    $(jq '.paths | length' docs/contracts/jira-agile-1.0.json)"
