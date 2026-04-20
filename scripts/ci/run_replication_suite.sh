#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

bash scripts/ci/run_postgres_replication_check.sh
bash scripts/ci/run_redis_replication_check.sh
bash scripts/ci/run_mongo_replication_check.sh
