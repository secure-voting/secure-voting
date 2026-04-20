#!/usr/bin/env bash
set -euo pipefail

BACKEND_BASE="${BACKEND_BASE:-http://127.0.0.1:3001}"
FRONTEND_BASE="${FRONTEND_BASE:-https://127.0.0.1:8080}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TLS_CA_CERT="${TLS_CA_CERT:-$ROOT_DIR/scripts/certs/out/ca.pem}"
API_BASE="${API_BASE:-}"
TIMEOUT_SEC="${TIMEOUT_SEC:-90}"

: "${BOOTSTRAP_ADMIN_EMAIL:?BOOTSTRAP_ADMIN_EMAIL is required}"
: "${BOOTSTRAP_ADMIN_PASSWORD:?BOOTSTRAP_ADMIN_PASSWORD is required}"

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing command: $1" >&2
    exit 1
  }
}

need curl
need python3

HTTP_CODE=""
HTTP_BODY=""

do_curl() {
  local method="$1"; shift
  local url="$1"; shift
  local tmp
  tmp="$(mktemp)"
  HTTP_CODE="$(curl --cacert "$TLS_CA_CERT" -4sS -o "$tmp" -w "%{http_code}" -X "$method" "$url" "$@" || true)"
  HTTP_BODY="$(cat "$tmp" || true)"
  rm -f "$tmp"
}

assert_code() {
  local want="$1"
  if [[ "$HTTP_CODE" != "$want" ]]; then
    echo "ASSERT FAIL: expected HTTP $want, got $HTTP_CODE" >&2
    echo "$HTTP_BODY" >&2
    exit 1
  fi
}

detect_api_base() {
  if curl -4fsS "$BACKEND_BASE/health" >/dev/null 2>&1; then
    API_BASE="$BACKEND_BASE/api/v1"
    echo "API detected via backend: $API_BASE"
    return 0
  fi

  local tmp
  tmp="$(mktemp)"
  local code
  code="$(curl --cacert "$TLS_CA_CERT" -4sS -o "$tmp" -w "%{http_code}" "$FRONTEND_BASE/api/v1/auth/me" || true)"
  local body
  body="$(cat "$tmp" || true)"
  rm -f "$tmp"

  if [[ "$code" == "401" || "$code" == "200" ]]; then
    API_BASE="$FRONTEND_BASE/api/v1"
    echo "API detected via frontend proxy: $API_BASE"
    return 0
  fi

  echo "Cannot detect API base." >&2
  echo "Tried backend:  $BACKEND_BASE" >&2
  echo "Tried frontend: $FRONTEND_BASE" >&2
  echo "Last frontend probe code=$code body=$body" >&2
  exit 1
}

json_get() {
  local key="$1"
  python3 -c '
import json
import sys

path = sys.argv[1]
raw = sys.stdin.read()

try:
    cur = json.loads(raw)
except Exception:
    print("")
    raise SystemExit(0)

for part in path.split("."):
    if isinstance(cur, dict):
        if part not in cur:
            print("")
            raise SystemExit(0)
        cur = cur[part]
    elif isinstance(cur, list):
        try:
            idx = int(part)
        except ValueError:
            print("")
            raise SystemExit(0)
        if idx < 0 or idx >= len(cur):
            print("")
            raise SystemExit(0)
        cur = cur[idx]
    else:
        print("")
        raise SystemExit(0)

if isinstance(cur, (dict, list)):
    print(json.dumps(cur))
elif cur is None:
    print("")
else:
    print(cur)
' "$key" <<<"$HTTP_BODY"
}

extract_token() {
  python3 -c '
import json
import sys

raw = sys.argv[1] if len(sys.argv) > 1 else ""

try:
    obj = json.loads(raw)
except Exception:
    print("")
    raise SystemExit(0)

for key in ("access_token", "token", "accessToken"):
    value = obj.get(key)
    if isinstance(value, str) and value:
        print(value)
        raise SystemExit(0)

print("")
' "${1:-}"
}

response_error_code() {
  python3 -c '
import json
import sys

raw = sys.argv[1] if len(sys.argv) > 1 else ""

try:
    obj = json.loads(raw)
except Exception:
    print("")
    raise SystemExit(0)

err = obj.get("error")
if not isinstance(err, dict):
    print("")
    raise SystemExit(0)

code = err.get("code")
if isinstance(code, str):
    print(code)
else:
    print("")
' "${1:-}"
}

rand_suffix() {
  python3 - <<'PY'
import uuid
print(uuid.uuid4().hex[:10])
PY
}

new_idempotency_key() {
  python3 - <<'PY'
import uuid
print(uuid.uuid4())
PY
}

rfc3339_after_hours() {
  local hours="$1"
  python3 - "$hours" <<'PY'
from datetime import datetime, timedelta, timezone
import sys
hours = int(sys.argv[1])
print((datetime.now(timezone.utc) + timedelta(hours=hours)).replace(microsecond=0).isoformat().replace("+00:00", "Z"))
PY
}

find_rule_for_format() {
  local format="$1"

  do_curl GET "$API_BASE/capabilities/tally-rules" -H "$ADMIN_AUTH"
  assert_code 200

  python3 - "$format" "$HTTP_BODY" <<'PY'
import json
import sys

wanted = sys.argv[1]
raw = sys.argv[2] if len(sys.argv) > 2 else ""

try:
    obj = json.loads(raw)
except Exception:
    print("")
    raise SystemExit(0)

items = obj.get("items") or []
for item in items:
    if not isinstance(item, dict):
        continue
    if not item.get("supports_election_tally"):
        continue
    formats = item.get("ballot_formats") or []
    if wanted in formats:
        rid = item.get("id") or ""
        if isinstance(rid, str) and rid.strip():
            print(rid.strip())
            raise SystemExit(0)

print("")
PY
}

create_and_open_election() {
  local title="$1"
  local ballot_format="$2"
  local tally_rule="$3"
  local extra_json="$4"

  local start_at
  local end_at
  start_at="$(rfc3339_after_hours 1)"
  end_at="$(rfc3339_after_hours 2)"

  do_curl POST "$API_BASE/elections" \
    -H 'Content-Type: application/json' \
    -H "$ADMIN_AUTH" \
    -d "{
      \"title\":\"$title\",
      \"description\":\"vote formats e2e\",
      \"start_at\":\"$start_at\",
      \"end_at\":\"$end_at\",
      \"tally_rule\":\"$tally_rule\",
      \"ballot_format\":\"$ballot_format\",
      \"access_mode\":\"open\",
      \"show_aggregates\":true,
      \"committee_size\":1,
      $extra_json
      \"candidates\":[
        {\"name\":\"Alice\"},
        {\"name\":\"Bob\"},
        {\"name\":\"Carol\"}
      ]
    }"
  assert_code 200

  local election_id
  election_id="$(json_get id)"
  if [[ -z "$election_id" ]]; then
    echo "missing election id" >&2
    echo "$HTTP_BODY" >&2
    exit 1
  fi

  do_curl POST "$API_BASE/elections/$election_id/actions/schedule" -H "$ADMIN_AUTH"
  assert_code 200

  do_curl POST "$API_BASE/elections/$election_id/actions/open" -H "$ADMIN_AUTH"
  assert_code 200

  echo "$election_id"
}

get_candidate_ids() {
  local election_id="$1"

  do_curl GET "$API_BASE/elections/$election_id/ballot" -H "$VOTER_AUTH"
  assert_code 200

  C1="$(json_get candidates.0.id)"
  C2="$(json_get candidates.1.id)"
  C3="$(json_get candidates.2.id)"

  if [[ -z "$C1" || -z "$C2" || -z "$C3" ]]; then
    echo "failed to extract candidate ids" >&2
    echo "$HTTP_BODY" >&2
    exit 1
  fi
}

assert_my_ballot_ok() {
  local election_id="$1"

  do_curl GET "$API_BASE/elections/$election_id/ballots/me" -H "$VOTER_AUTH"
  assert_code 200

  local status
  status="$(json_get status)"
  if [[ -z "$status" ]]; then
    echo "missing ballot status" >&2
    echo "$HTTP_BODY" >&2
    exit 1
  fi
}

assert_results_hidden_before_publish() {
  local election_id="$1"

  do_curl GET "$API_BASE/elections/$election_id/results" -H "$VOTER_AUTH"
  if [[ "$HTTP_CODE" != "403" ]]; then
    echo "expected results to be hidden before publish, got HTTP $HTTP_CODE" >&2
    echo "$HTTP_BODY" >&2
    exit 1
  fi

  local code
  code="$(response_error_code "$HTTP_BODY")"
  if [[ "$code" != "not_published" ]]; then
    echo "expected error.code=not_published, got $code" >&2
    echo "$HTTP_BODY" >&2
    exit 1
  fi
}

wait_publish_and_assert_results() {
  local election_id="$1"

  local deadline
  deadline=$(( $(date +%s) + TIMEOUT_SEC ))

  while true; do
    do_curl POST "$API_BASE/elections/$election_id/actions/publish" -H "$ADMIN_AUTH"
    if [[ "$HTTP_CODE" == "200" ]]; then
      break
    fi

    if (( $(date +%s) >= deadline )); then
      echo "publish did not become available in time for election $election_id" >&2
      echo "$HTTP_BODY" >&2
      exit 1
    fi

    sleep 2
  done

  do_curl GET "$API_BASE/elections/$election_id/results" -H "$VOTER_AUTH"
  assert_code 200

  local winner0
  winner0="$(json_get winners.0)"
  if [[ -z "$winner0" ]]; then
    echo "missing winners in published results" >&2
    echo "$HTTP_BODY" >&2
    exit 1
  fi
}

close_publish_and_assert_results() {
  local election_id="$1"

  do_curl POST "$API_BASE/elections/$election_id/actions/close" -H "$ADMIN_AUTH"
  assert_code 200

  assert_results_hidden_before_publish "$election_id"
  wait_publish_and_assert_results "$election_id"
}

echo "== detect api base =="
detect_api_base

echo "== login bootstrap admin =="
do_curl POST "$API_BASE/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$BOOTSTRAP_ADMIN_EMAIL\",\"password\":\"$BOOTSTRAP_ADMIN_PASSWORD\"}"
assert_code 200

ADMIN_TOKEN="$(extract_token "$HTTP_BODY")"
if [[ -z "$ADMIN_TOKEN" ]]; then
  echo "failed to extract admin token" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi

ADMIN_AUTH="Authorization: Bearer $ADMIN_TOKEN"

APPROVAL_RULE="$(find_rule_for_format approval)"
RANKING_RULE="$(find_rule_for_format ranking)"
SCORE_RULE="$(find_rule_for_format score)"

if [[ -z "$APPROVAL_RULE" ]]; then
  echo "no election tally rule supports ballot_format=approval" >&2
  exit 1
fi
if [[ -z "$RANKING_RULE" ]]; then
  echo "no election tally rule supports ballot_format=ranking" >&2
  exit 1
fi
if [[ -z "$SCORE_RULE" ]]; then
  echo "no election tally rule supports ballot_format=score" >&2
  exit 1
fi

echo "approval_rule=$APPROVAL_RULE"
echo "ranking_rule=$RANKING_RULE"
echo "score_rule=$SCORE_RULE"

echo "== register voter =="
SFX="$(rand_suffix)"
VOTER_EMAIL="formats_${SFX}@local.dev"
VOTER_PASSWORD="voterpass1"

do_curl POST "$API_BASE/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$VOTER_EMAIL\",\"password\":\"$VOTER_PASSWORD\"}"
assert_code 200

VOTER_TOKEN="$(extract_token "$HTTP_BODY")"
if [[ -z "$VOTER_TOKEN" ]]; then
  echo "failed to extract voter token" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi
VOTER_AUTH="Authorization: Bearer $VOTER_TOKEN"

echo "== approval election =="
APPROVAL_ELECTION_ID="$(create_and_open_election "E2E approval $SFX" "approval" "$APPROVAL_RULE" "\"approval_max_choices\":2,")"
get_candidate_ids "$APPROVAL_ELECTION_ID"

do_curl POST "$API_BASE/elections/$APPROVAL_ELECTION_ID/ballots/submit" \
  -H 'Content-Type: application/json' \
  -H "$VOTER_AUTH" \
  -H "Idempotency-Key: $(new_idempotency_key)" \
  -d "{\"approval_set\":[\"$C1\",\"$C2\"]}"
assert_code 200

assert_my_ballot_ok "$APPROVAL_ELECTION_ID"
close_publish_and_assert_results "$APPROVAL_ELECTION_ID"

echo "== ranking election =="
RANKING_ELECTION_ID="$(create_and_open_election "E2E ranking $SFX" "ranking" "$RANKING_RULE" "\"ranking_top_k\":3,")"
get_candidate_ids "$RANKING_ELECTION_ID"

do_curl POST "$API_BASE/elections/$RANKING_ELECTION_ID/ballots/submit" \
  -H 'Content-Type: application/json' \
  -H "$VOTER_AUTH" \
  -H "Idempotency-Key: $(new_idempotency_key)" \
  -d "{\"ranking\":[\"$C1\",\"$C2\",\"$C3\"]}"
assert_code 200

assert_my_ballot_ok "$RANKING_ELECTION_ID"
close_publish_and_assert_results "$RANKING_ELECTION_ID"

echo "== score election =="
SCORE_ELECTION_ID="$(create_and_open_election "E2E score $SFX" "score" "$SCORE_RULE" "\"score_min\":0,\"score_max\":5,\"score_step\":1,\"score_allow_skip\":false,")"
get_candidate_ids "$SCORE_ELECTION_ID"

do_curl POST "$API_BASE/elections/$SCORE_ELECTION_ID/ballots/submit" \
  -H 'Content-Type: application/json' \
  -H "$VOTER_AUTH" \
  -H "Idempotency-Key: $(new_idempotency_key)" \
  -d "{\"scores\":{\"$C1\":5,\"$C2\":3,\"$C3\":1}}"
assert_code 200

assert_my_ballot_ok "$SCORE_ELECTION_ID"
close_publish_and_assert_results "$SCORE_ELECTION_ID"

echo
echo "E2E vote formats with publish/results: PASS"