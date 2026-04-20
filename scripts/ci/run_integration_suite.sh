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

export TLS_CA_CERT="$ROOT_DIR/scripts/certs/out/ca.pem"

echo "== generate local TLS certs for integration =="
bash scripts/certs/generate.sh

docker compose "${COMPOSE_ARGS[@]}" config > "${ARTIFACTS_DIR}/compose.config.yaml"

docker compose "${COMPOSE_ARGS[@]}" up -d db cache mongo

WAIT_ATTEMPTS=60 WAIT_SLEEP_SECONDS=2 wait_for_compose "${COMPOSE_ARGS[@]}"

export POSTGRES_DSN="postgres://admin:${POSTGRES_PASSWORD}@127.0.0.1:15432/secure-voting?sslmode=verify-full&sslrootcert=${TLS_CA_CERT}"
export REDIS_ADDR="127.0.0.1:16379"
export REDIS_PASSWORD="${REDIS_PASSWORD}"
export REDIS_TLS="true"
export REDIS_TLS_CA="${TLS_CA_CERT}"
export MONGO_URI="mongodb://root:${MONGO_INITDB_ROOT_PASSWORD}@127.0.0.1:17017/?authSource=admin&tls=true&tlsCAFile=${TLS_CA_CERT}"
export MONGO_DB="secure_voting"
export TOKEN_TTL="720h"
export IDEMPOTENCY_TTL="24h"
export MAX_UPLOAD_BYTES="10485760"

export DISABLE_COMPUTE="1"
export AUTH_RATE_LIMIT_TTL="1s"
export WRITE_RATE_LIMIT_TTL="1s"

export COMPUTE_GRPC_ADDR=""
export COMPUTE_TLS="false"
export COMPUTE_TLS_CA=""
export COMPUTE_TLS_SERVER_NAME=""

(
  cd apps/backend
  SECURE_VOTING_INTEGRATION=1 go test ./internal/httpserver -v
)