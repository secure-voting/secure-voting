#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

source scripts/ci/common.sh

ARTIFACTS_DIR="$(ci_artifact_dir mongo-replication)"
export COMPOSE_FILE="docker-compose.mongo-replication.yml"

cleanup() {
  collect_compose_artifacts mongo-replication
  docker compose down -v --remove-orphans || true
}
trap cleanup EXIT

echo "== start mongo primary/secondary =="
docker compose up -d

WAIT_ATTEMPTS=60 WAIT_SLEEP_SECONDS=2 wait_for_compose

echo "== wait replica set init =="
deadline=$(( $(date +%s) + 120 ))
PRIMARY_READY="false"
SECONDARY_READY="false"

while [[ $(date +%s) -lt $deadline ]]; do
  PRIMARY_READY="$(
    docker compose exec -T mongo-primary mongosh --quiet --host localhost --port 27017 --eval '
      try {
        const h = db.hello();
        print(h.isWritablePrimary === true ? "true" : "false");
      } catch (e) {
        print("false");
      }
    ' | tr -d '\r[:space:]'
  )"

  SECONDARY_READY="$(
    docker compose exec -T mongo-secondary mongosh --quiet --host localhost --port 27017 --eval '
      try {
        const h = db.hello();
        print(h.secondary === true ? "true" : "false");
      } catch (e) {
        print("false");
      }
    ' | tr -d '\r[:space:]'
  )"

  if [[ "$PRIMARY_READY" == "true" && "$SECONDARY_READY" == "true" ]]; then
    break
  fi

  sleep 2
done

if [[ "$PRIMARY_READY" != "true" ]]; then
  echo "mongo-primary did not become writable primary"
  exit 1
fi

if [[ "$SECONDARY_READY" != "true" ]]; then
  echo "mongo-secondary did not become secondary"
  exit 1
fi

SUFFIX="$(python3 - <<'PY'
import uuid
print(uuid.uuid4().hex[:10])
PY
)"
MARKER_NAME="mongo_replication_${SUFFIX}"

echo "== verify primary accepts writes =="
docker compose exec -T mongo-primary mongosh --quiet --host localhost --port 27017 --eval "
db = db.getSiblingDB('replication_check');
const res = db.items.insertOne(
  {name: '$MARKER_NAME', created_at: new Date()},
  {writeConcern: {w: 2, wtimeout: 60000}}
);
printjson(res);
"

echo "== wait for secondary to receive marker =="
deadline=$(( $(date +%s) + 120 ))
SECONDARY_COUNT="0"

while [[ $(date +%s) -lt $deadline ]]; do
  SECONDARY_COUNT="$(
    docker compose exec -T mongo-secondary mongosh --quiet --host localhost --port 27017 --eval "
      db.getMongo().setReadPref('secondaryPreferred');
      db = db.getSiblingDB('replication_check');
      print(db.items.countDocuments({name: '$MARKER_NAME'}));
    " | tr -d '\r[:space:]'
  )"

  if [[ "$SECONDARY_COUNT" == "1" ]]; then
    break
  fi

  sleep 2
done

if [[ "$SECONDARY_COUNT" != "1" ]]; then
  docker compose exec -T mongo-primary mongosh --quiet --host localhost --port 27017 --eval 'EJSON.stringify(rs.status())' > "$ARTIFACTS_DIR/mongo-rs-status-primary.json" || true
  docker compose exec -T mongo-secondary mongosh --quiet --host localhost --port 27017 --eval 'EJSON.stringify(rs.status())' > "$ARTIFACTS_DIR/mongo-rs-status-secondary.json" || true
  echo "secondary did not receive marker document"
  exit 1
fi

echo "== verify roles =="
PRIMARY_HELLO="$ARTIFACTS_DIR/mongo-primary-hello.json"
SECONDARY_HELLO="$ARTIFACTS_DIR/mongo-secondary-hello.json"

docker compose exec -T mongo-primary mongosh --quiet --host localhost --port 27017 --eval 'EJSON.stringify(db.hello())' > "$PRIMARY_HELLO"
docker compose exec -T mongo-secondary mongosh --quiet --host localhost --port 27017 --eval 'EJSON.stringify(db.hello())' > "$SECONDARY_HELLO"

if ! grep -q '"isWritablePrimary"[[:space:]]*:[[:space:]]*true' "$PRIMARY_HELLO"; then
  echo "mongo-primary is not writable primary"
  cat "$PRIMARY_HELLO"
  exit 1
fi

if ! grep -q '"secondary"[[:space:]]*:[[:space:]]*true' "$SECONDARY_HELLO"; then
  echo "mongo-secondary is not secondary"
  cat "$SECONDARY_HELLO"
  exit 1
fi

echo
echo "MONGO REPLICATION CHECK: PASS"