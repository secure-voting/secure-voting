#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

BACKUP_ROOT="${1:-$ROOT_DIR/.backups}"
KEEP_DAILY="${KEEP_DAILY:-7}"
KEEP_WEEKLY="${KEEP_WEEKLY:-4}"
KEEP_MONTHLY="${KEEP_MONTHLY:-6}"

mkdir -p "$BACKUP_ROOT"

mapfile -t BACKUP_DIRS < <(find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d | sort)

if [[ "${#BACKUP_DIRS[@]}" -eq 0 ]]; then
  echo "no backups to prune"
  exit 0
fi

declare -A KEEP_MAP=()

# keep latest N daily
daily_count=0
for dir in $(printf '%s\n' "${BACKUP_DIRS[@]}" | sort -r); do
  stamp="$(basename "$dir")"
  day="${stamp:0:8}"
  if [[ -z "${KEEP_MAP["daily:$day"]+x}" ]]; then
    KEEP_MAP["daily:$day"]="$dir"
    KEEP_MAP["dir:$dir"]=1
    daily_count=$((daily_count + 1))
    if (( daily_count >= KEEP_DAILY )); then
      break
    fi
  fi
done

# keep latest N weekly
weekly_count=0
for dir in $(printf '%s\n' "${BACKUP_DIRS[@]}" | sort -r); do
  stamp="$(basename "$dir")"
  year="${stamp:0:4}"
  month="${stamp:4:2}"
  day="${stamp:6:2}"
  week="$(date -u -d "${year}-${month}-${day}" +%G-%V)"
  if [[ -z "${KEEP_MAP["week:$week"]+x}" ]]; then
    KEEP_MAP["week:$week"]="$dir"
    KEEP_MAP["dir:$dir"]=1
    weekly_count=$((weekly_count + 1))
    if (( weekly_count >= KEEP_WEEKLY )); then
      break
    fi
  fi
done

# keep latest N monthly
monthly_count=0
for dir in $(printf '%s\n' "${BACKUP_DIRS[@]}" | sort -r); do
  stamp="$(basename "$dir")"
  ym="${stamp:0:6}"
  if [[ -z "${KEEP_MAP["month:$ym"]+x}" ]]; then
    KEEP_MAP["month:$ym"]="$dir"
    KEEP_MAP["dir:$dir"]=1
    monthly_count=$((monthly_count + 1))
    if (( monthly_count >= KEEP_MONTHLY )); then
      break
    fi
  fi
done

deleted=0
for dir in "${BACKUP_DIRS[@]}"; do
  if [[ -z "${KEEP_MAP["dir:$dir"]+x}" ]]; then
    rm -rf "$dir"
    echo "deleted old backup: $dir"
    deleted=$((deleted + 1))
  fi
done

echo "prune completed, deleted=$deleted"