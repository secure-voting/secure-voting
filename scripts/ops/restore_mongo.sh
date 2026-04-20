#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <mongo_archive_file>"
  exit 1
fi

ARCHIVE_FILE="$1"

if [[ ! -f "$ARCHIVE_FILE" ]]; then
  echo "archive file not found: $ARCHIVE_FILE"
  exit 1
fi

if [[ ! -f .env && -f .env.example ]]; then
  cp .env.example .env
fi

if [[ ! -f .env ]]; then
  echo "missing .env"
  exit 1
fi

set -a
source .env
set +a

docker cp "$ARCHIVE_FILE" mongo-db:/tmp/mongo_restore.archive.gz

docker compose exec -T mongo sh -lc '
  mongosh \
    --host localhost \
    --port 27017 \
    --tls \
    --tlsCAFile /tmp/mongo-certs/ca.pem \
    --tlsAllowInvalidHostnames \
    --username root \
    --password "$MONGO_INITDB_ROOT_PASSWORD" \
    --authenticationDatabase admin \
    --quiet \
    --eval '\''db.getSiblingDB("secure_voting").dropDatabase()'\''
'

docker compose exec -T mongo sh -lc '
  mongorestore \
    --uri="mongodb://root:${MONGO_INITDB_ROOT_PASSWORD}@localhost:27017/secure_voting?authSource=admin&tls=true&tlsCAFile=/tmp/mongo-certs/ca.pem&tlsAllowInvalidHostnames=true" \
    --archive=/tmp/mongo_restore.archive.gz \
    --gzip
  rm -f /tmp/mongo_restore.archive.gz
'

echo "mongo restore completed from: $ARCHIVE_FILE"