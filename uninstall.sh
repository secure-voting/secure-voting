#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

YES=0
NO_BACKUP=0
IGNORE_BACKUP_ERRORS=0
REMOVE_ENV=0
REMOVE_BACKUPS=0
REMOVE_IMAGES=0
REMOVE_BUILD_CACHE=0

usage() {
  cat <<'USAGE'
Usage:
  bash uninstall.sh --yes [options]

Options:
  --yes                   Required confirmation for destructive uninstall.
  --no-backup             Do not create a backup before deleting containers and volumes.
  --ignore-backup-errors  Continue uninstall even if backup fails.
  --remove-env            Remove local .env.
  --remove-backups        Remove local .backups directory.
  --remove-images         Remove local images built by Docker Compose for this project.
  --remove-build-cache    Prune Docker build cache after uninstall.
  -h, --help              Show this help.

Examples:
  bash scripts/ops/uninstall.sh --yes
  bash scripts/ops/uninstall.sh --yes --no-backup
  bash scripts/ops/uninstall.sh --yes --no-backup --remove-images --remove-env
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
  printf '\nUNINSTALL FAILED, exit code: %s\n' "$exit_code" >&2
  printf 'Useful diagnostics:\n' >&2
  printf '  docker ps -a\n' >&2
  printf '  docker network ls\n' >&2
  printf '  docker volume ls\n' >&2
  exit "$exit_code"
}
trap on_error ERR

while [[ $# -gt 0 ]]; do
  case "$1" in
    --yes)
      YES=1
      shift
      ;;
    --no-backup)
      NO_BACKUP=1
      shift
      ;;
    --ignore-backup-errors)
      IGNORE_BACKUP_ERRORS=1
      shift
      ;;
    --remove-env)
      REMOVE_ENV=1
      shift
      ;;
    --remove-backups)
      REMOVE_BACKUPS=1
      shift
      ;;
    --remove-images)
      REMOVE_IMAGES=1
      shift
      ;;
    --remove-build-cache)
      REMOVE_BUILD_CACHE=1
      shift
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

[[ "$YES" -eq 1 ]] || fail "destructive uninstall requires --yes"

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

require_docker() {
  require_command docker
  docker compose version >/dev/null 2>&1 || fail "Docker Compose v2 is required: docker compose version failed"
  docker info >/dev/null 2>&1 || fail "Docker daemon is not available. Start Docker and retry."
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

container_running() {
  local name="$1"
  [[ "$(docker inspect -f '{{.State.Running}}' "$name" 2>/dev/null || echo false)" == "true" ]]
}

create_backup_if_possible() {
  if [[ "$NO_BACKUP" -eq 1 ]]; then
    log "skip backup"
    return 0
  fi

  if [[ ! -x scripts/ops/backup_all.sh && ! -f scripts/ops/backup_all.sh ]]; then
    warn "backup script not found, skipping backup"
    return 0
  fi

  if ! container_running postgres-db || ! container_running mongo-db; then
    warn "postgres-db or mongo-db is not running, skipping backup"
    return 0
  fi

  log "create backup before uninstall"
  mkdir -p .backups
  if bash scripts/ops/backup_all.sh "$ROOT_DIR/.backups"; then
    return 0
  fi

  if [[ "$IGNORE_BACKUP_ERRORS" -eq 1 ]]; then
    warn "backup failed, continuing because --ignore-backup-errors was set"
    return 0
  fi

  fail "backup failed. Re-run with --no-backup only if data loss is acceptable."
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
  log "remove containers"
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
  for attempt in 1 2 3 4 5; do
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
  log "remove networks"
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
  log "remove volumes"
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

remove_images_if_requested() {
  if [[ "$REMOVE_IMAGES" -ne 1 ]]; then
    return 0
  fi

  log "remove local project images"
  local p id image
  while IFS= read -r p; do
    [[ -n "$p" ]] || continue
    while IFS= read -r id; do
      [[ -n "$id" ]] || continue
      docker image rm -f "$id" >/dev/null 2>&1 || true
    done < <(docker images -q --filter "label=com.docker.compose.project=$p" 2>/dev/null || true)

    for image in \
      "${p}-frontend" \
      "${p}-backend" \
      "${p}-compute" \
      "${p}_frontend" \
      "${p}_backend" \
      "${p}_compute"; do
      docker image rm -f "$image" >/dev/null 2>&1 || true
    done
  done < <(candidate_project_names)
}

remove_generated_files() {
  log "remove generated files"
  rm -rf scripts/certs/out
  rm -rf .ci-artifacts

  if [[ "$REMOVE_ENV" -eq 1 ]]; then
    rm -f .env
  fi

  if [[ "$REMOVE_BACKUPS" -eq 1 ]]; then
    rm -rf .backups
  fi
}

prune_build_cache_if_requested() {
  if [[ "$REMOVE_BUILD_CACHE" -eq 1 ]]; then
    log "prune Docker build cache"
    docker builder prune -f || true
  fi
}

print_remaining_diagnostics() {
  log "remaining secure-voting docker objects"
  printf 'Containers:\n'
  docker ps -a --format '{{.Names}}' | grep -E '^(ts-frontend|go-backend|go-worker|postgres-db|postgres-webui|redis-cache|redis-webui|mongo-db|mongodb-webui|rust-compute|go-compute-runner|kafka|kafka-ui|kafka-init)$' || true

  printf '\nNetworks:\n'
  docker network ls --format '{{.Name}}' | grep -E '(secure-voting|secure_voting|app_network|db_network|rpc_network|redis_network|kafka_network|debug_network)' || true

  printf '\nVolumes:\n'
  docker volume ls --format '{{.Name}}' | grep -E '(secure-voting|secure_voting|db-data|redis-data|mongo-data|kafka-data|pgadmin-data|redisinsight-data)' || true
}

main() {
  require_docker

  if [[ ! -f docker-compose.yml ]]; then
    fail "docker-compose.yml not found. Run this script from inside the secure-voting repository."
  fi

  create_backup_if_possible

  log "compose down"
  compose_down_default
  local p
  while IFS= read -r p; do
    [[ -n "$p" ]] || continue
    compose_down_for_project "$p"
  done < <(candidate_project_names)

  remove_known_containers
  remove_compose_networks
  remove_compose_volumes
  remove_images_if_requested
  remove_generated_files
  prune_build_cache_if_requested
  print_remaining_diagnostics

  log "uninstall completed"
  if [[ "$REMOVE_BACKUPS" -ne 1 ]]; then
    printf 'Backups were preserved in .backups if they existed or were created.\n'
  fi
  if [[ "$REMOVE_ENV" -ne 1 ]]; then
    printf '.env was preserved. Use --remove-env to delete it.\n'
  fi
}

main "$@"