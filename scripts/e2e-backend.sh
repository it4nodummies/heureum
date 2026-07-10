#!/usr/bin/env bash
# scripts/e2e-backend.sh — avvia il backend su SQLite effimero seedato, porta 8080.
set -euo pipefail
cd "$(dirname "$0")/.."
export APP_SECRET="${APP_SECRET:-e2e-secret}"
export DB_DRIVER=sqlite
export DB_DSN="${DB_DSN:-/tmp/openjira-e2e.db}"
rm -f "$DB_DSN"
go run ./cmd/seed
exec go run ./cmd/server
