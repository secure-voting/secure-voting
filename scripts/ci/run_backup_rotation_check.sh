#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

unset COMPOSE_FILE
unset COMPOSE_PROFILES
unset COMPOSE_PROJECT_NAME

source scripts/ci/common.sh

ARTIFACTS_DIR="$(ci_artifact_dir backup-rotation)"
export COMPOSE_FILE="docker-compose.yml"

cleanup() {
  collect_compose_artifacts backup-rotation
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

echo "== generate local TLS certs =="
bash scripts/certs/generate.sh

echo "== start db/cache/mongo =="
docker compose up -d db cache mongo

WAIT_ATTEMPTS=60 WAIT_SLEEP_SECONDS=2 wait_for_compose

BACKUP_ROOT="$ARTIFACTS_DIR/backups"
mkdir -p "$BACKUP_ROOT"

echo "== create synthetic backup generations =="
for stamp in \
  20260101T000000Z \
  20260102T000000Z \
  20260103T000000Z \
  20260104T000000Z \
  20260105T000000Z \
  20260106T000000Z \
  20260107T000000Z \
  20260108T000000Z \
  20260115T000000Z \
  20260201T000000Z \
  20260301T000000Z \
  20260401T000000Z \
  20260501T000000Z \
  20260601T000000Z \
  20260701T000000Z; do
  mkdir -p "$BACKUP_ROOT/$stamp"
  printf 'stub\n' > "$BACKUP_ROOT/$stamp/postgres_secure_voting.dump"
done

echo "== create real backup bundle =="
bash scripts/ops/backup_all.sh "$BACKUP_ROOT"

echo "== prune old backups =="
KEEP_DAILY=7 KEEP_WEEKLY=4 KEEP_MONTHLY=6 bash scripts/ops/prune_old_backups.sh "$BACKUP_ROOT"

echo "== verify at least one real backup remains =="
if ! find "$BACKUP_ROOT" -type f -name 'postgres_secure_voting.dump' | grep -q .; then
  echo "no postgres backups remain after prune"
  exit 1
fi

if ! find "$BACKUP_ROOT" -type f -name 'mongo_secure_voting.archive.gz' | grep -q .; then
  echo "no mongo backups remain after prune"
  exit 1
fi

find "$BACKUP_ROOT" -maxdepth 2 -type f | sort > "$ARTIFACTS_DIR/remaining-backups.txt"

echo
echo "BACKUP ROTATION CHECK: PASS"