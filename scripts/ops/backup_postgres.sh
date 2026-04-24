#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

OUT_DIR="${1:-$ROOT_DIR/.ci-artifacts/manual-backups/$(date -u +%Y%m%dT%H%M%SZ)}"
mkdir -p "$OUT_DIR"

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

OUT_FILE="$OUT_DIR/postgres_secure_voting.dump"

docker compose exec -T db sh -lc '
  export PGPASSWORD="$POSTGRES_PASSWORD"
  pg_dump -U admin -d secure-voting --format=custom --no-owner --no-privileges
' > "$OUT_FILE"

sha256sum "$OUT_FILE" > "$OUT_FILE.sha256"

echo "postgres backup created: $OUT_FILE"