#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <postgres_dump_file>"
  exit 1
fi

DUMP_FILE="$1"

if [[ ! -f "$DUMP_FILE" ]]; then
  echo "dump file not found: $DUMP_FILE"
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

docker cp "$DUMP_FILE" postgres-db:/tmp/postgres_restore.dump

docker compose exec -T db sh -lc '
  export PGPASSWORD="$POSTGRES_PASSWORD"
  psql -U admin -d postgres -v ON_ERROR_STOP=1 <<SQL
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = '\''secure-voting'\''
  AND pid <> pg_backend_pid();
DROP DATABASE IF EXISTS "secure-voting";
CREATE DATABASE "secure-voting";
SQL
'

docker compose exec -T db sh -lc '
  export PGPASSWORD="$POSTGRES_PASSWORD"
  pg_restore -U admin -d secure-voting --no-owner --no-privileges /tmp/postgres_restore.dump
  rm -f /tmp/postgres_restore.dump
'

echo "postgres restore completed from: $DUMP_FILE"