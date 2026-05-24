#!/usr/bin/env bash
# Smoke + load test: publish events, then hammer /top with `hey`.
# Requires: docker compose up (already running) and `hey` installed locally.

set -euo pipefail

URL="${URL:-http://localhost:8080/top?limit=10}"
DURATION="${DURATION:-30s}"
CONCURRENCY="${CONCURRENCY:-100}"
EVENTS="${EVENTS:-10000}"
UNIQUE="${UNIQUE:-50}"

if ! command -v hey >/dev/null 2>&1; then
  echo "hey not installed. Install via 'brew install hey' or 'go install github.com/rakyll/hey@latest'." >&2
  exit 1
fi

echo "[1/3] Health check..."
curl -fsS http://localhost:8080/health
echo

echo "[2/3] Producing $EVENTS events..."
"$(dirname "$0")/produce.sh" "$EVENTS" "$UNIQUE"

sleep 2  # give consumer time to drain

echo "[3/3] Hammering $URL for $DURATION with $CONCURRENCY workers..."
hey -z "$DURATION" -c "$CONCURRENCY" "$URL"

echo
echo "Current top:"
curl -fsS "$URL"
echo

echo
echo "Service metrics:"
curl -fsS http://localhost:8080/metrics | grep -E '^search_trends_'
