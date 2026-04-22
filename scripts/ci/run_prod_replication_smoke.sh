#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

unset COMPOSE_FILE
unset COMPOSE_PROFILES
unset COMPOSE_PROJECT_NAME

source scripts/ci/common.sh

ARTIFACTS_DIR="$(ci_artifact_dir prod-replication-smoke)"
COMPOSE_ARGS=(-f docker-compose.yml -f docker-compose.prod-replication.yml)

cleanup() {
  collect_compose_artifacts prod-replication-smoke "${COMPOSE_ARGS[@]}"
  docker compose "${COMPOSE_ARGS[@]}" down -v --remove-orphans || true
}
trap cleanup EXIT

if [[ ! -f .env && -f .env.example ]]; then
  cp .env.example .env
fi

if [[ ! -f .env ]]; then
  echo "missing .env"
  exit 1
fi

set -a
source .env
export COMPOSE_PROFILES=prod,prod-replication
set +a

echo "== generate local TLS certs =="
bash scripts/certs/generate.sh

echo "== start base infra without backend/frontend =="
docker compose "${COMPOSE_ARGS[@]}" up -d db cache mongo kafka compute

WAIT_ATTEMPTS=120 WAIT_SLEEP_SECONDS=2 wait_for_compose "${COMPOSE_ARGS[@]}"

echo "== start mongo secondary =="
docker compose "${COMPOSE_ARGS[@]}" up -d mongo-secondary

deadline=$(( $(date +%s) + 120 ))
while [[ $(date +%s) -lt $deadline ]]; do
  secondary_health="$(docker inspect mongo-db-secondary --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)"
  if [[ "$secondary_health" == "healthy" ]]; then
    break
  fi
  sleep 2
done

secondary_health="$(docker inspect mongo-db-secondary --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)"
if [[ "$secondary_health" != "healthy" ]]; then
  echo "mongo secondary did not become healthy"
  docker logs mongo-db-secondary --tail=200 || true
  exit 1
fi

echo "== start mongo replica set init =="
docker compose "${COMPOSE_ARGS[@]}" up -d mongo-rs-init-prod

echo "== wait for mongo replica set readiness =="
deadline=$(( $(date +%s) + 180 ))
mongo_primary="false"
mongo_secondary="false"

while [[ $(date +%s) -lt $deadline ]]; do
  mongo_primary="$(
    docker compose "${COMPOSE_ARGS[@]}" exec -T mongo mongosh --quiet --host localhost --port 27017 \
      --tls \
      --tlsCAFile /tmp/mongo-certs/ca.pem \
      --tlsAllowInvalidHostnames \
      --username root \
      --password "$MONGO_INITDB_ROOT_PASSWORD" \
      --authenticationDatabase admin \
      --eval 'try { print(db.hello().isWritablePrimary === true ? "true" : "false") } catch (e) { print("false") }' \
      2>/dev/null | tr -d '\r[:space:]'
  )"

  mongo_secondary="$(
    docker compose "${COMPOSE_ARGS[@]}" exec -T mongo-secondary mongosh --quiet --host localhost --port 27017 \
      --tls \
      --tlsCAFile /tmp/mongo-certs/ca.pem \
      --tlsAllowInvalidHostnames \
      --eval 'try { print(db.hello().secondary === true ? "true" : "false") } catch (e) { print("false") }' \
      2>/dev/null | tr -d '\r[:space:]'
  )"

  if [[ "$mongo_primary" == "true" && "$mongo_secondary" == "true" ]]; then
    break
  fi

  sleep 2
done

if [[ "$mongo_primary" != "true" ]]; then
  echo "mongo primary did not become ready"
  docker logs mongo-db --tail=200 || true
  docker logs mongo-rs-init-prod --tail=200 || true
  exit 1
fi

if [[ "$mongo_secondary" != "true" ]]; then
  echo "mongo secondary did not become ready"
  docker logs mongo-db-secondary --tail=200 || true
  docker logs mongo-rs-init-prod --tail=200 || true
  exit 1
fi

echo "== prepare postgres primary for replication =="
docker compose "${COMPOSE_ARGS[@]}" exec -T \
  -e POSTGRES_REPLICATION_PASSWORD="${POSTGRES_REPLICATION_PASSWORD:-replicatorpass}" \
  db sh -lc '
    set -eu

    export PGPASSWORD="$POSTGRES_PASSWORD"

    psql -U admin -d postgres <<SQL
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '\''replicator'\'') THEN
    CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD '\''$POSTGRES_REPLICATION_PASSWORD'\'';
  ELSE
    ALTER ROLE replicator WITH REPLICATION LOGIN PASSWORD '\''$POSTGRES_REPLICATION_PASSWORD'\'';
  END IF;
END
\$\$;

ALTER SYSTEM SET wal_level = '\''replica'\'';
ALTER SYSTEM SET max_wal_senders = '\''10'\'';
ALTER SYSTEM SET max_replication_slots = '\''10'\'';
ALTER SYSTEM SET hot_standby = '\''on'\'';
SQL

    HBA="${PGDATA:-/var/lib/postgresql/data}/pg_hba.conf"

    grep -q "^hostssl[[:space:]]\+replication[[:space:]]\+replicator[[:space:]]\+all[[:space:]]\+scram-sha-256$" "$HBA" || \
      echo "hostssl replication replicator all scram-sha-256" >> "$HBA"

    grep -q "^host[[:space:]]\+replication[[:space:]]\+replicator[[:space:]]\+all[[:space:]]\+scram-sha-256$" "$HBA" || \
      echo "host replication replicator all scram-sha-256" >> "$HBA"

    psql -U admin -d postgres -c "SELECT pg_reload_conf();"
  '

echo "== verify postgres primary pg_hba replication rules =="
docker compose "${COMPOSE_ARGS[@]}" exec -T db sh -lc '
  HBA="${PGDATA:-/var/lib/postgresql/data}/pg_hba.conf"
  grep -n "replication replicator" "$HBA"
'

echo "== restart postgres primary to apply wal settings =="
docker compose "${COMPOSE_ARGS[@]}" restart db

deadline=$(( $(date +%s) + 120 ))
while [[ $(date +%s) -lt $deadline ]]; do
  health="$(docker inspect postgres-db --format '{{.State.Health.Status}}' 2>/dev/null || true)"
  if [[ "$health" == "healthy" ]]; then
    break
  fi
  sleep 2
done

health="$(docker inspect postgres-db --format '{{.State.Health.Status}}' 2>/dev/null || true)"
if [[ "$health" != "healthy" ]]; then
  echo "postgres primary did not become healthy after restart"
  docker logs postgres-db --tail=200 || true
  exit 1
fi

echo "== start replica services =="
docker compose "${COMPOSE_ARGS[@]}" up -d db-replica redis-replica

deadline=$(( $(date +%s) + 120 ))
while [[ $(date +%s) -lt $deadline ]]; do
  health="$(docker inspect postgres-db-replica --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)"
  if [[ "$health" == "healthy" ]]; then
    break
  fi
  sleep 2
done

health="$(docker inspect postgres-db-replica --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)"
if [[ "$health" != "healthy" ]]; then
  echo "postgres replica did not become healthy"
  docker logs postgres-db-replica --tail=200 || true
  exit 1
fi

echo "== start backend/frontend after mongo replica set is ready =="
docker compose "${COMPOSE_ARGS[@]}" up -d backend frontend

deadline=$(( $(date +%s) + 120 ))
while [[ $(date +%s) -lt $deadline ]]; do
  backend_health="$(docker inspect go-backend --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)"
  frontend_health="$(docker inspect ts-frontend --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)"
  if [[ "$backend_health" == "healthy" && "$frontend_health" == "healthy" ]]; then
    break
  fi
  sleep 2
done

backend_health="$(docker inspect go-backend --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)"
frontend_health="$(docker inspect ts-frontend --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)"

if [[ "$backend_health" != "healthy" ]]; then
  echo "backend did not become healthy"
  docker logs go-backend --tail=200 || true
  exit 1
fi

if [[ "$frontend_health" != "healthy" ]]; then
  echo "frontend did not become healthy"
  docker logs ts-frontend --tail=200 || true
  exit 1
fi

echo "== verify base application health =="
curl -fsS http://127.0.0.1:3001/health > /dev/null

echo "== verify postgres replica =="
docker compose "${COMPOSE_ARGS[@]}" exec -T db sh -lc '
  export PGPASSWORD="$POSTGRES_PASSWORD"
  psql -U admin -d secure-voting -c "SELECT 1;" >/dev/null
'
docker compose "${COMPOSE_ARGS[@]}" exec -T db-replica sh -lc '
  export PGPASSWORD="$POSTGRES_PASSWORD"
  psql -U admin -d postgres -c "SELECT pg_is_in_recovery()::text;" -t -A
' | tr -d "[:space:]" | grep -q '^true$'

echo "== verify redis replica =="
docker compose "${COMPOSE_ARGS[@]}" exec -T cache redis-cli --tls --cacert /certs/ca.pem -h 127.0.0.1 -p 6380 -a "$REDIS_PASSWORD" ping | grep -q PONG
docker compose "${COMPOSE_ARGS[@]}" exec -T redis-replica redis-cli --tls --cacert /certs/ca.pem -h 127.0.0.1 -p 6380 INFO replication | tr -d '\r' | grep -q '^role:slave$'

echo "== verify mongo replica set =="
docker compose "${COMPOSE_ARGS[@]}" exec -T mongo mongosh --quiet --host localhost --port 27017 \
  --tls \
  --tlsCAFile /tmp/mongo-certs/ca.pem \
  --tlsAllowInvalidHostnames \
  --username root \
  --password "$MONGO_INITDB_ROOT_PASSWORD" \
  --authenticationDatabase admin \
  --eval 'db.hello().isWritablePrimary' | tr -d '\r[:space:]' | grep -q true

docker compose "${COMPOSE_ARGS[@]}" exec -T mongo-secondary mongosh --quiet --host localhost --port 27017 \
  --tls \
  --tlsCAFile /tmp/mongo-certs/ca.pem \
  --tlsAllowInvalidHostnames \
  --eval 'db.hello().secondary' | tr -d '\r[:space:]' | grep -q true

echo
echo "PROD REPLICATION SMOKE: PASS"