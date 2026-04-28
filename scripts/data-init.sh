#!/usr/bin/env bash
set -euo pipefail

# scripts/data-init.sh — per-step idempotent enabler for the awiki data layer.
# Mirrors scripts/task-init.sh: each step detects its own state and skips
# work that is already done.

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

note() { echo "DATA-INIT|$*"; }
warn() { echo "DATA-INIT|WARN|$*" >&2; }

step_dirs() {
  for d in content/datasets data; do
    if [[ ! -d "$d" ]]; then
      mkdir -p "$d"
      note "created $d"
    else
      note "skip $d (exists)"
    fi
  done
}

main() {
  note "start"
  step_dirs
  note "done"
}

main "$@"
