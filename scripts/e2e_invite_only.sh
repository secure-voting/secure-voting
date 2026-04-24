#!/usr/bin/env bash
set -euo pipefail

BACKEND_BASE="${BACKEND_BASE:-http://127.0.0.1:3001}"
FRONTEND_BASE="${FRONTEND_BASE:-https://127.0.0.1:8080}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TLS_CA_CERT="${TLS_CA_CERT:-$ROOT_DIR/scripts/certs/out/ca.pem}"
API_BASE="${API_BASE:-}"
TIMEOUT_SEC="${TIMEOUT_SEC:-60}"

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

rand_suffix() {
  python3 - <<'PY'
import uuid
print(uuid.uuid4().hex[:10])
PY
}

START_AT="$(date -u -d '+10 minutes' +%Y-%m-%dT%H:%M:%SZ)"
END_AT="$(date -u -d '+2 days' +%Y-%m-%dT%H:%M:%SZ)"

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

SFX="$(rand_suffix)"
INVITED_EMAIL="invited_${SFX}@local.dev"
INVITED_PASSWORD="invitedpass1"
PLAIN_EMAIL="plain_${SFX}@local.dev"
PLAIN_PASSWORD="plainpass1"

echo "== create invite-only election =="
do_curl POST "$API_BASE/elections" \
  -H 'Content-Type: application/json' \
  -H "$ADMIN_AUTH" \
  -d "{
    \"title\":\"E2E invite-only $SFX\",
    \"description\":\"invite-only flow\",
    \"start_at\":\"$START_AT\",
    \"end_at\":\"$END_AT\",
    \"tally_rule\":\"plurality\",
    \"ballot_format\":\"ranking\",
    \"access_mode\":\"invite\",
    \"show_aggregates\":true,
    \"committee_size\":1,
    \"ranking_top_k\":3,
    \"candidates\":[
      {\"name\":\"Alice Alpha\",\"meta\":{\"description\":\"Candidate 1\"}},
      {\"name\":\"Boris Beta\",\"meta\":{\"description\":\"Candidate 2\"}},
      {\"name\":\"Carol Gamma\",\"meta\":{\"description\":\"Candidate 3\"}}
    ]
  }"
assert_code 200

ELECTION_ID="$(json_get id)"
if [[ -z "$ELECTION_ID" ]]; then
  echo "missing election id" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi

echo "== register invited voter without invite =="
do_curl POST "$API_BASE/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$INVITED_EMAIL\",\"password\":\"$INVITED_PASSWORD\"}"
assert_code 200

INVITED_TOKEN="$(extract_token "$HTTP_BODY")"
if [[ -z "$INVITED_TOKEN" ]]; then
  echo "failed to extract invited voter token" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi

echo "== register ordinary voter without invite =="
do_curl POST "$API_BASE/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$PLAIN_EMAIL\",\"password\":\"$PLAIN_PASSWORD\"}"
assert_code 200

PLAIN_TOKEN="$(extract_token "$HTTP_BODY")"
if [[ -z "$PLAIN_TOKEN" ]]; then
  echo "failed to extract plain voter token" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi
PLAIN_AUTH="Authorization: Bearer $PLAIN_TOKEN"

echo "== create invite for already registered invited voter =="
do_curl POST "$API_BASE/elections/$ELECTION_ID/invites" \
  -H 'Content-Type: application/json' \
  -H "$ADMIN_AUTH" \
  -d "{\"email\":\"$INVITED_EMAIL\"}"
assert_code 200

INVITE_CODE="$(json_get invite_code)"
if [[ -z "$INVITE_CODE" ]]; then
  echo "missing invite_code in create invite response" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi

echo "== invited voter accepts invite via login with invite_code =="
do_curl POST "$API_BASE/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$INVITED_EMAIL\",\"password\":\"$INVITED_PASSWORD\",\"invite_code\":\"$INVITE_CODE\"}"
assert_code 200

INVITED_TOKEN="$(extract_token "$HTTP_BODY")"
if [[ -z "$INVITED_TOKEN" ]]; then
  echo "failed to extract invited voter token after invite acceptance" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi
INVITED_AUTH="Authorization: Bearer $INVITED_TOKEN"

echo "== invited voter can access ballot meta =="
do_curl GET "$API_BASE/elections/$ELECTION_ID/ballot" -H "$INVITED_AUTH"
assert_code 200

echo "== ordinary voter cannot access ballot meta =="
do_curl GET "$API_BASE/elections/$ELECTION_ID/ballot" -H "$PLAIN_AUTH"
if [[ "$HTTP_CODE" == "200" ]]; then
  echo "expected non-invited voter to be denied, but got 200" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi

case "$HTTP_CODE" in
  403|404)
    echo "OK: ordinary voter denied with HTTP $HTTP_CODE"
    ;;
  *)
    echo "expected HTTP 403 or 404 for non-invited voter, got $HTTP_CODE" >&2
    echo "$HTTP_BODY" >&2
    exit 1
    ;;
esac

echo
echo "E2E invite-only: PASS"