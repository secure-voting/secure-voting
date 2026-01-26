#!/usr/bin/env bash
set -euo pipefail

BACKEND_BASE="${BACKEND_BASE:-http://localhost:3001}"
FRONTEND_BASE="${FRONTEND_BASE:-http://localhost:8080}"
TIMEOUT_SEC="${TIMEOUT_SEC:-20}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing command: $1" >&2; exit 1; }; }
need curl
need python3
need docker

HTTP_CODE=""
HTTP_BODY=""

do_curl() {
  local method="$1"; shift
  local url="$1"; shift
  local tmp
  tmp="$(mktemp)"
  HTTP_CODE="$(curl -4sS -o "$tmp" -w "%{http_code}" -X "$method" "$url" "$@" || true)"
  HTTP_BODY="$(cat "$tmp" || true)"
  rm -f "$tmp"
}

do_curl_headers() {
  curl -4sS -D - -o /dev/null "$@" || true
}

extract_token() {
  python3 -c '
import json,sys
s=sys.stdin.read()
try:
  o=json.loads(s)
except Exception:
  print("")
  sys.exit(0)
for k in ("access_token","accessToken","token"):
  v=o.get(k)
  if isinstance(v,str) and v:
    print(v)
    sys.exit(0)
print("")
' <<<"${1:-}"
}

json_get() {
  local key="$1"
  python3 -c '
import json,sys
key=sys.argv[1]
s=sys.stdin.read()
try:
  obj=json.loads(s)
except Exception:
  print("")
  sys.exit(0)

cur=obj
for part in key.split("."):
  if isinstance(cur, dict) and part in cur:
    cur=cur[part]
  else:
    print("")
    sys.exit(0)

if isinstance(cur,(dict,list)):
  import json as _j
  print(_j.dumps(cur))
else:
  print(cur)
' "$key" <<<"$HTTP_BODY"
}

assert_code() {
  local want="$1"
  if [[ "$HTTP_CODE" != "$want" ]]; then
    echo "ASSERT FAIL: expected HTTP $want, got $HTTP_CODE" >&2
    echo "$HTTP_BODY" >&2
    exit 1
  fi
}

wait_until() {
  local desc="$1"
  local cmd="$2"
  local end=$((SECONDS + TIMEOUT_SEC))
  while (( SECONDS < end )); do
    if eval "$cmd" >/dev/null 2>&1; then
      echo "OK: $desc"
      return 0
    fi
    sleep 1
  done
  echo "TIMEOUT: $desc" >&2
  return 1
}

rand_suffix() {
  python3 -c 'import uuid; print(uuid.uuid4().hex[:10])'
}

echo "== smoke: backend health =="
do_curl GET "$BACKEND_BASE/health"
assert_code 200

echo "== smoke: frontend serves HTML =="
do_curl GET "$FRONTEND_BASE/"
assert_code 200

echo "== detect: is API reachable via frontend (nginx proxy)? =="
CTYPE="$(do_curl_headers "$FRONTEND_BASE/api/v1/auth/me" | awk -F': ' 'tolower($1)=="content-type"{print tolower($2)}' | tail -n1 | tr -d '\r')"
if [[ "$CTYPE" == *"application/json"* ]]; then
  API_BASE="$FRONTEND_BASE"
  echo "API via frontend: YES ($FRONTEND_BASE -> backend)"
else
  API_BASE="$BACKEND_BASE"
  echo "API via frontend: NO (using backend directly: $BACKEND_BASE)"
fi

echo "== auth: create unique admin + voter =="
SFX="$(rand_suffix)"
ADMIN_EMAIL="admin_${SFX}@local.dev"
VOTER_EMAIL="voter_${SFX}@local.dev"
ADMIN_PASS="adminadmin"
VOTER_PASS="voterpass1"

echo "admin=$ADMIN_EMAIL pass=$ADMIN_PASS"
echo "voter=$VOTER_EMAIL pass=$VOTER_PASS"

# register admin (обычно возвращает access_token)
do_curl POST "$API_BASE/api/v1/auth/register" -H 'content-type: application/json' \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASS\",\"role\":\"admin\"}"

if [[ "$HTTP_CODE" == "200" ]]; then
  ADMIN_TOKEN="$(extract_token "$HTTP_BODY")"
else
  do_curl POST "$API_BASE/api/v1/auth/login" -H 'content-type: application/json' \
    -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASS\"}"
  if [[ "$HTTP_CODE" != "200" ]]; then
    echo "admin login failed: HTTP $HTTP_CODE" >&2
    echo "$HTTP_BODY" >&2
    exit 1
  fi
  ADMIN_TOKEN="$(extract_token "$HTTP_BODY")"
fi

if [[ -z "${ADMIN_TOKEN:-}" ]]; then
  echo "no admin token; last HTTP=$HTTP_CODE body:" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi
AUTH="Authorization: Bearer $ADMIN_TOKEN"

# register voter
do_curl POST "$API_BASE/api/v1/auth/register" -H 'content-type: application/json' \
  -d "{\"email\":\"$VOTER_EMAIL\",\"password\":\"$VOTER_PASS\",\"role\":\"voter\"}"

if [[ "$HTTP_CODE" == "200" ]]; then
  VOTER_TOKEN="$(extract_token "$HTTP_BODY")"
else
  do_curl POST "$API_BASE/api/v1/auth/login" -H 'content-type: application/json' \
    -d "{\"email\":\"$VOTER_EMAIL\",\"password\":\"$VOTER_PASS\"}"
  if [[ "$HTTP_CODE" != "200" ]]; then
    echo "voter login failed: HTTP $HTTP_CODE" >&2
    echo "$HTTP_BODY" >&2
    exit 1
  fi
  VOTER_TOKEN="$(extract_token "$HTTP_BODY")"
fi

if [[ -z "${VOTER_TOKEN:-}" ]]; then
  echo "no voter token; last HTTP=$HTTP_CODE body:" >&2
  echo "$HTTP_BODY" >&2
  exit 1
fi
VAUTH="Authorization: Bearer $VOTER_TOKEN"

echo "== sanity: /auth/me (admin+voter) =="
do_curl GET "$API_BASE/api/v1/auth/me" -H "$AUTH"
assert_code 200
do_curl GET "$API_BASE/api/v1/auth/me" -H "$VAUTH"
assert_code 200

echo "== suite A: state machine + pause/resume + publish gating =="
do_curl POST "$API_BASE/api/v1/elections" -H 'content-type: application/json' -H "$AUTH" -d "{
  \"title\":\"E2E state machine $SFX\",
  \"description\":\"state transitions\",
  \"start_at\":\"2026-01-26T00:00:00Z\",
  \"end_at\":\"2026-02-26T00:00:00Z\",
  \"tally_rule\":\"plurality\",
  \"ballot_format\":\"ranking\",
  \"access_mode\":\"open\",
  \"show_aggregates\":true,
  \"committee_size\":1,
  \"ranking_top_k\":3,
  \"candidates\":[{\"name\":\"A\"},{\"name\":\"B\"},{\"name\":\"C\"}]
}"
assert_code 200
EID="$(json_get id)"
[[ -n "$EID" ]] || { echo "no EID" >&2; exit 1; }

do_curl POST "$API_BASE/api/v1/elections/$EID/actions/open" -H "$AUTH"
[[ "$HTTP_CODE" == "409" ]] || { echo "expected 409 on open from draft, got $HTTP_CODE" >&2; echo "$HTTP_BODY" >&2; exit 1; }

do_curl POST "$API_BASE/api/v1/elections/$EID/actions/schedule" -H "$AUTH"; assert_code 200
do_curl POST "$API_BASE/api/v1/elections/$EID/actions/open" -H "$AUTH"; assert_code 200
do_curl POST "$API_BASE/api/v1/elections/$EID/actions/pause" -H "$AUTH"; assert_code 200

do_curl GET "$API_BASE/api/v1/elections/$EID/ballot" -H "$VAUTH"; assert_code 200

C1="$(python3 -c 'import json,sys; b=json.load(sys.stdin); print(b["candidates"][0]["id"])' <<<"$HTTP_BODY")"
C2="$(python3 -c 'import json,sys; b=json.load(sys.stdin); print(b["candidates"][1]["id"])' <<<"$HTTP_BODY")"
C3="$(python3 -c 'import json,sys; b=json.load(sys.stdin); print(b["candidates"][2]["id"])' <<<"$HTTP_BODY")"

IDK1="$(python3 -c 'import uuid; print(uuid.uuid4())')"
do_curl POST "$API_BASE/api/v1/elections/$EID/ballots/submit" -H 'content-type: application/json' -H "$VAUTH" -H "Idempotency-Key: $IDK1" \
  -d "{\"ranking\":[\"$C1\",\"$C2\",\"$C3\"]}"
[[ "$HTTP_CODE" == "409" ]] || { echo "expected 409 while paused, got $HTTP_CODE" >&2; echo "$HTTP_BODY" >&2; exit 1; }

do_curl POST "$API_BASE/api/v1/elections/$EID/actions/resume" -H "$AUTH"; assert_code 200

IDK2="$(python3 -c 'import uuid; print(uuid.uuid4())')"
do_curl POST "$API_BASE/api/v1/elections/$EID/ballots/submit" -H 'content-type: application/json' -H "$VAUTH" -H "Idempotency-Key: $IDK2" \
  -d "{\"ranking\":[\"$C1\",\"$C2\",\"$C3\"]}"
assert_code 200

do_curl POST "$API_BASE/api/v1/elections/$EID/actions/close" -H "$AUTH"; assert_code 200

wait_until "results available for admin" "curl -4fsS \"$API_BASE/api/v1/elections/$EID/results\" -H \"$AUTH\" >/dev/null"

do_curl GET "$API_BASE/api/v1/elections/$EID/results" -H "$VAUTH"
[[ "$HTTP_CODE" == "403" ]] || { echo "expected 403 before publish, got $HTTP_CODE" >&2; echo "$HTTP_BODY" >&2; exit 1; }

do_curl POST "$API_BASE/api/v1/elections/$EID/actions/publish" -H "$AUTH"; assert_code 200

do_curl GET "$API_BASE/api/v1/elections/$EID/results" -H "$VAUTH"; assert_code 200
PA="$(json_get published_at)"
[[ -n "$PA" ]] || { echo "expected published_at for voter" >&2; echo "$HTTP_BODY" >&2; exit 1; }

echo "== frontend<->backend integration checks =="
if [[ "$API_BASE" == "$FRONTEND_BASE" ]]; then
  echo "frontend proxy path is used; API calls validated through nginx."
else
  echo "no nginx proxy detected; checking CORS preflight for browser calls to :3001 from :8080"
  PREF="$(curl -4sS -i -X OPTIONS "$BACKEND_BASE/api/v1/auth/login" \
    -H "Origin: $FRONTEND_BASE" \
    -H "Access-Control-Request-Method: POST" \
    -H "Access-Control-Request-Headers: content-type" || true)"
  echo "$PREF" | sed -n '1,25p'
  echo "$PREF" | grep -qi "access-control-allow-origin" || {
    echo "WARNING: no Access-Control-Allow-Origin in preflight response. Browser frontend may fail if it calls backend directly." >&2
  }
fi

echo
echo "ALL TESTS: PASS"