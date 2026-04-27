#!/usr/bin/env bash
set -euo pipefail

# scripts/task-init.sh — per-step idempotent enabler for the awiki task layer.
# Each step detects its own existing state and skips work that is already done,
# so partial-failure re-runs are safe. There is no binary "already enabled"
# gate.

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

CONFIG_FILE=".awiki/config"
LAST_REVIEW=".awiki/last-review"
TASK_COUNT=".awiki/task-count"
WIKI_MD="WIKI.md"
TEMPLATE="scripts/templates/wiki-task-layer.md"

note() { echo "TASK-INIT|$*"; }
warn() { echo "TASK-INIT|WARN|$*" >&2; }

step_state_files() {
  mkdir -p .awiki
  if [[ ! -f "$LAST_REVIEW" ]]; then
    date '+%Y-%m-%d' > "$LAST_REVIEW"
    note "created $LAST_REVIEW"
  else
    note "skip $LAST_REVIEW (exists)"
  fi
  if [[ ! -f "$TASK_COUNT" ]]; then
    echo 0 > "$TASK_COUNT"
    note "created $TASK_COUNT"
  else
    note "skip $TASK_COUNT (exists)"
  fi
}

step_config() {
  if [[ ! -f "$CONFIG_FILE" ]]; then
    mkdir -p .awiki
    : > "$CONFIG_FILE"
  fi
  if ! grep -q '^AWIKI_AGENDA_AFTER_N=' "$CONFIG_FILE"; then
    printf -- 'AWIKI_AGENDA_AFTER_N=5\n' >> "$CONFIG_FILE"
    note "appended AWIKI_AGENDA_AFTER_N=5"
  else
    note "skip AWIKI_AGENDA_AFTER_N (already set)"
  fi
  if ! grep -q '^AWIKI_TASK_LAYER=' "$CONFIG_FILE"; then
    printf -- 'AWIKI_TASK_LAYER=on\n' >> "$CONFIG_FILE"
    note "appended AWIKI_TASK_LAYER=on"
  else
    note "skip AWIKI_TASK_LAYER (already set)"
  fi
}

main() {
  note "start"
  step_config
  step_state_files
  note "done"
}

main "$@"
