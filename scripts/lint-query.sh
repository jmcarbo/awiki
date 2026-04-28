#!/usr/bin/env bash
# scripts/lint-query.sh — Q-code query linter. Sourced by lint.sh OR run standalone.
set -uo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
RESOLVE_PY="$REPO_ROOT/scripts/lib/query-resolve.py"
DETERMINISM_PY="$REPO_ROOT/scripts/lib/query-determinism.py"
ENGINE="$REPO_ROOT/scripts/lib/query-engine.sh"

: "${FIX:=0}"

_emit_q() {
  local level="$1"
  level="$(printf '%s' "$level" | tr '[:lower:]' '[:upper:]')"
  printf 'LINT|%s|%s|%s|%s\n' "$level" "$2" "$3" "$4"
}

_query_pages() {
  find content/queries -maxdepth 1 -type f -name '*.md' 2>/dev/null
}

lint_query_one() {
  local page="$1"
  local rel="${page#$REPO_ROOT/}"
  rel="${rel#./}"

  # Q1 — must contain a ```sql fence under ## SQL.
  local sql
  sql="$(python3 - "$page" <<'PY'
import re, sys, pathlib
text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
m = re.search(r"```sql\s*\n(.*?)\n```", text, re.S)
print(m.group(1) if m else "", end="")
PY
)"
  if [[ -z "$sql" ]]; then
    _emit_q error "$rel" Q1 "no \`\`\`sql fence found"
    return 1
  fi

  # Q2 — every referenced slug must exist on disk.
  local missing=0
  while IFS= read -r slug; do
    [[ -z "$slug" ]] && continue
    if [[ ! -f "content/datasets/$slug.md" ]]; then
      _emit_q error "$rel" Q2 "unknown dataset: $slug"
      missing=1
    fi
  done < <(printf '%s' "$sql" | python3 "$RESOLVE_PY")
  return $missing
}

lint_query_all() {
  local rc=0
  while IFS= read -r page; do
    [[ -z "$page" ]] && continue
    lint_query_one "$page" || rc=1
  done < <(_query_pages)
  return $rc
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  lint_query_all
fi
