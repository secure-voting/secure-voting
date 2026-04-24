#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

unset COMPOSE_FILE
unset COMPOSE_PROFILES
unset COMPOSE_PROJECT_NAME

source scripts/ci/common.sh

ARTIFACTS_DIR="$(ci_artifact_dir mtbf-light)"
COMPOSE_ARGS=(-f docker-compose.yml)

cleanup() {
  collect_compose_artifacts mtbf-light "${COMPOSE_ARGS[@]}"
  docker compose "${COMPOSE_ARGS[@]}" down -v --remove-orphans || true
}
trap cleanup EXIT

if [[ ! -f .env && -f .env.example ]]; then
  cp .env.example .env
fi

if [[ ! -f .env ]]; then
  echo "missing .env"
  exit 1
fi

set -a
source .env
export COMPOSE_PROFILES=prod
set +a

DURATION_SECONDS="${DURATION_SECONDS:-7200}"
SAMPLE_INTERVAL_SECONDS="${SAMPLE_INTERVAL_SECONDS:-30}"

echo "== generate local TLS certs =="
bash scripts/certs/generate.sh

echo "== start prod stack =="
docker compose "${COMPOSE_ARGS[@]}" up -d

WAIT_ATTEMPTS=120 WAIT_SLEEP_SECONDS=2 wait_for_compose "${COMPOSE_ARGS[@]}"

STARTED_AT="$(date +%s)"
DEADLINE=$((STARTED_AT + DURATION_SECONDS))
SAMPLES=0
FAILURES=0

echo "started_at_epoch=$STARTED_AT" > "$ARTIFACTS_DIR/mtbf-light-summary.txt"
echo "duration_seconds=$DURATION_SECONDS" >> "$ARTIFACTS_DIR/mtbf-light-summary.txt"
echo "sample_interval_seconds=$SAMPLE_INTERVAL_SECONDS" >> "$ARTIFACTS_DIR/mtbf-light-summary.txt"

while [[ $(date +%s) -lt $DEADLINE ]]; do
  NOW="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  BACKEND_OK=0
  FRONTEND_OK=0

  if curl -fsS http://127.0.0.1:3001/health >/dev/null 2>&1; then
    BACKEND_OK=1
  fi

  if wget --no-check-certificate -q -O - https://127.0.0.1:8080/ >/dev/null 2>&1; then
    FRONTEND_OK=1
  fi

  STATUS_JSON="$(curl -fsS http://127.0.0.1:3001/api/v1/system/status 2>/dev/null || true)"
  if [[ -z "$STATUS_JSON" ]]; then
    STATUS_JSON="unavailable"
  fi

  echo "$NOW backend_ok=$BACKEND_OK frontend_ok=$FRONTEND_OK status=$STATUS_JSON" >> "$ARTIFACTS_DIR/mtbf-light.log"

  if [[ "$BACKEND_OK" != "1" || "$FRONTEND_OK" != "1" ]]; then
    FAILURES=$((FAILURES + 1))
  fi

  SAMPLES=$((SAMPLES + 1))
  sleep "$SAMPLE_INTERVAL_SECONDS"
done

echo "samples=$SAMPLES" >> "$ARTIFACTS_DIR/mtbf-light-summary.txt"
echo "failures=$FAILURES" >> "$ARTIFACTS_DIR/mtbf-light-summary.txt"

if (( FAILURES > 0 )); then
  echo "MTBF LIGHT CHECK: FAIL"
  exit 1
fi

echo "MTBF LIGHT CHECK: PASS"