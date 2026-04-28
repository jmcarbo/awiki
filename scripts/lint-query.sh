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

  # Q3 — determinism (only on materialized type:query pages).
  if printf '%s' "$sql" | python3 "$DETERMINISM_PY" 2>/tmp/q3.err >/dev/null; then
    :
  else
    _emit_q error "$rel" Q3 "$(cat /tmp/q3.err | tr '\n' ' ')"
    rm -f /tmp/q3.err
    missing=1
  fi
  rm -f /tmp/q3.err

  # Q4 — sidecar staleness.
  local slug; slug="$(basename "$page" .md)"
  local sidecar="content/queries/$slug.sql.hash"
  if [[ -f "$sidecar" ]]; then
    local now_hash; now_hash="sha256-$(bash "$ENGINE" hash "$sql")"
    local prev_hash; prev_hash="$(cat "$sidecar" | tr -d '[:space:]')"
    if [[ "$now_hash" != "$prev_hash" ]]; then
      _emit_q error "$rel" Q4 "sidecar stale: $sidecar (run \`just query-render-one $slug\`)"
      missing=1
    fi
  fi
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

lint_query_fences() {
  # Q5 — every page with awiki-query fences must have a sidecar entry whose
  # current hash matches. (Sidecar is pages-relative, named <page>.queries.json.)
  python3 - <<'PY' > /tmp/q5.tsv
import json, re, base64, pathlib
for page in pathlib.Path("content").rglob("*.md"):
    text = page.read_text(encoding="utf-8")
    fences = list(re.finditer(
        r'```sql\s+awiki-query\s+id="([a-z0-9][a-z0-9-]*)"\s*\n(.*?)\n```',
        text, re.S))
    if not fences:
        continue
    side = page.with_suffix(".queries.json")
    entries = json.loads(side.read_text()) if side.exists() else {}
    for m in fences:
        fid = m.group(1)
        sql_b64 = base64.b64encode(m.group(2).encode()).decode()
        prev = entries.get(fid, "")
        print(f"{page}\t{fid}\t{sql_b64}\t{prev}")
PY
  local rc=0
  while IFS=$'\t' read -r page fid sql_b64 prev; do
    [[ -z "$page" ]] && continue
    local sql; sql="$(printf '%s' "$sql_b64" | base64 --decode)"
    local now; now="sha256-$(bash "$ENGINE" hash "$sql")"
    if [[ "$prev" != "$now" ]]; then
      _emit_q error "$page" Q5 "managed region stale or tampered (id=$fid)"
      rc=1
    fi
  done < /tmp/q5.tsv
  rm -f /tmp/q5.tsv
  return $rc
}

lint_query_priv() {
  : # Q-PRIV stub for Stage 3. Intentionally a no-op.
  return 0
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  rc=0
  lint_query_all || rc=1
  lint_query_fences || rc=1
  lint_query_priv || rc=1
  exit $rc
fi
