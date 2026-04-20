#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"
export COMPOSE_FILE="docker-compose.yml"

source scripts/ci/common.sh

ARTIFACTS_DIR="$(ci_artifact_dir restore)"

cleanup() {
  collect_compose_artifacts restore
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
export COMPOSE_PROFILES=db
set +a

echo "== generate local TLS certs for restore check =="
bash scripts/certs/generate.sh

echo "== start db/cache/mongo =="
docker compose up -d db cache mongo

WAIT_ATTEMPTS=60 WAIT_SLEEP_SECONDS=2 wait_for_compose

SUFFIX="$(python3 - <<'PY'
import uuid
print(uuid.uuid4().hex[:10])
PY
)"

PG_EMAIL="restore_check_${SUFFIX}@local.dev"
MONGO_NAME="restore-check-${SUFFIX}"
BACKUP_DIR="$ARTIFACTS_DIR/backup-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$BACKUP_DIR"

echo "== seed postgres marker =="
docker compose exec -T db sh -lc "
  export PGPASSWORD='$POSTGRES_PASSWORD'
  psql -U admin -d secure-voting -v ON_ERROR_STOP=1 -c \"
    INSERT INTO users (email, password_hash, role)
    VALUES ('$PG_EMAIL', 'restore-check', 'voter');
  \"
"

echo "== seed mongo marker =="
docker compose exec -T mongo sh -lc "
  mongosh \
    --host localhost \
    --port 27017 \
    --tls \
    --tlsCAFile /tmp/mongo-certs/ca.pem \
    --tlsAllowInvalidHostnames \
    --username root \
    --password '$MONGO_INITDB_ROOT_PASSWORD' \
    --authenticationDatabase admin \
    --quiet \
    --eval '
      db = db.getSiblingDB(\"secure_voting\");
      db.datasets.insertOne({
        name: \"$MONGO_NAME\",
        description: \"restore check marker\",
        source: \"generate\",
        format: \"ranking\",
        candidates: [{id:\"c1\",name:\"Alice\"}],
        parameters: {},
        created_at: new Date()
      });
    '
"

echo "== create backups =="
bash scripts/ops/backup_postgres.sh "$BACKUP_DIR"
bash scripts/ops/backup_mongo.sh "$BACKUP_DIR"

echo "== mutate data after backup =="
docker compose exec -T db sh -lc "
  export PGPASSWORD='$POSTGRES_PASSWORD'
  psql -U admin -d secure-voting -v ON_ERROR_STOP=1 -c \"
    DELETE FROM users WHERE email = '$PG_EMAIL';
  \"
"

docker compose exec -T mongo sh -lc "
  mongosh \
    --host localhost \
    --port 27017 \
    --tls \
    --tlsCAFile /tmp/mongo-certs/ca.pem \
    --tlsAllowInvalidHostnames \
    --username root \
    --password '$MONGO_INITDB_ROOT_PASSWORD' \
    --authenticationDatabase admin \
    --quiet \
    --eval '
      db = db.getSiblingDB(\"secure_voting\");
      db.datasets.deleteOne({name: \"$MONGO_NAME\"});
    '
"

echo "== restore backups =="
RESTORE_STARTED_AT="$(date +%s)"

bash scripts/ops/restore_postgres.sh "$BACKUP_DIR/postgres_secure_voting.dump"
bash scripts/ops/restore_mongo.sh "$BACKUP_DIR/mongo_secure_voting.archive.gz"

RESTORE_FINISHED_AT="$(date +%s)"
RESTORE_SECONDS="$((RESTORE_FINISHED_AT - RESTORE_STARTED_AT))"

echo "== verify postgres marker restored =="
PG_COUNT="$(
docker compose exec -T db sh -lc "
  export PGPASSWORD='$POSTGRES_PASSWORD'
  psql -U admin -d secure-voting -t -A -c \"
    SELECT COUNT(*) FROM users WHERE email = '$PG_EMAIL';
  \"
" | tr -d '[:space:]'
)"

if [[ "$PG_COUNT" != "1" ]]; then
  echo "postgres restore check failed: expected 1 restored row, got $PG_COUNT"
  exit 1
fi

echo "== verify mongo marker restored =="
MONGO_COUNT="$(
docker compose exec -T mongo sh -lc "
  mongosh \
    --host localhost \
    --port 27017 \
    --tls \
    --tlsCAFile /tmp/mongo-certs/ca.pem \
    --tlsAllowInvalidHostnames \
    --username root \
    --password '$MONGO_INITDB_ROOT_PASSWORD' \
    --authenticationDatabase admin \
    --quiet \
    --eval '
      db = db.getSiblingDB(\"secure_voting\");
      print(db.datasets.countDocuments({name: \"$MONGO_NAME\"}));
    '
" | tr -d '[:space:]'
)"

if [[ "$MONGO_COUNT" != "1" ]]; then
  echo "mongo restore check failed: expected 1 restored document, got $MONGO_COUNT"
  exit 1
fi

echo "== restore duration =="
echo "restore_seconds=$RESTORE_SECONDS" | tee "$ARTIFACTS_DIR/restore-duration.txt"

if (( RESTORE_SECONDS > 600 )); then
  echo "restore check failed: restore took more than 600 seconds"
  exit 1
fi

echo
echo "RESTORE CHECK: PASS"