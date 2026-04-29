#!/usr/bin/env bash
set -euo pipefail

BACKEND_BASE="${BACKEND_BASE:-http://127.0.0.1:3001}"
FRONTEND_BASE="${FRONTEND_BASE:-https://127.0.0.1:8080}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TLS_CA_CERT="${TLS_CA_CERT:-$ROOT_DIR/scripts/certs/out/ca.pem}"
API_BASE="${API_BASE:-}"
TIMEOUT_SEC="${TIMEOUT_SEC:-60}"
START_AT="$(date -u -d '+10 minutes' +%Y-%m-%dT%H:%M:%SZ)"
END_AT="$(date -u -d '+2 days' +%Y-%m-%dT%H:%M:%SZ)"

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

assert_code() {
  local want="$1"
  if [[ "$HTTP_CODE" != "$want" ]]; then
    echo "ASSERT FAIL: expected HTTP $want, got $HTTP_CODE" >&2
    echo "$HTTP_BODY" >&2
    exit 1
  fi
}

detect_api_base() {
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

  if curl -4fsS "$BACKEND_BASE/health" >/dev/null 2>&1; then
    API_BASE="$BACKEND_BASE/api/v1"
    echo "API detected via backend: $API_BASE"
    return 0
  fi

  echo "Cannot detect API base." >&2
  echo "Tried frontend: $FRONTEND_BASE" >&2
  echo "Tried backend:  $BACKEND_BASE" >&2
  echo "Last frontend probe code=$code body=$body" >&2
  exit 1
}

rand_suffix() {
  python3 - <<'PY'
import uuid
print(uuid.uuid4().hex[:10])
PY
}

wait_until() {
  local desc="$1"
  local url="$2"
  local auth_header="$3"
  local want_code="$4"

  local deadline=$((SECONDS + TIMEOUT_SEC))
  while (( SECONDS < deadline )); do
    do_curl GET "$url" -H "$auth_header"
    if [[ "$HTTP_CODE" == "$want_code" ]]; then
      echo "OK: $desc"
      return 0
    fi
    sleep 1
  done

  echo "TIMEOUT: $desc" >&2
  echo "Last HTTP=$HTTP_CODE body=$HTTP_BODY" >&2
  exit 1
}

wait_until_publish_succeeds() {
  local election_id="$1"
  local deadline=$((SECONDS + TIMEOUT_SEC))

  while (( SECONDS < deadline )); do
    do_curl POST "$API_BASE/elections/$election_id/actions/publish" -H "$ADMIN_AUTH"
    if [[ "$HTTP_CODE" == "200" ]]; then
      echo "OK: publish succeeded"
      return 0
    fi
    sleep 1
  done

  echo "TIMEOUT: publish did not succeed" >&2
  echo "Last HTTP=$HTTP_CODE body=$HTTP_BODY" >&2
  exit 1
}

echo "== detect api base =="
detect_api_base

echo "== login bootstrap admin =="
do_curl POST "$API_BASE/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$BOOTSTRAP_ADMIN_EMAIL\",\"password\":\"$BOOTSTRAP_ADMIN_PASSWORD\",\"replace_existing_session\":true}"
assert_code 200

ADMIN_TOKEN="$(extract_token "$HTTP_BODY")"
if [[ -z "$ADMIN_TOKEN" ]]; then
  echo "failed to extract admin token" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi
ADMIN_AUTH="Authorization: Bearer $ADMIN_TOKEN"

echo "== register voter =="
SFX="$(rand_suffix)"
VOTER_EMAIL="voter_${SFX}@local.dev"
VOTER_PASSWORD="voterpass1"

do_curl POST "$API_BASE/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$VOTER_EMAIL\",\"password\":\"$VOTER_PASSWORD\"}"
assert_code 200

VOTER_TOKEN="$(extract_token "$HTTP_BODY")"
if [[ -z "$VOTER_TOKEN" ]]; then
  echo "failed to extract voter token from register response" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi
VOTER_AUTH="Authorization: Bearer $VOTER_TOKEN"

echo "== create election =="
do_curl POST "$API_BASE/elections" \
  -H 'Content-Type: application/json' \
  -H "$ADMIN_AUTH" \
  -d "{
    \"title\":\"E2E election lifecycle $SFX\",
    \"description\":\"user-flow e2e\",
    \"start_at\":\"$START_AT\",
    \"end_at\":\"$END_AT\",
    \"tally_rule\":\"plurality\",
    \"ballot_format\":\"ranking\",
    \"access_mode\":\"open\",
    \"show_aggregates\":true,
    \"committee_size\":1,
    \"ranking_top_k\":3,
    \"candidates\":[
      {\"name\":\"Alice\"},
      {\"name\":\"Bob\"},
      {\"name\":\"Carol\"}
    ]
  }"
assert_code 200

ELECTION_ID="$(json_get id)"
if [[ -z "$ELECTION_ID" ]]; then
  echo "missing election id" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi

echo "== schedule and open election =="
do_curl POST "$API_BASE/elections/$ELECTION_ID/actions/schedule" -H "$ADMIN_AUTH"
assert_code 200

do_curl POST "$API_BASE/elections/$ELECTION_ID/actions/open" -H "$ADMIN_AUTH"
assert_code 200

echo "== get ballot meta =="
do_curl GET "$API_BASE/elections/$ELECTION_ID/ballot" -H "$VOTER_AUTH"
assert_code 200

C1="$(json_get candidates.0.id)"
C2="$(json_get candidates.1.id)"
C3="$(json_get candidates.2.id)"

if [[ -z "$C1" || -z "$C2" || -z "$C3" ]]; then
  echo "failed to extract candidate ids" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi

echo "== submit ranking ballot =="
IDEMPOTENCY_KEY="$(python3 - <<'PY'
import uuid
print(uuid.uuid4())
PY
)"

do_curl POST "$API_BASE/elections/$ELECTION_ID/ballots/submit" \
  -H 'Content-Type: application/json' \
  -H "$VOTER_AUTH" \
  -H "Idempotency-Key: $IDEMPOTENCY_KEY" \
  -d "{\"ranking\":[\"$C1\",\"$C2\",\"$C3\"]}"
assert_code 200

echo "== close election =="
do_curl POST "$API_BASE/elections/$ELECTION_ID/actions/close" -H "$ADMIN_AUTH"
assert_code 200

echo "== verify voter cannot read results before publish =="
do_curl GET "$API_BASE/elections/$ELECTION_ID/results" -H "$VOTER_AUTH"
if [[ "$HTTP_CODE" != "403" ]]; then
  echo "expected voter results before publish to be 403, got $HTTP_CODE" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi

ERROR_CODE="$(json_get error.code)"
if [[ "$ERROR_CODE" != "not_published" ]]; then
  echo "expected error.code=not_published, got $ERROR_CODE" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi

echo "== wait until publish becomes available =="
wait_until_publish_succeeds "$ELECTION_ID"

echo "== verify voter can read results after publish =="
do_curl GET "$API_BASE/elections/$ELECTION_ID/results" -H "$VOTER_AUTH"
assert_code 200

PUBLISHED_AT="$(json_get published_at)"
if [[ -z "$PUBLISHED_AT" ]]; then
  echo "expected published_at in results response" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi

echo
echo "E2E election lifecycle: PASS"