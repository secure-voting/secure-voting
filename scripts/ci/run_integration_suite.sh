#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

source scripts/ci/common.sh

ARTIFACTS_DIR="$(ci_artifact_dir integration)"
COMPOSE_ARGS=(-f docker-compose.yml -f docker-compose.integration.yml)

cleanup() {
  collect_compose_artifacts integration "${COMPOSE_ARGS[@]}"
  docker compose "${COMPOSE_ARGS[@]}" down -v --remove-orphans || true
}
trap cleanup EXIT

if [[ ! -f .env.integration ]]; then
  echo "ERROR: .env.integration not found"
  exit 1
fi

set -a
source .env.integration
export COMPOSE_PROFILES=db
set +a

docker compose "${COMPOSE_ARGS[@]}" config > "${ARTIFACTS_DIR}/compose.config.yaml"

docker compose "${COMPOSE_ARGS[@]}" up -d db cache mongo

WAIT_ATTEMPTS=60 WAIT_SLEEP_SECONDS=2 wait_for_compose "${COMPOSE_ARGS[@]}"

export POSTGRES_DSN="postgres://admin:${POSTGRES_PASSWORD}@127.0.0.1:15432/secure-voting?sslmode=disable"
export REDIS_ADDR="127.0.0.1:16379"
export REDIS_PASSWORD="${REDIS_PASSWORD}"
export MONGO_URI="mongodb://root:${MONGO_INITDB_ROOT_PASSWORD}@127.0.0.1:17017/?authSource=admin"
export MONGO_DB="secure_voting"
export TOKEN_TTL="720h"
export IDEMPOTENCY_TTL="24h"
export MAX_UPLOAD_BYTES="10485760"

(
  cd apps/backend
  SECURE_VOTING_INTEGRATION=1 go test ./internal/httpserver -v
)