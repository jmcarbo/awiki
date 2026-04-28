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

WIKI_MD="WIKI.md"
TEMPLATE="scripts/templates/wiki-data-layer.md"

step_wiki_md() {
  if [[ ! -f "$TEMPLATE" ]]; then
    warn "missing $TEMPLATE — cannot patch WIKI.md"
    return 1
  fi
  if [[ ! -f "$WIKI_MD" ]]; then
    warn "no $WIKI_MD found — skipping patch"
    return 0
  fi

  local begin='<!-- BEGIN data-layer -->'
  local end='<!-- END data-layer -->'
  if grep -qF "$begin" "$WIKI_MD"; then
    # Replace the existing block in-place (idempotent + refresh).
    awk -v begin="$begin" -v end="$end" -v template_file="$TEMPLATE" '
      BEGIN { in_block=0; while ((getline line < template_file) > 0) tmpl = tmpl line "\n" }
      index($0, begin) { print tmpl; in_block=1; next }
      in_block && index($0, end) { in_block=0; next }
      !in_block { print }
    ' "$WIKI_MD" > "$WIKI_MD.tmp"
    mv "$WIKI_MD.tmp" "$WIKI_MD"
    note "refreshed data-layer block in $WIKI_MD"
  else
    printf -- '\n' >> "$WIKI_MD"
    cat "$TEMPLATE" >> "$WIKI_MD"
    note "appended data-layer block to $WIKI_MD"
  fi
}

main() {
  note "start"
  step_dirs
  step_wiki_md
  step_config
  note "done"
}

main "$@"
