#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

source scripts/ci/common.sh

ARTIFACTS_DIR="$(ci_artifact_dir pg-replication)"
export COMPOSE_FILE="docker-compose.postgres-replication.yml"

cleanup() {
  collect_compose_artifacts pg-replication
  docker compose down -v --remove-orphans || true
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
: "${POSTGRES_PASSWORD:?missing POSTGRES_PASSWORD}"
export POSTGRES_REPLICATION_PASSWORD="${POSTGRES_REPLICATION_PASSWORD:-replicatorpass}"
set +a

echo "== start postgres primary/replica =="
docker compose up -d

WAIT_ATTEMPTS=60 WAIT_SLEEP_SECONDS=2 wait_for_compose

echo "== verify primary accepts writes =="
SUFFIX="$(python3 - <<'PY'
import uuid
print(uuid.uuid4().hex[:10])
PY
)"
MARKER_EMAIL="pg_replication_${SUFFIX}@local.dev"

docker compose exec -T pg-primary sh -lc "
  export PGPASSWORD='$POSTGRES_PASSWORD'
  psql -h localhost -U admin -d secure-voting -v ON_ERROR_STOP=1 -c \"
    INSERT INTO users (email, password_hash, role)
    VALUES ('$MARKER_EMAIL', 'replication-check', 'voter');
  \"
"

echo "== wait for replica to receive marker =="
deadline=$(( $(date +%s) + 60 ))
REPLICA_COUNT="0"

while [[ $(date +%s) -lt $deadline ]]; do
  REPLICA_COUNT="$(
    docker compose exec -T pg-replica sh -lc "
      export PGPASSWORD='$POSTGRES_PASSWORD'
      psql -h localhost -U admin -d secure-voting -t -A -c \"
        SELECT COUNT(*) FROM users WHERE email = '$MARKER_EMAIL';
      \"
    " | tr -d '[:space:]'
  )"

  if [[ "$REPLICA_COUNT" == "1" ]]; then
    break
  fi

  sleep 2
done

if [[ "$REPLICA_COUNT" != "1" ]]; then
  echo "replica did not receive marker row"
  exit 1
fi

echo "== verify replica is read-only =="
set +e
READONLY_OUT="$(
  docker compose exec -T pg-replica sh -lc "
    export PGPASSWORD='$POSTGRES_PASSWORD'
    psql -h localhost -U admin -d secure-voting -v ON_ERROR_STOP=1 -c \"
      INSERT INTO users (email, password_hash, role)
      VALUES ('readonly_check_${SUFFIX}@local.dev', 'x', 'voter');
    \"
  " 2>&1
)"
READONLY_RC=$?
set -e

if [[ $READONLY_RC -eq 0 ]]; then
  echo "replica unexpectedly accepted write"
  echo "$READONLY_OUT"
  exit 1
fi

echo "$READONLY_OUT" > "$ARTIFACTS_DIR/replica-readonly.txt"

echo "== verify streaming replication status on primary =="
docker compose exec -T pg-primary sh -lc "
  export PGPASSWORD='$POSTGRES_PASSWORD'
  psql -h localhost -U admin -d secure-voting -t -A -c \"
    SELECT application_name || ':' || state
    FROM pg_stat_replication;
  \"
" | tee "$ARTIFACTS_DIR/pg_stat_replication.txt"

if ! grep -q "streaming" "$ARTIFACTS_DIR/pg_stat_replication.txt"; then
  echo "primary does not report streaming replica"
  exit 1
fi

echo
echo "POSTGRES REPLICATION CHECK: PASS"