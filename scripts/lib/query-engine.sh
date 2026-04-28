#!/usr/bin/env bash
# scripts/lib/query-engine.sh — pure DuckDB engine: extract refs, run SQL,
# emit JSON rows on stdout. Surfaces (CLI, page, fence) wrap this.
#
# Subcommands:
#   run "<SQL>"   — run query, print DuckDB JSON rows on stdout
#   hash "<SQL>"  — print sha256 of (normalized SQL + sorted slug:filehash)
#
# Exit codes: 2 duckdb-missing | 3 unknown-dataset | 4 extract-fail
#             5 sql-parse | 9 external-attach-rejected
set -uo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
DATASETS_DIR="${AWIKI_DATASETS_DIR:-content/datasets}"
CACHE_DIR="${AWIKI_QUERY_CACHE:-.cache/duckdb}"
EXTRACT_PY="$REPO_ROOT/scripts/lib/query-extract.py"
RESOLVE_PY="$REPO_ROOT/scripts/lib/query-resolve.py"

note() { echo "QUERY|$*"; }
die()  { echo "QUERY|ERROR|$*" >&2; }

_require_duckdb() {
  command -v duckdb >/dev/null 2>&1 || { die "duckdb CLI not found"; return 2; }
}

_reject_attach() {
  local sql="$1"
  if printf '%s' "$sql" | grep -iE '\bATTACH\b' >/dev/null; then
    die "external attach not supported until stage 3"
    return 9
  fi
  return 0
}

_resolve_refs() {
  local sql="$1"
  printf '%s' "$sql" | python3 "$RESOLVE_PY"
}

_extract_one() {
  local slug="$1" fmt="$2"
  python3 "$EXTRACT_PY" --slug="$slug" --datasets-dir="$DATASETS_DIR" \
    --out="$CACHE_DIR/$slug.$fmt"
}

_format_for() {
  local slug="$1"
  python3 - "$DATASETS_DIR/$slug.md" <<'PY'
import re, sys, pathlib
text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
m = re.search(r"^format:\s*(\S+)", text, re.M)
print(m.group(1) if m else "csv")
PY
}

_view_ddl() {
  local slug="$1" fmt="$2" path="$CACHE_DIR/$slug.$fmt"
  case "$fmt" in
    csv|tsv|dsv) printf "CREATE VIEW %s AS SELECT * FROM read_csv('%s', AUTO_DETECT=TRUE);\n" "$slug" "$path" ;;
    json)        printf "CREATE VIEW %s AS SELECT * FROM read_json_auto('%s');\n" "$slug" "$path" ;;
    *)           printf "CREATE VIEW %s AS SELECT * FROM read_csv('%s', AUTO_DETECT=TRUE);\n" "$slug" "$path" ;;
  esac
}

_prepare() {
  local sql="$1"
  _require_duckdb || return $?
  _reject_attach "$sql" || return $?
  mkdir -p "$CACHE_DIR"
  local refs ddl=""
  refs="$(_resolve_refs "$sql")"
  while IFS= read -r slug; do
    [[ -z "$slug" ]] && continue
    if [[ ! -f "$DATASETS_DIR/$slug.md" ]]; then
      die "unknown dataset: $slug"
      return 3
    fi
    local fmt; fmt="$(_format_for "$slug")"
    _extract_one "$slug" "$fmt" || return 4
    ddl+="$(_view_ddl "$slug" "$fmt")"
  done <<< "$refs"
  printf '%s' "$ddl"
}

cmd_run() {
  local sql="$1"
  local ddl
  ddl="$(_prepare "$sql")" || return $?
  local script="$ddl
$sql
"
  local out
  out="$(printf '%s' "$script" | duckdb -json 2>&1)"
  local rc=$?
  if [[ $rc -ne 0 ]]; then
    printf '%s\n' "$out" >&2
    return 5
  fi
  printf '%s\n' "$out"
}

cmd_hash() {
  local sql="$1"
  local refs slug fmt path file_hash payload=""
  refs="$(_resolve_refs "$sql")" || return $?
  while IFS= read -r slug; do
    [[ -z "$slug" ]] && continue
    fmt="$(_format_for "$slug")"
    path="$DATASETS_DIR/$slug.md"
    file_hash="$(shasum -a 256 "$path" | awk '{print $1}')"
    payload+="$slug:$file_hash"$'\n'
  done <<< "$refs"
  local norm
  norm="$(printf '%s' "$sql" | tr -s '[:space:]' ' ' | sed -e 's/^ *//' -e 's/ *$//')"
  printf '%s\n%s' "$norm" "$payload" | shasum -a 256 | awk '{print $1}'
}

main() {
  local cmd="${1:-}"
  shift || true
  case "$cmd" in
    run)  cmd_run "$@" ;;
    hash) cmd_hash "$@" ;;
    *) die "unknown subcommand: $cmd"; return 1 ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
