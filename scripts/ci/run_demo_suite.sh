#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

unset COMPOSE_FILE
unset COMPOSE_PROFILES
unset COMPOSE_PROJECT_NAME

bash scripts/ci/run_integration_suite.sh
bash scripts/ci/run_restore_check.sh
bash scripts/ci/run_backup_rotation_check.sh
bash scripts/ci/run_prod_replication_smoke.sh
bash scripts/ci/run_replication_suite.sh
bash scripts/ci/run_e2e_suite.sh