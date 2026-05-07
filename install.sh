#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

FRESH=0
RESET_ENV=0
WITH_DEBUG=0
PRUNE_BUILD_CACHE=0
SKIP_BUILD=0
WAIT_TIMEOUT_SECONDS=420

usage() {
  cat <<'USAGE'
Usage:
  bash scripts/ops/install.sh [options]

Options:
  --fresh              Stop and remove old containers, networks, volumes and generated certs before install.
  --reset-env          Recreate .env from .env.example and generate fresh local secrets.
  --with-debug         Start debug web UIs in addition to the production stack.
  --prune-build-cache  Prune Docker build cache before building. Use this after broken or interrupted builds.
  --skip-build         Start existing images without rebuilding them.
  --wait-timeout N     Healthcheck timeout in seconds. Default: 420.
  -h, --help           Show this help.

Examples:
  bash scripts/ops/install.sh --fresh --reset-env
  bash scripts/ops/install.sh --fresh --reset-env --with-debug
  bash scripts/ops/install.sh --fresh --reset-env --prune-build-cache
USAGE
}

log() {
  printf '\n== %s ==\n' "$*"
}

warn() {
  printf 'WARN: %s\n' "$*" >&2
}

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

on_error() {
  local exit_code="$?"
  printf '\nINSTALL FAILED, exit code: %s\n' "$exit_code" >&2
  printf 'Useful diagnostics:\n' >&2
  printf '  docker compose ps\n' >&2
  printf '  docker compose logs --tail=200 backend\n' >&2
  printf '  docker compose logs --tail=200 db mongo cache kafka\n' >&2
  printf '  docker system df\n' >&2
  printf '\nCommon recovery commands:\n' >&2
  printf '  bash scripts/ops/uninstall.sh --yes --no-backup\n' >&2
  printf '  bash scripts/ops/install.sh --fresh --reset-env --prune-build-cache\n' >&2
  exit "$exit_code"
}
trap on_error ERR

while [[ $# -gt 0 ]]; do
  case "$1" in
    --fresh)
      FRESH=1
      shift
      ;;
    --reset-env)
      RESET_ENV=1
      shift
      ;;
    --with-debug)
      WITH_DEBUG=1
      shift
      ;;
    --prune-build-cache)
      PRUNE_BUILD_CACHE=1
      shift
      ;;
    --skip-build)
      SKIP_BUILD=1
      shift
      ;;
    --wait-timeout)
      [[ $# -ge 2 ]] || fail "--wait-timeout requires a value"
      WAIT_TIMEOUT_SECONDS="$2"
      [[ "$WAIT_TIMEOUT_SECONDS" =~ ^[0-9]+$ ]] || fail "--wait-timeout must be a positive integer"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown option: $1"
      ;;
  esac
done

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

require_docker() {
  require_command docker
  docker compose version >/dev/null 2>&1 || fail "Docker Compose v2 is required: docker compose version failed"
  docker info >/dev/null 2>&1 || fail "Docker daemon is not available. Start Docker and retry."
}

random_secret() {
  openssl rand -hex 32
}

get_env_value() {
  local key="$1"
  if [[ ! -f .env ]]; then
    return 0
  fi
  awk -F= -v k="$key" '$1 == k {sub(/^[^=]*=/, ""); print; exit}' .env | sed 's/^"//; s/"$//'
}

upsert_env() {
  local key="$1"
  local value="$2"
  local tmp
  tmp="$(mktemp)"

  if [[ -f .env ]] && grep -qE "^${key}=" .env; then
    awk -v k="$key" -v v="$value" '
      BEGIN { done = 0 }
      index($0, k "=") == 1 { print k "=" v; done = 1; next }
      { print }
      END { if (done == 0) print k "=" v }
    ' .env > "$tmp"
  else
    [[ -f .env ]] && cat .env > "$tmp"
    if [[ -s "$tmp" ]] && [[ "$(tail -c 1 "$tmp")" != "" ]]; then
      printf '\n' >> "$tmp"
    fi
    printf '%s=%s\n' "$key" "$value" >> "$tmp"
  fi

  mv "$tmp" .env
}

ensure_env_value() {
  local key="$1"
  local default_value="$2"
  local current
  current="$(get_env_value "$key")"
  if [[ -z "$current" ]]; then
    upsert_env "$key" "$default_value"
  fi
}

ensure_secret_env() {
  local key="$1"
  shift
  local current
  current="$(get_env_value "$key")"

  if [[ -z "$current" ]]; then
    upsert_env "$key" "$(random_secret)"
    return
  fi

  local insecure
  for insecure in "$@"; do
    if [[ "$current" == "$insecure" ]]; then
      upsert_env "$key" "$(random_secret)"
      return
    fi
  done
}

prepare_env() {
  log "prepare .env"

  if [[ "$RESET_ENV" -eq 1 ]]; then
    rm -f .env
  fi

  if [[ ! -f .env ]]; then
    if [[ -f .env.example ]]; then
      cp .env.example .env
    else
      cat > .env <<'ENVEOF'
POSTGRES_PASSWORD=
REDIS_PASSWORD=
MONGO_INITDB_ROOT_PASSWORD=
BOOTSTRAP_ADMIN_EMAIL=admin@example.com
BOOTSTRAP_ADMIN_PASSWORD=
BOOTSTRAP_RESEARCHER_EMAIL=researcher@example.com
BOOTSTRAP_RESEARCHER_PASSWORD=
WRITE_RATE_LIMIT=30
WRITE_RATE_LIMIT_TTL=1m
AUTH_RATE_LIMIT=10
AUTH_RATE_LIMIT_TTL=1m
ADMIN_TRUSTED_CIDRS=
COMPUTE_GRPC_ADDR=rust-compute:50051
COMPUTE_TLS=true
COMPUTE_TLS_CA=/certs/ca.pem
COMPUTE_TLS_SERVER_NAME=rust-compute
FRONTEND_TLS_HOSTS=localhost,127.0.0.1,ts-frontend
EMAIL_VERIFICATION_MODE=dev
SMTP_HOST=
SMTP_PORT=587
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_FROM_EMAIL=
SMTP_FROM_NAME="Secure Voting"
SMTP_TLS_MODE=starttls
ENVEOF
    fi
  fi

  ensure_secret_env POSTGRES_PASSWORD postgres_dev_pass postgres postgrespass password
  ensure_secret_env REDIS_PASSWORD redis_dev_pass redis redispass password
  ensure_secret_env MONGO_INITDB_ROOT_PASSWORD mongo_dev_pass mongo mongopass password
  ensure_secret_env BOOTSTRAP_ADMIN_PASSWORD 'AdminPass123!' admin adminpass password
  ensure_secret_env BOOTSTRAP_RESEARCHER_PASSWORD 'ResearchPass123!' 'ResearcherPass123!' researcher researcherpass password

  ensure_env_value BOOTSTRAP_ADMIN_EMAIL admin@example.com
  ensure_env_value BOOTSTRAP_RESEARCHER_EMAIL researcher@example.com
  ensure_env_value WRITE_RATE_LIMIT 30
  ensure_env_value WRITE_RATE_LIMIT_TTL 1m
  ensure_env_value AUTH_RATE_LIMIT 10
  ensure_env_value AUTH_RATE_LIMIT_TTL 1m
  ensure_env_value ADMIN_TRUSTED_CIDRS ""
  ensure_env_value COMPUTE_GRPC_ADDR rust-compute:50051
  ensure_env_value COMPUTE_TLS true
  ensure_env_value COMPUTE_TLS_CA /certs/ca.pem
  ensure_env_value COMPUTE_TLS_SERVER_NAME rust-compute
  ensure_env_value FRONTEND_TLS_HOSTS localhost,127.0.0.1,ts-frontend
  ensure_env_value EMAIL_VERIFICATION_MODE dev
  ensure_env_value SMTP_HOST ""
  ensure_env_value SMTP_PORT 587
  ensure_env_value SMTP_USERNAME ""
  ensure_env_value SMTP_PASSWORD ""
  ensure_env_value SMTP_FROM_EMAIL ""
  ensure_env_value SMTP_FROM_NAME '"Secure Voting"'
  ensure_env_value SMTP_TLS_MODE starttls

  chmod 600 .env || true
}

source_env() {
  set -a
  source .env
  set +a
}

known_container_names() {
  cat <<'EOFNAMES'
ts-frontend
go-backend
go-worker
postgres-db
postgres-webui
redis-cache
redis-webui
mongo-db
mongodb-webui
rust-compute
go-compute-runner
kafka
kafka-ui
kafka-init
EOFNAMES
}

candidate_project_names() {
  local base
  base="$(basename "$ROOT_DIR")"
  {
    printf '%s\n' "$base"
    printf '%s\n' "${base//-/_}"
    printf '%s\n' secure-voting
    printf '%s\n' secure_voting
    docker compose ls --format json 2>/dev/null | sed -n 's/.*"Name":"\([^"]*\)".*/\1/p' || true
    while IFS= read -r c; do
      docker inspect -f '{{ index .Config.Labels "com.docker.compose.project" }}' "$c" 2>/dev/null || true
    done < <(known_container_names)
  } | awk 'NF && !seen[$0]++'
}

compose_down_for_project() {
  local project="$1"
  COMPOSE_FILE="$ROOT_DIR/docker-compose.yml" docker compose --project-name "$project" --profile prod --profile debug down -v --remove-orphans --timeout 20 >/dev/null 2>&1 || true
}

compose_down_default() {
  unset COMPOSE_PROJECT_NAME
  unset COMPOSE_PROFILES
  COMPOSE_FILE="$ROOT_DIR/docker-compose.yml" docker compose --profile prod --profile debug down -v --remove-orphans --timeout 20 >/dev/null 2>&1 || true
}

remove_known_containers() {
  log "remove stale containers"
  local c
  while IFS= read -r c; do
    [[ -n "$c" ]] || continue
    if docker inspect "$c" >/dev/null 2>&1; then
      docker rm -f "$c" >/dev/null 2>&1 || true
    fi
  done < <(known_container_names)

  local p id
  while IFS= read -r p; do
    [[ -n "$p" ]] || continue
    while IFS= read -r id; do
      [[ -n "$id" ]] || continue
      docker rm -f "$id" >/dev/null 2>&1 || true
    done < <(docker ps -aq --filter "label=com.docker.compose.project=$p" 2>/dev/null || true)
  done < <(candidate_project_names)
}

remove_network_force() {
  local network="$1"
  docker network inspect "$network" >/dev/null 2>&1 || return 0

  local endpoint
  while IFS= read -r endpoint; do
    [[ -n "$endpoint" ]] || continue
    docker network disconnect -f "$network" "$endpoint" >/dev/null 2>&1 || true
  done < <(docker network inspect "$network" --format '{{range $id, $v := .Containers}}{{println $id}}{{end}}' 2>/dev/null || true)

  local attempt
  for attempt in 1 2 3; do
    if docker network rm "$network" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    while IFS= read -r endpoint; do
      [[ -n "$endpoint" ]] || continue
      docker network disconnect -f "$network" "$endpoint" >/dev/null 2>&1 || true
    done < <(docker network inspect "$network" --format '{{range $id, $v := .Containers}}{{println $id}}{{end}}' 2>/dev/null || true)
  done

  warn "network is still in use and was not removed: $network"
  docker network inspect "$network" --format '{{json .Containers}}' 2>/dev/null || true
}

remove_compose_networks() {
  log "remove stale networks"
  local p suffix id
  while IFS= read -r p; do
    [[ -n "$p" ]] || continue
    for suffix in app_network db_network rpc_network redis_network kafka_network debug_network; do
      remove_network_force "${p}_${suffix}"
      remove_network_force "${p}-${suffix}"
    done

    while IFS= read -r id; do
      [[ -n "$id" ]] || continue
      remove_network_force "$id"
    done < <(docker network ls -q --filter "label=com.docker.compose.project=$p" 2>/dev/null || true)
  done < <(candidate_project_names)
}

remove_compose_volumes() {
  log "remove stale volumes"
  local p suffix vol id
  while IFS= read -r p; do
    [[ -n "$p" ]] || continue
    for suffix in db-data redis-data mongo-data kafka-data pgadmin-data redisinsight-data; do
      vol="${p}_${suffix}"
      docker volume rm -f "$vol" >/dev/null 2>&1 || true
      vol="${p}-${suffix}"
      docker volume rm -f "$vol" >/dev/null 2>&1 || true
    done

    while IFS= read -r id; do
      [[ -n "$id" ]] || continue
      docker volume rm -f "$id" >/dev/null 2>&1 || true
    done < <(docker volume ls -q --filter "label=com.docker.compose.project=$p" 2>/dev/null || true)
  done < <(candidate_project_names)

  rm -rf db-data cache mongo-data iggy >/dev/null 2>&1 || true
}

remove_generated_runtime_files() {
  log "remove generated local runtime files"
  rm -rf scripts/certs/out
  rm -rf .ci-artifacts
}

cleanup_for_fresh_install() {
  log "fresh cleanup"
  compose_down_default
  local p
  while IFS= read -r p; do
    [[ -n "$p" ]] || continue
    compose_down_for_project "$p"
  done < <(candidate_project_names)
  remove_known_containers
  remove_compose_networks
  remove_compose_volumes
  remove_generated_runtime_files
}

ensure_script_permissions() {
  log "ensure scripts are executable"
  find scripts -type f -name '*.sh' -exec chmod +x {} +
}

generate_certs() {
  log "generate fresh TLS certificates"
  rm -rf scripts/certs/out
  mkdir -p scripts/certs/out
  source_env
  bash scripts/certs/generate.sh

  local cert
  for cert in \
    scripts/certs/out/ca.pem \
    scripts/certs/out/frontend.pem \
    scripts/certs/out/db.pem \
    scripts/certs/out/redis.pem \
    scripts/certs/out/mongo.pem \
    scripts/certs/out/kafka.pem \
    scripts/certs/out/compute.pem; do
    [[ -f "$cert" ]] || fail "certificate was not generated: $cert"
    openssl x509 -checkend 2592000 -noout -in "$cert" >/dev/null || fail "certificate expires too soon or is invalid: $cert"
  done
}

prune_build_cache_if_requested() {
  if [[ "$PRUNE_BUILD_CACHE" -eq 1 ]]; then
    log "prune Docker build cache"
    docker builder prune -f || true
  fi
}

compose_up() {
  log "build and start stack"
  unset COMPOSE_PROJECT_NAME
  unset COMPOSE_PROFILES
  export COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"

  local -a cmd=(docker compose --profile prod)
  if [[ "$WITH_DEBUG" -eq 1 ]]; then
    cmd+=(--profile debug)
  fi

  if [[ "$SKIP_BUILD" -eq 1 ]]; then
    cmd+=(up -d --remove-orphans)
  else
    cmd+=(up -d --build --remove-orphans)
  fi

  local attempt
  for attempt in 1 2; do
    if "${cmd[@]}"; then
      return 0
    fi

    warn "docker compose up failed, attempt $attempt"
    docker compose ps || true
    if [[ "$attempt" -eq 1 ]]; then
      warn "cleaning stale containers and networks before retry"
      remove_known_containers
      remove_compose_networks
      sleep 2
    fi
  done

  fail "docker compose up failed after retry"
}

container_exists() {
  docker inspect "$1" >/dev/null 2>&1
}

print_container_logs_tail() {
  local name="$1"
  if container_exists "$name"; then
    printf '\n--- logs: %s ---\n' "$name" >&2
    docker logs --tail=120 "$name" >&2 || true
  fi
}

wait_container_healthy_or_running() {
  local name="$1"
  local deadline status health
  deadline=$((SECONDS + WAIT_TIMEOUT_SECONDS))

  while (( SECONDS < deadline )); do
    if ! container_exists "$name"; then
      sleep 2
      continue
    fi

    status="$(docker inspect -f '{{.State.Status}}' "$name" 2>/dev/null || true)"
    health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$name" 2>/dev/null || true)"

    if [[ "$health" == "healthy" ]]; then
      printf 'OK: %s is healthy\n' "$name"
      return 0
    fi

    if [[ "$health" == "none" && "$status" == "running" ]]; then
      printf 'OK: %s is running\n' "$name"
      return 0
    fi

    if [[ "$status" == "exited" || "$status" == "dead" ]]; then
      print_container_logs_tail "$name"
      fail "$name exited before becoming ready"
    fi

    sleep 2
  done

  print_container_logs_tail "$name"
  fail "timeout waiting for container: $name"
}

wait_kafka_init() {
  local deadline status exit_code
  deadline=$((SECONDS + WAIT_TIMEOUT_SECONDS))

  while (( SECONDS < deadline )); do
    if ! container_exists kafka-init; then
      sleep 2
      continue
    fi
    status="$(docker inspect -f '{{.State.Status}}' kafka-init 2>/dev/null || true)"
    exit_code="$(docker inspect -f '{{.State.ExitCode}}' kafka-init 2>/dev/null || true)"
    if [[ "$status" == "exited" && "$exit_code" == "0" ]]; then
      printf 'OK: kafka-init completed\n'
      return 0
    fi
    if [[ "$status" == "exited" && "$exit_code" != "0" ]]; then
      print_container_logs_tail kafka-init
      fail "kafka-init failed"
    fi
    sleep 2
  done

  print_container_logs_tail kafka-init
  fail "timeout waiting for kafka-init"
}

wait_stack() {
  log "wait for services"
  wait_container_healthy_or_running postgres-db
  wait_container_healthy_or_running redis-cache
  wait_container_healthy_or_running mongo-db
  wait_container_healthy_or_running kafka
  wait_kafka_init
  wait_container_healthy_or_running rust-compute
  wait_container_healthy_or_running go-backend
  wait_container_healthy_or_running go-worker
  wait_container_healthy_or_running go-compute-runner
  wait_container_healthy_or_running ts-frontend
}

print_summary() {
  log "installation completed"
  docker compose --profile prod --profile debug ps || true
  printf '\nFrontend: https://127.0.0.1:8080\n'
  printf 'Backend health: http://127.0.0.1:3001/health\n'
  printf 'Admin email: %s\n' "$(get_env_value BOOTSTRAP_ADMIN_EMAIL)"
  printf 'Researcher email: %s\n' "$(get_env_value BOOTSTRAP_RESEARCHER_EMAIL)"
  printf '\nPasswords are stored only in local .env. Keep this file private.\n'
}

main() {
  require_command openssl
  require_docker

  if [[ ! -f docker-compose.yml ]]; then
    fail "docker-compose.yml not found. Run this script from inside the secure-voting repository."
  fi

  if [[ "$FRESH" -eq 1 ]]; then
    cleanup_for_fresh_install
  fi

  prepare_env
  ensure_script_permissions
  generate_certs
  prune_build_cache_if_requested
  compose_up
  wait_stack
  print_summary
}

main "$@"