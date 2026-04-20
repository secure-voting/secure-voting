#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"
export COMPOSE_FILE="docker-compose.yml"

source scripts/ci/common.sh

ARTIFACTS_DIR="$(ci_artifact_dir e2e)"

cleanup() {
  collect_compose_artifacts e2e
  docker compose down -v --remove-orphans || true
}
trap cleanup EXIT

if [[ ! -f .env && -f .env.example ]]; then
  cp .env.example .env
fi

if [[ ! -f .env ]]; then
  cat > .env <<'ENVEOF'
POSTGRES_PASSWORD=postgres_dev_pass
REDIS_PASSWORD=redis_dev_pass
MONGO_INITDB_ROOT_PASSWORD=mongo_dev_pass
BOOTSTRAP_ADMIN_EMAIL=admin@example.com
BOOTSTRAP_ADMIN_PASSWORD=AdminPass123!
BOOTSTRAP_RESEARCHER_EMAIL=researcher@example.com
BOOTSTRAP_RESEARCHER_PASSWORD=ResearchPass123!
ENVEOF
fi

if ! grep -q '^BOOTSTRAP_ADMIN_EMAIL=' .env; then
  printf '\nBOOTSTRAP_ADMIN_EMAIL=admin@example.com\n' >> .env
fi
if ! grep -q '^BOOTSTRAP_ADMIN_PASSWORD=' .env; then
  printf 'BOOTSTRAP_ADMIN_PASSWORD=AdminPass123!\n' >> .env
fi
if ! grep -q '^BOOTSTRAP_RESEARCHER_EMAIL=' .env; then
  printf 'BOOTSTRAP_RESEARCHER_EMAIL=researcher@example.com\n' >> .env
fi
if ! grep -q '^BOOTSTRAP_RESEARCHER_PASSWORD=' .env; then
  printf 'BOOTSTRAP_RESEARCHER_PASSWORD=ResearchPass123!\n' >> .env
fi

set -a
source .env
export COMPOSE_PROFILES=prod
export COMPOSE_DOCKER_CLI_BUILD=1
export DOCKER_BUILDKIT=1
set +a

export TLS_CA_CERT="$ROOT_DIR/scripts/certs/out/ca.pem"

echo "== generate local TLS certs =="
bash scripts/certs/generate.sh

wait_http() {
  local url="$1"
  local attempts="${2:-90}"
  local sleep_seconds="${3:-5}"

  for ((i=1; i<=attempts; i++)); do
    if curl --cacert "$TLS_CA_CERT" -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$sleep_seconds"
  done

  return 1
}

wait_kafka_topic() {
  local topic="$1"
  local attempts="${2:-60}"
  local sleep_seconds="${3:-2}"

  for ((i=1; i<=attempts; i++)); do
        if docker compose exec -T kafka kafka-topics --bootstrap-server kafka:29092 --list 2>/dev/null | grep -qx "$topic"; then
      return 0
    fi
    sleep "$sleep_seconds"
  done

  return 1
}

wait_running() {
  local service="$1"
  local attempts="${2:-60}"
  local sleep_seconds="${3:-3}"

  for ((i=1; i<=attempts; i++)); do
    local cid state
    cid="$(docker compose ps -q "$service" 2>/dev/null || true)"
    if [[ -n "$cid" ]]; then
      state="$(docker inspect -f '{{.State.Status}}' "$cid" 2>/dev/null || true)"
      if [[ "$state" == "running" ]]; then
        return 0
      fi
    fi
    sleep "$sleep_seconds"
  done

  return 1
}

echo "== start base prod stack =="
docker compose up -d --build db cache mongo kafka compute backend frontend

echo "== wait for compose services =="
WAIT_ATTEMPTS=90 WAIT_SLEEP_SECONDS=5 wait_for_compose

echo "== wait for backend/frontend http/https =="
wait_http "http://127.0.0.1:3001/health" 90 5
wait_http "https://127.0.0.1:8080/" 90 5

echo "== init kafka topics =="
docker compose exec -T kafka kafka-topics --bootstrap-server kafka:29092 --create --if-not-exists --topic secure-voting.compute.tasks --partitions 1 --replication-factor 1
docker compose exec -T kafka kafka-topics --bootstrap-server kafka:29092 --create --if-not-exists --topic secure-voting.compute.results --partitions 1 --replication-factor 1

echo "== verify kafka topics =="
docker compose exec -T kafka kafka-topics --bootstrap-server kafka:29092 --list | grep -qx 'secure-voting.compute.tasks'
docker compose exec -T kafka kafka-topics --bootstrap-server kafka:29092 --list | grep -qx 'secure-voting.compute.results'

echo "== wait for kafka topics =="
wait_kafka_topic secure-voting.compute.tasks 60 2
wait_kafka_topic secure-voting.compute.results 60 2

echo "== start worker services =="
docker compose up -d --build worker compute-runner

wait_running compute 60 3
wait_running worker 60 3
wait_running compute-runner 60 3

bash scripts/e2e_smoke.sh
bash scripts/e2e_election_lifecycle.sh
bash scripts/e2e_invite_only.sh
bash scripts/e2e_smoke_experiment.sh

if [[ "${RUN_OPTIONAL_VOTE_FORMATS_E2E:-0}" == "1" ]]; then
  bash scripts/e2e_vote_formats.sh
else
  echo "SKIP: e2e_vote_formats.sh is optional until approval/score election tally is implemented"
fi