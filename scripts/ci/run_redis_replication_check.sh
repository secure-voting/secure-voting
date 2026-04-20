#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

source scripts/ci/common.sh

ARTIFACTS_DIR="$(ci_artifact_dir redis-replication)"
export COMPOSE_FILE="docker-compose.redis-replication.yml"

cleanup() {
  collect_compose_artifacts redis-replication
  docker compose down -v --remove-orphans || true
}
trap cleanup EXIT

echo "== start redis primary/replica =="
docker compose up -d

WAIT_ATTEMPTS=60 WAIT_SLEEP_SECONDS=2 wait_for_compose

SUFFIX="$(python3 - <<'PY'
import uuid
print(uuid.uuid4().hex[:10])
PY
)"
MARKER_KEY="redis_replication_${SUFFIX}"
MARKER_VALUE="value_${SUFFIX}"

echo "== verify primary accepts writes =="
docker compose exec -T redis-primary redis-cli -p 6379 SET "$MARKER_KEY" "$MARKER_VALUE" >/dev/null

echo "== wait for replica to receive marker =="
deadline=$(( $(date +%s) + 60 ))
REPLICA_VALUE=""

while [[ $(date +%s) -lt $deadline ]]; do
  REPLICA_VALUE="$(
    docker compose exec -T redis-replica redis-cli -p 6379 --raw GET "$MARKER_KEY" | tr -d '\r'
  )"

  if [[ "$REPLICA_VALUE" == "$MARKER_VALUE" ]]; then
    break
  fi

  sleep 2
done

if [[ "$REPLICA_VALUE" != "$MARKER_VALUE" ]]; then
  echo "replica did not receive marker key"
  exit 1
fi

echo "== verify replica is read-only =="
set +e
READONLY_OUT="$(
  docker compose exec -T redis-replica redis-cli -p 6379 SET "readonly_${SUFFIX}" "x" 2>&1 | tr -d '\r'
)"
READONLY_RC=$?
set -e

printf '%s\n' "$READONLY_OUT" > "$ARTIFACTS_DIR/replica-readonly.txt"

if [[ "$READONLY_OUT" != *"READONLY"* ]]; then
  echo "replica unexpectedly accepted write"
  echo "$READONLY_OUT"
  exit 1
fi

echo "== verify replication role/status =="

PRIMARY_ROLE="$(
  docker compose exec -T redis-primary redis-cli -p 6379 --raw ROLE | head -n 1 | tr -d '\r'
)"
REPLICA_ROLE="$(
  docker compose exec -T redis-replica redis-cli -p 6379 --raw ROLE | head -n 1 | tr -d '\r'
)"

if [[ "$PRIMARY_ROLE" != "master" ]]; then
  echo "primary is not master: $PRIMARY_ROLE"
  exit 1
fi

if [[ "$REPLICA_ROLE" != "slave" ]]; then
  echo "replica is not slave: $REPLICA_ROLE"
  exit 1
fi

PRIMARY_INFO="$ARTIFACTS_DIR/redis-primary-info.txt"
REPLICA_INFO="$ARTIFACTS_DIR/redis-replica-info.txt"

docker compose exec -T redis-primary redis-cli -p 6379 INFO replication | tr -d '\r' > "$PRIMARY_INFO"
docker compose exec -T redis-replica redis-cli -p 6379 INFO replication | tr -d '\r' > "$REPLICA_INFO"

if ! grep -q '^connected_slaves:1$' "$PRIMARY_INFO"; then
  echo "primary does not report one connected replica"
  cat "$PRIMARY_INFO"
  exit 1
fi

if ! grep -q '^master_link_status:up$' "$REPLICA_INFO"; then
  echo "replica is not connected to master"
  cat "$REPLICA_INFO"
  exit 1
fi

echo
echo "REDIS REPLICATION CHECK: PASS"