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

echo "[1/7] wait backend health"
curl -fsS http://127.0.0.1:3001/health >/dev/null

echo "[2/7] login as researcher"
RESEARCHER_TOKEN="$(auth_register_login researcher)"

ME="$(curl -sS -H "Authorization: Bearer $RESEARCHER_TOKEN" "$BASE/auth/me")"
USER_ID="$(echo "$ME" | jq -r '.id // .user.id // empty')"
if [[ -z "$USER_ID" || "$USER_ID" == "null" ]]; then
  echo "cannot get user id from /auth/me: $ME"
  exit 1
fi
echo "user_id=$USER_ID"

echo "[3/7] seed dataset into mongo (datasets + dataset_ballots)"
MONGO_OUT="$(
docker exec -i mongo-db mongosh --quiet \
  --username root --password "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin <<'JS'
const db = db.getSiblingDB("secure_voting");

const now = new Date();
const ds = {
  name: "e2e dataset",
  description: "smoke inserted by script",
  source: "generate",
  format: "ranking",
  candidates: [
    { id: "c1", name: "A" },
    { id: "c2", name: "B" },
    { id: "c3", name: "C" },
    { id: "c4", name: "D" }
  ],
  created_at: now,
  seed: 42,
  parameters: { voters: 30 }
};

const ins = db.datasets.insertOne(ds);
const datasetId = ins.insertedId;

function ballot(i) {
  const base = ["c1","c2","c3","c4"];
  // простая вариативность ранжирования
  const rot = i % base.length;
  const r = base.slice(rot).concat(base.slice(0, rot));
  return {
    dataset_id: datasetId,
    voter_ref: "v" + i,
    ranking: r
  };
}

const ballots = [];
for (let i = 1; i <= 30; i++) ballots.push(ballot(i));
db.dataset_ballots.insertMany(ballots);

print(datasetId.valueOf());
JS
)"
DATASET_ID="$(echo "$MONGO_OUT" | tail -n 1 | tr -d '\r\n')"
if [[ -z "$DATASET_ID" ]]; then
  echo "failed to seed dataset in mongo"
  exit 1
fi
echo "dataset_id=$DATASET_ID"

echo "[4/7] seed experiment + experiment_run + job into postgres"
# генерим uuid внутри postgres, чтобы не зависеть от uuidgen на хосте
PSQL_OUT="$(
docker exec -i postgres-db psql -U admin -d secure-voting -At <<SQL
WITH exp AS (
  INSERT INTO experiments (id, type, params, created_by, created_at, status, seed)
  VALUES (gen_random_uuid(), 'algo', '{}'::jsonb, '$USER_ID'::uuid, now(), 'draft', 123)
  RETURNING id
),
run AS (
  INSERT INTO experiment_runs (id, experiment_id, dataset_id, status)
  SELECT gen_random_uuid(), exp.id, '$DATASET_ID', 'queued'
  FROM exp
  RETURNING id, experiment_id
),
job AS (
  INSERT INTO jobs (id, kind, status, progress, created_by, experiment_id, experiment_run_id, payload, created_at)
  SELECT
    gen_random_uuid(),
    'experiment_run',
    'queued',
    0,
    '$USER_ID'::uuid,
    run.experiment_id,
    run.id,
    jsonb_build_object('experiment_id', run.experiment_id::text, 'dataset_id', '$DATASET_ID', 'run_id', run.id::text),
    now()
  FROM run
  RETURNING id
)
SELECT
  (SELECT id::text FROM exp) || '|' ||
  (SELECT id::text FROM run) || '|' ||
  (SELECT id::text FROM job);
SQL
)"
IFS='|' read -r EXPERIMENT_ID RUN_ID JOB_ID <<<"$PSQL_OUT"
if [[ -z "$EXPERIMENT_ID" || -z "$RUN_ID" || -z "$JOB_ID" ]]; then
  echo "failed to seed postgres: $PSQL_OUT"
  exit 1
fi
echo "experiment_id=$EXPERIMENT_ID"
echo "run_id=$RUN_ID"
echo "job_id=$JOB_ID"

echo "[5/7] poll jobs until experiment_run done/error (worker + kafka + compute-runner + grpc)"
deadline=$(( $(date +%s) + 120 ))
status=""
while [[ $(date +%s) -lt $deadline ]]; do
  j="$(curl -sS -H "Authorization: Bearer $RESEARCHER_TOKEN" "$BASE/jobs/$JOB_ID" || true)"
  status="$(echo "$j" | jq -r '.status // empty' 2>/dev/null || true)"
  prog="$(echo "$j" | jq -r '.progress // empty' 2>/dev/null || true)"
  echo "job status=$status progress=$prog"
  if [[ "$status" == "done" || "$status" == "error" ]]; then
    break
  fi
  sleep 1
done

if [[ "$status" != "done" ]]; then
  echo "job did not finish as done (status=$status). job:"
  echo "$j" | jq '.'
  echo "tips: check docker logs go-worker / go-compute-runner / rust-compute and kafka topics consumers"
  exit 1
fi

echo "[6/7] fetch experiment run result via API"
res="$(curl -sS -H "Authorization: Bearer $RESEARCHER_TOKEN" "$BASE/experiment-runs/$RUN_ID/result" || true)"
echo "$res" | jq '.'

echo "[7/7] sanity check: mongo experiment_results has run_id"
mongo_check="$(
docker exec -i mongo-db mongosh --quiet \
  --username root --password "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin <<JS
const db = db.getSiblingDB("secure_voting");
const doc = db.experiment_results.findOne({ run_id: "$RUN_ID" });
print(doc ? "ok" : "missing");
JS
)"
echo "$mongo_check" | tail -n 1

echo "E2E experiment_run OK"
