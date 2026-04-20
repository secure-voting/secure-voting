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

OUT_FILE="$OUT_DIR/mongo_secure_voting.archive.gz"

docker compose exec -T mongo sh -lc '
  mongodump \
    --uri="mongodb://root:${MONGO_INITDB_ROOT_PASSWORD}@localhost:27017/secure_voting?authSource=admin&tls=true&tlsCAFile=/tmp/mongo-certs/ca.pem&tlsAllowInvalidHostnames=true" \
    --archive \
    --gzip
' > "$OUT_FILE"

sha256sum "$OUT_FILE" > "$OUT_FILE.sha256"

echo "mongo backup created: $OUT_FILE"