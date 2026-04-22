#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

unset COMPOSE_FILE
unset COMPOSE_PROFILES
unset COMPOSE_PROJECT_NAME

OUT_ROOT="${1:-$ROOT_DIR/.backups}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT_DIR="$OUT_ROOT/$STAMP"

mkdir -p "$OUT_DIR"

bash scripts/ops/backup_postgres.sh "$OUT_DIR"
bash scripts/ops/backup_mongo.sh "$OUT_DIR"

cat > "$OUT_DIR/manifest.txt" <<EOF
created_at_utc=$STAMP
host=$(hostname)
repo=secure-voting
branch_hint=feat/dev-version-0.3.0
EOF

find "$OUT_DIR" -maxdepth 1 -type f -print | sort > "$OUT_DIR/files.txt"

echo "backup bundle created: $OUT_DIR"