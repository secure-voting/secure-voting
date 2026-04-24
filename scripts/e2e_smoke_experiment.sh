#!/usr/bin/env bash
set -euo pipefail

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing $1"; exit 1; }; }
need curl
need jq
need docker

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"

if [[ ! -f "$ROOT_DIR/.env" ]]; then
  echo "missing $ROOT_DIR/.env"
  exit 1
fi

set -a
. "$ROOT_DIR/.env"
set +a

: "${BOOTSTRAP_RESEARCHER_EMAIL:?missing BOOTSTRAP_RESEARCHER_EMAIL in .env}"
: "${BOOTSTRAP_RESEARCHER_PASSWORD:?missing BOOTSTRAP_RESEARCHER_PASSWORD in .env}"

BASE="${BASE:-http://127.0.0.1:3001/api/v1}"
RUN_TIMEOUT_SECONDS="${RUN_TIMEOUT_SECONDS:-300}"
BACKEND_HEALTH_TIMEOUT_SECONDS="${BACKEND_HEALTH_TIMEOUT_SECONDS:-120}"
ASYNC_READY_TIMEOUT_SECONDS="${ASYNC_READY_TIMEOUT_SECONDS:-120}"

dc() {
  (
    cd "$ROOT_DIR"
    docker compose "$@"
  )
}

dump_compose_debug() {
  echo
  echo "==== docker compose ps -a ===="
  dc ps -a || true
  echo
  echo "==== docker compose logs --tail=300 worker compute-runner compute backend kafka db ===="
  dc logs --tail=300 worker compute-runner compute backend kafka db || true
  echo
}

on_exit() {
  local rc=$?
  trap - EXIT
  if [[ $rc -ne 0 ]]; then
    echo
    echo "E2E experiment failed with exit code $rc"
    dump_compose_debug
  fi
  exit "$rc"
}
trap on_exit EXIT

post_json() {
  local url="$1"; shift
  curl -sS -H "Content-Type: application/json" "$@" "$url"
}

get_auth() {
  local url="$1"; shift
  local token="$1"; shift
  curl -sS -H "Authorization: Bearer $token" "$url"
}

wait_http_ok() {
  local url="$1"
  local timeout="$2"
  local started_at
  started_at="$(date +%s)"

  while true; do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi

    if (( $(date +%s) - started_at >= timeout )); then
      echo "timeout waiting for HTTP endpoint: $url"
      return 1
    fi

    sleep 2
  done
}

wait_service_running() {
  local service="$1"
  local timeout="$2"
  local started_at
  started_at="$(date +%s)"

  while true; do
    if dc ps --status running "$service" 2>/dev/null | grep -q "$service"; then
      return 0
    fi

    if (( $(date +%s) - started_at >= timeout )); then
      echo "timeout waiting for service to be running: $service"
      dc ps -a || true
      return 1
    fi

    sleep 2
  done
}

wait_log_pattern() {
  local service="$1"
  local pattern="$2"
  local timeout="$3"
  local started_at
  local logs

  started_at="$(date +%s)"

  while true; do
    logs="$(dc logs --no-color "$service" 2>/dev/null || true)"

    if grep -F -q -- "$pattern" <<<"$logs"; then
      return 0
    fi

    if (( $(date +%s) - started_at >= timeout )); then
      echo "timeout waiting for log pattern in service=$service pattern=$pattern"
      dc logs --tail=200 "$service" || true
      return 1
    fi

    sleep 2
  done
}

echo "[1/10] wait backend health"
wait_http_ok "http://127.0.0.1:3001/health" "$BACKEND_HEALTH_TIMEOUT_SECONDS"

echo "[2/10] wait worker container running"
wait_service_running "worker" "$ASYNC_READY_TIMEOUT_SECONDS"

echo "[3/10] wait compute-runner container running"
wait_service_running "compute-runner" "$ASYNC_READY_TIMEOUT_SECONDS"

echo "[4/10] wait worker readiness"
wait_log_pattern "worker" "worker started:" "$ASYNC_READY_TIMEOUT_SECONDS"

echo "[5/10] wait compute-runner readiness"
wait_log_pattern "compute-runner" "compute client connected:" "$ASYNC_READY_TIMEOUT_SECONDS"

echo "[6/10] login as bootstrap researcher"
LOGIN_JSON="$(post_json "$BASE/auth/login" -d "{
  \"email\": \"${BOOTSTRAP_RESEARCHER_EMAIL}\",
  \"password\": \"${BOOTSTRAP_RESEARCHER_PASSWORD}\"
}")"
RESEARCHER_TOKEN="$(echo "$LOGIN_JSON" | jq -r '.access_token // .token // .accessToken // empty')"

if [[ -z "$RESEARCHER_TOKEN" ]]; then
  echo "login failed:"
  echo "$LOGIN_JSON" | jq .
  exit 1
fi

ME="$(get_auth "$BASE/auth/me" "$RESEARCHER_TOKEN")"
USER_ID="$(echo "$ME" | jq -r '.id // .user.id // empty')"
USER_ROLE="$(echo "$ME" | jq -r '.role // .user.role // empty')"

if [[ -z "$USER_ID" || "$USER_ID" == "null" ]]; then
  echo "cannot get user id from /auth/me:"
  echo "$ME" | jq .
  exit 1
fi

if [[ "$USER_ROLE" != "researcher" && "$USER_ROLE" != "admin" ]]; then
  echo "expected bootstrap researcher/admin, got role=$USER_ROLE"
  echo "$ME" | jq .
  exit 1
fi

echo "user_id=$USER_ID role=$USER_ROLE"

echo "[7/10] generate dataset via API"
DATASET_NAME="e2e-exp-ranking-$(date +%s)"
DATASET_JSON="$(post_json "$BASE/datasets/generate" \
  -H "Authorization: Bearer $RESEARCHER_TOKEN" \
  -d "{
    \"name\": \"${DATASET_NAME}\",
    \"description\": \"researcher smoke dataset\",
    \"format\": \"ranking\",
    \"candidates\": [
      {\"id\": \"c1\", \"name\": \"Alice\"},
      {\"id\": \"c2\", \"name\": \"Bob\"},
      {\"id\": \"c3\", \"name\": \"Carol\"}
    ],
    \"voters\": 20,
    \"seed\": 42
  }")"

DATASET_ID="$(echo "$DATASET_JSON" | jq -r '.id // empty')"
if [[ -z "$DATASET_ID" || "$DATASET_ID" == "null" ]]; then
  echo "dataset generate failed:"
  echo "$DATASET_JSON" | jq .
  exit 1
fi
echo "dataset_id=$DATASET_ID"

echo "[8/10] create experiment via API"
EXPERIMENT_JSON="$(post_json "$BASE/experiments" \
  -H "Authorization: Bearer $RESEARCHER_TOKEN" \
  -d '{
    "type": "algo",
    "params": {
      "ballot_format": "ranking",
      "tally_rule": "plurality",
      "committee_size": 1
    },
    "seed": 42
  }')"

EXPERIMENT_ID="$(echo "$EXPERIMENT_JSON" | jq -r '.id // empty')"
if [[ -z "$EXPERIMENT_ID" || "$EXPERIMENT_ID" == "null" ]]; then
  echo "experiment create failed:"
  echo "$EXPERIMENT_JSON" | jq .
  exit 1
fi
echo "experiment_id=$EXPERIMENT_ID"

echo "[9/10] create experiment run batch via API"
BATCH_JSON="$(post_json "$BASE/experiment-runs/batch" \
  -H "Authorization: Bearer $RESEARCHER_TOKEN" \
  -d "{
    \"experiment_id\": \"${EXPERIMENT_ID}\",
    \"dataset_ids\": [\"${DATASET_ID}\"]
  }")"

RUN_ID="$(echo "$BATCH_JSON" | jq -r '.items[0].run_id // .runs[0].run_id // .[0].run_id // .run_id // empty')"
JOB_ID="$(echo "$BATCH_JSON" | jq -r '.items[0].job_id // .runs[0].job_id // .[0].job_id // .job_id // empty')"

if [[ -z "$RUN_ID" || "$RUN_ID" == "null" ]]; then
  echo "batch create failed:"
  echo "$BATCH_JSON" | jq .
  exit 1
fi
echo "run_id=$RUN_ID"
echo "job_id=${JOB_ID:-<none>}"

echo "[10/10] poll run until done/error"
deadline=$(( $(date +%s) + RUN_TIMEOUT_SECONDS ))
RUN_JSON=""
RUN_STATUS=""
while [[ $(date +%s) -lt $deadline ]]; do
  RUN_JSON="$(get_auth "$BASE/experiment-runs/$RUN_ID" "$RESEARCHER_TOKEN" || true)"
  RUN_STATUS="$(echo "$RUN_JSON" | jq -r '.status // empty' 2>/dev/null || true)"
  echo "run status=$RUN_STATUS"

  if [[ "$RUN_STATUS" == "done" ]]; then
    break
  fi

  if [[ "$RUN_STATUS" == "error" ]]; then
    echo "run finished with error:"
    echo "$RUN_JSON" | jq .
    if [[ -n "${JOB_ID:-}" && "$JOB_ID" != "null" ]]; then
      echo "job snapshot:"
      get_auth "$BASE/jobs/$JOB_ID" "$RESEARCHER_TOKEN" | jq .
    fi
    exit 1
  fi

  sleep 2
done

if [[ "$RUN_STATUS" != "done" ]]; then
  echo "run did not finish as done:"
  echo "$RUN_JSON" | jq .
  if [[ -n "${JOB_ID:-}" && "$JOB_ID" != "null" ]]; then
    echo "job snapshot:"
    get_auth "$BASE/jobs/$JOB_ID" "$RESEARCHER_TOKEN" | jq .
  fi
  exit 1
fi

if [[ -n "${JOB_ID:-}" && "$JOB_ID" != "null" ]]; then
  echo "[extra] fetch job snapshot"
  JOB_JSON="$(get_auth "$BASE/jobs/$JOB_ID" "$RESEARCHER_TOKEN" || true)"
  echo "$JOB_JSON" | jq .
fi

echo "[extra] fetch experiment result via API"
RESULT_JSON="$(get_auth "$BASE/experiment-runs/$RUN_ID/result" "$RESEARCHER_TOKEN" || true)"
echo "$RESULT_JSON" | jq .

RESULT_RUN_ID="$(echo "$RESULT_JSON" | jq -r '.run_id // empty' 2>/dev/null || true)"
if [[ "$RESULT_RUN_ID" != "$RUN_ID" ]]; then
  echo "unexpected result payload:"
  echo "$RESULT_JSON" | jq .
  exit 1
fi

echo "E2E experiment_run OK"