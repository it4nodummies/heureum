#!/usr/bin/env bash
# scripts/e2e-backend.sh — avvia il backend su SQLite effimero seedato, porta 8080.
set -euo pipefail
cd "$(dirname "$0")/.."
export APP_SECRET="${APP_SECRET:-e2e-secret}"
# Disabilita il rate-limit sugli endpoint di auth: l'intera suite E2E (~74 test)
# effettua il login da localhost (stesso IP) con --workers=1, superando la soglia
# di produzione (10/5min) e ricevendo 429. 0 = nessun limite.
export APP_AUTH_RATELIMIT="${APP_AUTH_RATELIMIT:-0}"
export DB_DRIVER=sqlite
export DB_DSN="${DB_DSN:-/tmp/openjira-e2e.db}"
rm -f "$DB_DSN"
go run ./cmd/seed
exec go run ./cmd/server
