#!/usr/bin/env bash
# scripts/query.sh — DuckDB query layer CLI.
# Subcommands: run, new, render, render-one, fence-render.
set -uo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

ENGINE="$REPO_ROOT/scripts/lib/query-engine.sh"

note() { echo "QUERY|$*"; }
die()  { echo "QUERY|ERROR|$*" >&2; exit 1; }

cmd_run() {
  local sql=""
  local out=""
  local force=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --out=*)   out="${1#--out=}" ;;
      --force)   force=1 ;;
      --)        shift; break ;;
      -*)        die "unknown flag: $1" ;;
      *)         sql="$1" ;;
    esac
    shift
  done
  [[ -n "$sql" ]] || die "missing SQL"
  if [[ -n "$out" ]]; then
    bash "$ENGINE" run "$sql" > /tmp/q-rows.json
    local rc=$?
    [[ $rc -eq 0 ]] || exit $rc
    # Materialization happens in Task 8.
    die "--out=<slug> requires Task 8 materializer (not yet implemented)"
  else
    bash "$ENGINE" run "$sql"
  fi
}

main() {
  local cmd="${1:-}"
  shift || true
  case "$cmd" in
    run)           cmd_run "$@" ;;
    "")            die "usage: query.sh <run|new|render|render-one|fence-render>" ;;
    *)             die "unknown subcommand: $cmd" ;;
  esac
}

main "$@"
