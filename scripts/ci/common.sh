#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

ci_artifact_dir() {
  local kind="$1"
  local dir="$ROOT_DIR/.ci-artifacts/$kind"
  mkdir -p "$dir"
  printf '%s\n' "$dir"
}

wait_for_compose() {
  local compose_args=("$@")
  local attempts="${WAIT_ATTEMPTS:-90}"
  local sleep_seconds="${WAIT_SLEEP_SECONDS:-5}"
  local service_names
  mapfile -t service_names < <(docker compose "${compose_args[@]}" ps --services)

  if [[ "${#service_names[@]}" -eq 0 ]]; then
    echo "No compose services found for args: ${compose_args[*]}" >&2
    return 1
  fi

  for ((i=1; i<=attempts; i++)); do
    local all_ready=1
    for service in "${service_names[@]}"; do
      local cid state health
      cid="$(docker compose "${compose_args[@]}" ps -q "$service")"
      if [[ -z "$cid" ]]; then
        all_ready=0
        break
      fi
      state="$(docker inspect -f '{{.State.Status}}' "$cid" 2>/dev/null || true)"
      health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$cid" 2>/dev/null || true)"
      if [[ "$state" != "running" ]]; then
        all_ready=0
        break
      fi
      if [[ -n "$health" && "$health" != "healthy" ]]; then
        all_ready=0
        break
      fi
    done
    if [[ "$all_ready" -eq 1 ]]; then
      return 0
    fi
    sleep "$sleep_seconds"
  done

  docker compose "${compose_args[@]}" ps || true
  return 1
}

collect_compose_artifacts() {
  local kind="$1"
  shift
  local compose_args=("$@")
  local out_dir
  out_dir="$(ci_artifact_dir "$kind")"

  docker compose "${compose_args[@]}" ps > "$out_dir/compose-ps.txt" 2>&1 || true
  docker compose "${compose_args[@]}" logs --no-color > "$out_dir/compose-logs.txt" 2>&1 || true
}
