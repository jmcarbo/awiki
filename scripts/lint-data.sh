#!/usr/bin/env bash
# scripts/lint-data.sh — D-code dataset linter.
# Sourced by lint.sh OR run standalone. Exits 0 always; emits LINT|<level>|<file>|<code>|<msg>.

set -uo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
DATASETS_DIR="$REPO_ROOT/content/datasets"
ROWS_PY="$REPO_ROOT/scripts/lib/dataset-rows.py"

# shellcheck source=/dev/null
[[ -f "$REPO_ROOT/scripts/lib/dataset-fm.sh" ]] && source "$REPO_ROOT/scripts/lib/dataset-fm.sh"

_emit() { # _emit <level> <file> <code> <msg>
  printf 'LINT|%s|%s|%s|%s\n' "$1" "$2" "$3" "$4"
}

_fence_info() { # _fence_info <page>: print info-string of the fence directly under ## Data, or empty
  awk '
    /^## Data[[:space:]]*$/ { in_data=1; next }
    in_data && /^```/ {
      info=$0; sub(/^```/, "", info); sub(/[[:space:]]+$/, "", info); print info; exit
    }
  ' "$1"
}

lint_data_one() {
  local page="$1"
  local rel="${page#$REPO_ROOT/}"
  local storage format data_path
  storage="$(fm_get "$page" storage || true)"
  format="$(fm_get "$page" format || true)"
  data_path="$(fm_get "$page" data_path || true)"

  if [[ -z "$storage" ]]; then _emit error "$rel" D1 "missing storage"; return; fi
  if [[ -z "$format"  ]]; then _emit error "$rel" D1 "missing format";  return; fi

  if [[ "$storage" == "file" ]]; then
    if [[ -z "$data_path" ]]; then _emit error "$rel" D2 "storage=file but data_path missing"; return; fi
    if [[ ! -f "$REPO_ROOT/$data_path" ]]; then _emit error "$rel" D2 "data file absent: $data_path"; return; fi
  fi

  if [[ "$storage" == "inline" ]]; then
    local info
    info="$(_fence_info "$page")"
    if [[ -z "$info" ]]; then
      _emit error "$rel" D3 "storage=inline but no fenced block under ## Data"
      return
    fi
    if [[ "$info" != "$format" ]]; then
      _emit error "$rel" D4 "frontmatter format=$format != fence info=$info"
    fi
  fi
}

lint_data_all() {
  local rc=0
  if [[ ! -d "$DATASETS_DIR" ]]; then return 0; fi
  while IFS= read -r -d '' page; do
    lint_data_one "$page"
  done < <(find "$DATASETS_DIR" -type f -name '*.md' -print0)
  return $rc
}

# Run as standalone when invoked directly.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  lint_data_all
fi
