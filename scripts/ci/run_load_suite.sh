#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

export COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"
unset COMPOSE_PROJECT_NAME
unset COMPOSE_PROFILES

source scripts/ci/common.sh

ARTIFACTS_DIR="$(ci_artifact_dir load)"

collect_load_artifacts() {
  mkdir -p "$ARTIFACTS_DIR"

  if [[ -d tests/load/load-results ]]; then
    cp -R tests/load/load-results "$ARTIFACTS_DIR/load-results" || true
  fi
}

cleanup() {
  cd "$ROOT_DIR"

  collect_load_artifacts
  collect_compose_artifacts load
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
BOOTSTRAP_RESEARCHER_PASSWORD=ResearcherPass123!
EMAIL_VERIFICATION_MODE=dev
ENVEOF
fi

if ! grep -q '^POSTGRES_PASSWORD=' .env; then
  printf '\nPOSTGRES_PASSWORD=postgres_dev_pass\n' >> .env
fi
if ! grep -q '^REDIS_PASSWORD=' .env; then
  printf 'REDIS_PASSWORD=redis_dev_pass\n' >> .env
fi
if ! grep -q '^MONGO_INITDB_ROOT_PASSWORD=' .env; then
  printf 'MONGO_INITDB_ROOT_PASSWORD=mongo_dev_pass\n' >> .env
fi
if ! grep -q '^BOOTSTRAP_ADMIN_EMAIL=' .env; then
  printf 'BOOTSTRAP_ADMIN_EMAIL=admin@example.com\n' >> .env
fi
if ! grep -q '^BOOTSTRAP_ADMIN_PASSWORD=' .env; then
  printf 'BOOTSTRAP_ADMIN_PASSWORD=AdminPass123!\n' >> .env
fi
if ! grep -q '^BOOTSTRAP_RESEARCHER_EMAIL=' .env; then
  printf 'BOOTSTRAP_RESEARCHER_EMAIL=researcher@example.com\n' >> .env
fi
if ! grep -q '^BOOTSTRAP_RESEARCHER_PASSWORD=' .env; then
  printf 'BOOTSTRAP_RESEARCHER_PASSWORD=ResearcherPass123!\n' >> .env
fi
if ! grep -q '^EMAIL_VERIFICATION_MODE=' .env; then
  printf 'EMAIL_VERIFICATION_MODE=dev\n' >> .env
fi

set -a
source .env
export COMPOSE_PROFILES=prod
export COMPOSE_DOCKER_CLI_BUILD=1
export DOCKER_BUILDKIT=1
export TLS_CA_CERT="$ROOT_DIR/scripts/certs/out/ca.pem"
export FRONTEND_BASE="https://127.0.0.1:8080"
export BACKEND_BASE="http://127.0.0.1:3001"
export API_BASE="https://127.0.0.1:8080/api/v1"

export AUTH_RATE_LIMIT="${CI_AUTH_RATE_LIMIT:-1000}"
export AUTH_RATE_LIMIT_TTL="${CI_AUTH_RATE_LIMIT_TTL:-1s}"
export WRITE_RATE_LIMIT="${CI_WRITE_RATE_LIMIT:-1000}"
export WRITE_RATE_LIMIT_TTL="${CI_WRITE_RATE_LIMIT_TTL:-1s}"
set +a

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

  echo "HTTP endpoint is not ready: $url" >&2
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

  echo "Compose service is not running: $service" >&2
  return 1
}

kafka_topics_tls() {
  docker compose exec -T kafka sh -lc '
cat > /tmp/kafka-client-ssl.properties <<EOF
security.protocol=SSL
ssl.truststore.location=/etc/kafka/secrets/kafka.truststore.p12
ssl.truststore.password=changeit
ssl.truststore.type=PKCS12
ssl.endpoint.identification.algorithm=
EOF

kafka-topics \
  --bootstrap-server kafka:29092 \
  --command-config /tmp/kafka-client-ssl.properties \
  "$@"
' sh "$@"
}

echo "== clean previous runtime stack =="
docker compose down -v --remove-orphans || true

echo "== generate local TLS certs =="
bash scripts/certs/generate.sh

echo "== start base prod stack =="
docker compose up -d --build db cache mongo kafka compute backend frontend

echo "== wait for base compose services =="
WAIT_ATTEMPTS=90 WAIT_SLEEP_SECONDS=5 wait_for_compose

echo "== wait for backend/frontend =="
wait_http "https://127.0.0.1:8080/" 90 5
curl -fsS "http://127.0.0.1:3001/health" >/dev/null

echo "== init kafka topics over TLS =="
kafka_topics_tls --create --if-not-exists --topic secure-voting.compute.tasks --partitions 1 --replication-factor 1
kafka_topics_tls --create --if-not-exists --topic secure-voting.compute.results --partitions 1 --replication-factor 1

echo "== verify kafka topics over TLS =="
kafka_topics_tls --list | grep -qx 'secure-voting.compute.tasks'
kafka_topics_tls --list | grep -qx 'secure-voting.compute.results'

echo "== start worker services =="
docker compose up -d --build worker compute-runner

wait_running compute 60 3
wait_running worker 60 3
wait_running compute-runner 60 3

echo "== install load test dependencies =="
cd "$ROOT_DIR/tests/load"
npm ci

echo "== typecheck load tests =="
npm run typecheck

echo "== run ballot load smoke tests =="
BALLOT_FORMAT=ranking VOTERS="${LOAD_BALLOT_VOTERS:-10}" CONCURRENCY="${LOAD_BALLOT_CONCURRENCY:-5}" npm run ballot
BALLOT_FORMAT=approval VOTERS="${LOAD_BALLOT_VOTERS:-10}" CONCURRENCY="${LOAD_BALLOT_CONCURRENCY:-5}" npm run ballot
BALLOT_FORMAT=score VOTERS="${LOAD_BALLOT_VOTERS:-10}" CONCURRENCY="${LOAD_BALLOT_CONCURRENCY:-5}" npm run ballot

echo "== run dataset generate load smoke test =="
DATASET_FORMAT=all DATASETS="${LOAD_DATASETS:-6}" DATASET_VOTERS="${LOAD_DATASET_VOTERS:-20}" CONCURRENCY="${LOAD_DATASET_CONCURRENCY:-3}" npm run datasets

echo "== run jobs polling load smoke test =="
REQUESTS="${LOAD_JOB_REQUESTS:-30}" CONCURRENCY="${LOAD_JOB_CONCURRENCY:-10}" npm run jobs

echo "== run experiment-run load smoke test =="
EXPERIMENT_FORMATS=ranking,approval,score RUNS_PER_FORMAT="${LOAD_RUNS_PER_FORMAT:-1}" DATASET_VOTERS="${LOAD_EXPERIMENT_DATASET_VOTERS:-20}" CONCURRENCY="${LOAD_EXPERIMENT_CONCURRENCY:-1}" npm run experiments

cd "$ROOT_DIR"