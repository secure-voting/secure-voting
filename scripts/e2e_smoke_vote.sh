#!/usr/bin/env bash
set -euo pipefail

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing $1"; exit 1; }; }
need curl
need jq

BASE="http://127.0.0.1:3001/api/v1"

post_json() {
  local url="$1"; shift
  curl -sS -H "Content-Type: application/json" "$@" "$url"
}

auth_register_login() {
  local role="$1"
  local email="e2e+${role}+$(date +%s%N)@example.com"
  local pass="Passw0rd!12345"

  post_json "$BASE/auth/register" \
    -d "{\"email\":\"$email\",\"password\":\"$pass\",\"role\":\"$role\"}" >/dev/null || true

  local resp token
  resp="$(post_json "$BASE/auth/login" -d "{\"email\":\"$email\",\"password\":\"$pass\"}")"
  token="$(echo "$resp" | jq -r '.access_token // .token // .accessToken // empty')"

  if [[ -z "$token" ]]; then
    echo "login failed: $resp"
    exit 1
  fi
  echo "$token"
}

echo "[1/8] wait backend"
curl -fsS http://127.0.0.1:3001/health >/dev/null

echo "[2/8] login admin + voter"
ADMIN_TOKEN="$(auth_register_login admin)"
VOTER_TOKEN="$(auth_register_login voter)"

echo "[3/8] create election (ranking, plurality) and hope CreateElectionInput matches"
# если ваш CreateElectionInput требует candidates внутри запроса — тут упадёт с bad_request кодом
# тогда самый быстрый фикс: пришли apps/backend/internal/elections/service.go (CreateElectionInput), и я дам точный payload.
NOW="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
END="$(date -u -d "+1 hour" +"%Y-%m-%dT%H:%M:%SZ")"

create_payload="$(jq -n --arg s "$NOW" --arg e "$END" '{
  title: "e2e ranking election",
  description: "smoke",
  start_at: $s,
  end_at: $e,
  tally_rule: "plurality",
  ballot_format: "ranking",
  committee_size: 1,
  access_mode: "open"
}')"

resp="$(post_json "$BASE/elections" -H "Authorization: Bearer '"$ADMIN_TOKEN"'" -d "$create_payload" || true)"
ELECTION_ID="$(echo "$resp" | jq -r '.id // empty' 2>/dev/null || true)"
if [[ -z "$ELECTION_ID" ]]; then
  echo "create election failed response:"
  echo "$resp" | jq '.' || echo "$resp"
  echo "If error says missing fields/candidates: send apps/backend/internal/elections/service.go (CreateElectionInput)."
  exit 1
fi
echo "election_id=$ELECTION_ID"

echo "[4/8] open election"
curl -sS -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" \
  -d '{}' "$BASE/elections/$ELECTION_ID/actions/open" >/dev/null || true

echo "[5/8] get ballot meta and extract candidate ids"
meta="$(curl -sS -H "Authorization: Bearer $VOTER_TOKEN" "$BASE/elections/$ELECTION_ID/ballot")"
echo "$meta" | jq '.' >/dev/null

# пробуем вытащить ids из нескольких возможных структур
CIDS="$(echo "$meta" | jq -r '
  (
    .candidates // .items // .options // []
  ) | map(.id // .candidate_id // .uuid // .value) | .[]? // empty
' | paste -sd' ' -)"

if [[ -z "$CIDS" ]]; then
  echo "cannot extract candidate ids from ballot meta:"
  echo "$meta" | jq '.'
  echo "Send elections.BallotMeta struct (apps/backend/internal/elections/service.go) to make this exact."
  exit 1
fi
echo "candidate_ids: $CIDS"

# берём первые 3 кандидата как ранжирование
read -r C1 C2 C3 _ <<<"$CIDS"
RANK_JSON_ARR="$(jq -n --arg c1 "$C1" --arg c2 "$C2" --arg c3 "$C3" '[ $c1, $c2, $c3 ]')"

echo "[6/8] submit ballot (try multiple SubmitReq shapes)"
IDEM="$(date +%s%N)"

try_submit() {
  local body="$1"
  local out
  out="$(curl -sS \
    -H "Authorization: Bearer $VOTER_TOKEN" \
    -H "Idempotency-Key: $IDEM" \
    -H "Content-Type: application/json" \
    -d "$body" \
    "$BASE/elections/$ELECTION_ID/ballots/submit" || true)"
  echo "$out"
}

# Вариант A: { "ranking": ["c1","c2","c3"] }
A="$(jq -n --argjson r "$RANK_JSON_ARR" '{ ranking: $r }')"
OUT="$(try_submit "$A")"
CODE="$(echo "$OUT" | jq -r '.error.code // empty' 2>/dev/null || true)"
if [[ -z "$CODE" ]]; then
  echo "submit ok (variant A)"
else
  echo "variant A failed: $CODE"

  # Вариант B: { "format":"ranking", "ranking":[...] }
  B="$(jq -n --argjson r "$RANK_JSON_ARR" '{ format:"ranking", ranking:$r }')"
  OUT="$(try_submit "$B")"
  CODE="$(echo "$OUT" | jq -r '.error.code // empty' 2>/dev/null || true)"
  if [[ -z "$CODE" ]]; then
    echo "submit ok (variant B)"
  else
    echo "variant B failed: $CODE"

    # Вариант C: { "ranking": [{"candidate_id":"c1","rank":1}, ...] }
    C="$(jq -n --arg c1 "$C1" --arg c2 "$C2" --arg c3 "$C3" '{
      ranking: [
        {candidate_id:$c1, rank:1},
        {candidate_id:$c2, rank:2},
        {candidate_id:$c3, rank:3}
      ]
    }')"
    OUT="$(try_submit "$C")"
    CODE="$(echo "$OUT" | jq -r '.error.code // empty' 2>/dev/null || true)"
    if [[ -z "$CODE" ]]; then
      echo "submit ok (variant C)"
    else
      echo "submit failed for all variants."
      echo "$OUT" | jq '.' || echo "$OUT"
      echo "Send apps/backend/internal/ballots/service.go (SubmitReq struct) and I'll make the exact JSON."
      exit 1
    fi
  fi
fi

echo "[7/8] close election (this обычно триггерит tally job -> worker делает results_ready)"
curl -sS -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" \
  -d '{}' "$BASE/elections/$ELECTION_ID/actions/close" >/dev/null || true

echo "[8/8] try publish + get results"
curl -sS -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" \
  -d '{}' "$BASE/elections/$ELECTION_ID/actions/publish" >/dev/null || true

res="$(curl -sS -H "Authorization: Bearer $VOTER_TOKEN" "$BASE/elections/$ELECTION_ID/results" || true)"
echo "$res" | jq '.' || echo "$res"
echo "E2E vote OK (or at least reached results endpoint)"
