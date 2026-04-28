#!/usr/bin/env bash
set -euo pipefail

# scripts/data-init.sh — per-step idempotent enabler for the awiki data layer.
# Mirrors scripts/task-init.sh: each step detects its own state and skips
# work that is already done.

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

note() { echo "DATA-INIT|$*"; }
warn() { echo "DATA-INIT|WARN|$*" >&2; }

CONFIG_FILE=".awiki/config"

step_config() {
  mkdir -p .awiki
  if [[ ! -f "$CONFIG_FILE" ]]; then
    : > "$CONFIG_FILE"
    note "created $CONFIG_FILE"
  fi
  _ensure_kv "AWIKI_DATA_LAYER" "on"
  _ensure_kv "AWIKI_DATASET_INLINE_MAX_ROWS" "500"
  _ensure_kv "AWIKI_DATASET_INLINE_MAX_BYTES" "51200"
}

_ensure_kv() {
  local key="$1" default="$2"
  if grep -q "^${key}=" "$CONFIG_FILE"; then
    note "skip ${key} (already set)"
  else
    printf -- '%s=%s\n' "$key" "$default" >> "$CONFIG_FILE"
    note "appended ${key}=${default}"
  fi
}

step_dirs() {
  for d in content/datasets data; do
    if [[ ! -d "$d" ]]; then
      mkdir -p "$d"
      touch "$d/.gitkeep"
      note "created $d"
    else
      note "skip $d (exists)"
    fi
  done
}

main() {
  note "start"
  step_dirs
  step_config
  note "done"
}

main "$@"
