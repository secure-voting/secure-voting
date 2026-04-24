#!/usr/bin/env bash
set -euo pipefail

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing $1"; exit 1; }; }
need curl
need jq

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
RUN_TIMEOUT_SECONDS="${RUN_TIMEOUT_SECONDS:-180}"

post_json() {
  local url="$1"; shift
  curl -sS -H "Content-Type: application/json" "$@" "$url"
}

get_auth() {
  local url="$1"; shift
  local token="$1"; shift
  curl -sS -H "Authorization: Bearer $token" "$url"
}

echo "== login bootstrap researcher =="
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

ME_JSON="$(get_auth "$BASE/auth/me" "$RESEARCHER_TOKEN")"
USER_ID="$(echo "$ME_JSON" | jq -r '.id // .user.id // empty')"
USER_ROLE="$(echo "$ME_JSON" | jq -r '.role // .user.role // empty')"
echo "user_id=$USER_ID role=$USER_ROLE"

if [[ "$USER_ROLE" != "researcher" && "$USER_ROLE" != "admin" ]]; then
  echo "unexpected role for experiment flow: $USER_ROLE"
  exit 1
fi

echo "== generate synthetic ranking dataset =="
DATASET_NAME="e2e-multi-rule-ranking-$(date +%s)"
DATASET_JSON="$(post_json "$BASE/datasets/generate" \
  -H "Authorization: Bearer $RESEARCHER_TOKEN" \
  -d "{
    \"name\": \"${DATASET_NAME}\",
    \"description\": \"multi-rule ranking synthetic dataset\",
    \"format\": \"ranking\",
    \"candidates\": [
      {\"id\": \"c1\", \"name\": \"Alice\"},
      {\"id\": \"c2\", \"name\": \"Bob\"},
      {\"id\": \"c3\", \"name\": \"Carol\"},
      {\"id\": \"c4\", \"name\": \"Dave\"}
    ],
    \"voters\": 30,
    \"seed\": 424242
  }")"

DATASET_ID="$(echo "$DATASET_JSON" | jq -r '.id // empty')"
if [[ -z "$DATASET_ID" || "$DATASET_ID" == "null" ]]; then
  echo "dataset generation failed:"
  echo "$DATASET_JSON" | jq .
  exit 1
fi
echo "dataset_id=$DATASET_ID"

RULES=(
  "plurality"
  "borda"
  "black"
  "simpson"
  "hare"
  "nanson"
  "coombs"
)

RESULTS_FILE="$(mktemp)"
echo "[]" > "$RESULTS_FILE"

for RULE in "${RULES[@]}"; do
  echo
  echo "== create experiment for rule: $RULE =="

  EXPERIMENT_JSON="$(post_json "$BASE/experiments" \
    -H "Authorization: Bearer $RESEARCHER_TOKEN" \
    -d "{
      \"type\": \"algo\",
      \"params\": {
        \"ballot_format\": \"ranking\",
        \"tally_rule\": \"${RULE}\",
        \"committee_size\": 1
      },
      \"seed\": 424242
    }")"

  EXPERIMENT_ID="$(echo "$EXPERIMENT_JSON" | jq -r '.id // empty')"
  if [[ -z "$EXPERIMENT_ID" || "$EXPERIMENT_ID" == "null" ]]; then
    echo "experiment creation failed for rule=$RULE"
    echo "$EXPERIMENT_JSON" | jq .
    exit 1
  fi
  echo "experiment_id=$EXPERIMENT_ID"

  echo "== create run batch for rule: $RULE =="

  BATCH_JSON="$(post_json "$BASE/experiment-runs/batch" \
    -H "Authorization: Bearer $RESEARCHER_TOKEN" \
    -d "{
      \"experiment_id\": \"${EXPERIMENT_ID}\",
      \"dataset_ids\": [\"${DATASET_ID}\"]
    }")"

  RUN_ID="$(echo "$BATCH_JSON" | jq -r '.items[0].run_id // .runs[0].run_id // .[0].run_id // .run_id // empty')"
  JOB_ID="$(echo "$BATCH_JSON" | jq -r '.items[0].job_id // .runs[0].job_id // .[0].job_id // .job_id // empty')"

  if [[ -z "$RUN_ID" || "$RUN_ID" == "null" ]]; then
    echo "run batch creation failed for rule=$RULE"
    echo "$BATCH_JSON" | jq .
    exit 1
  fi

  echo "run_id=$RUN_ID"
  echo "job_id=${JOB_ID:-<none>}"

  echo "== poll run for rule: $RULE =="

  deadline=$(( $(date +%s) + RUN_TIMEOUT_SECONDS ))
  RUN_STATUS=""
  RUN_JSON=""

  while [[ $(date +%s) -lt $deadline ]]; do
    RUN_JSON="$(get_auth "$BASE/experiment-runs/$RUN_ID" "$RESEARCHER_TOKEN" || true)"
    RUN_STATUS="$(echo "$RUN_JSON" | jq -r '.status // empty' 2>/dev/null || true)"
    echo "rule=$RULE status=$RUN_STATUS"

    if [[ "$RUN_STATUS" == "done" ]]; then
      break
    fi

    if [[ "$RUN_STATUS" == "error" ]]; then
      echo "experiment run finished with error for rule=$RULE"
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
    echo "experiment run did not finish as done for rule=$RULE"
    echo "$RUN_JSON" | jq .
    if [[ -n "${JOB_ID:-}" && "$JOB_ID" != "null" ]]; then
      echo "job snapshot:"
      get_auth "$BASE/jobs/$JOB_ID" "$RESEARCHER_TOKEN" | jq .
    fi
    exit 1
  fi

  echo "== fetch result for rule: $RULE =="

  RESULT_JSON="$(get_auth "$BASE/experiment-runs/$RUN_ID/result" "$RESEARCHER_TOKEN" || true)"
  RESULT_RUN_ID="$(echo "$RESULT_JSON" | jq -r '.run_id // empty' 2>/dev/null || true)"
  WINNERS_JSON="$(echo "$RESULT_JSON" | jq -c '.winners // []' 2>/dev/null || true)"

  if [[ "$RESULT_RUN_ID" != "$RUN_ID" ]]; then
    echo "unexpected result payload for rule=$RULE"
    echo "$RESULT_JSON" | jq .
    exit 1
  fi

  echo "winners=$WINNERS_JSON"

  tmp_file="$(mktemp)"
  jq --arg rule "$RULE" \
     --arg experiment_id "$EXPERIMENT_ID" \
     --arg run_id "$RUN_ID" \
     --arg job_id "${JOB_ID:-}" \
     --argjson winners "$WINNERS_JSON" \
     '. + [{
       rule: $rule,
       experiment_id: $experiment_id,
       run_id: $run_id,
       job_id: $job_id,
       winners: $winners
     }]' "$RESULTS_FILE" > "$tmp_file"
  mv "$tmp_file" "$RESULTS_FILE"
done

echo
echo "== summary =="
cat "$RESULTS_FILE" | jq .

echo
echo "E2E multi-rule experiment scenario: PASS"