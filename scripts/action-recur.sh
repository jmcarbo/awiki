#!/usr/bin/env bash
set -euo pipefail

# action-recur.sh — re-emit open copies of completed [x] every:... actions.
#
# Idempotent. Reads actions.tsv (built under lock by action-scan.sh) for chain
# state. Emits each instance immediately above its [x] predecessor on the same
# page. Uses temp-file + atomic rename for writes. Refuses to emit at chain
# >=200 (exit 6). Honors AWIKI_RECUR_SEP from action-grammar.sh.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=lib/action-grammar.sh
. "$SCRIPT_DIR/lib/action-grammar.sh"
# shellcheck source=lib/lock.sh
. "$SCRIPT_DIR/lib/lock.sh"

usage() {
  cat <<EOF
Usage:
  action-recur.sh [--dry-run] <page-path>
  action-recur.sh [--dry-run] --all
  action-recur.sh -h | --help
EOF
}

main() {
  local dry_run=0
  local all=0
  local target=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --dry-run) dry_run=1; shift ;;
      --all)     all=1;     shift ;;
      -h|--help) usage; exit 0 ;;
      --)        shift; break ;;
      -*)        echo "action-recur: unknown flag: $1" >&2; usage >&2; exit 2 ;;
      *)         target="$1"; shift ;;
    esac
  done

  if [[ "$all" -eq 1 && -n "$target" ]]; then
    echo "action-recur: --all and <page-path> are mutually exclusive" >&2
    exit 2
  fi
  if [[ "$all" -eq 0 && -z "$target" ]]; then
    usage >&2
    exit 2
  fi

  if [[ "$all" -eq 1 ]]; then
    # Discover pages with completed every: lines from actions.tsv.
    local actions_tsv="${AWIKI_REPO_ROOT:-.}/.awiki/maps/actions.tsv"
    if [[ ! -f "$actions_tsv" ]]; then
      echo "action-recur: actions.tsv missing; run action-scan.sh first" >&2
      exit 3
    fi
    # Column layout matches phase 17. status=x AND every is non-empty.
    local pages
    pages="$(awk -F'\t' 'NR>1 && $2=="x" && $11!="" { print $4 }' "$actions_tsv" | sort -u)"
    if [[ -z "$pages" ]]; then
      return 0
    fi
    while IFS= read -r page; do
      [[ -z "$page" ]] && continue
      recur_one_page "$page" "$dry_run"
    done <<<"$pages"
    return 0
  fi

  recur_one_page "$target" "$dry_run"
}

# recur_one_page <page-path> <dry-run-flag>
recur_one_page() {
  local page="$1"
  local dry="$2"

  if [[ ! -f "$page" ]]; then
    echo "action-recur: page not found: $page" >&2
    exit 3
  fi

  if [[ "$dry" -eq 0 ]]; then
    awiki_lock_with --timeout=30 -- recur_one_page_locked "$page"
  else
    # Dry-run reads only; shared lock is sufficient. The diff output goes to
    # stdout for the caller to inspect.
    awiki_lock_shared --timeout=30 -- recur_one_page_diff "$page"
  fi
}

# Compute the would-be page contents into a temp file and emit `diff -u`.
recur_one_page_diff() {
  local page="$1"
  local tmp
  tmp="$(mktemp "${page}.tmp.XXXXXX")"
  trap 'rm -f "$tmp"' RETURN
  recur_compute_new_contents "$page" > "$tmp"
  # If no change, exit cleanly with no output.
  if cmp -s "$page" "$tmp"; then
    return 0
  fi
  diff -u "$page" "$tmp" || true
}

recur_one_page_locked() {
  local page="$1"
  local tmp
  tmp="$(mktemp "${page}.tmp.XXXXXX")"
  trap 'rm -f "$tmp"' EXIT
  recur_compute_new_contents "$page" > "$tmp"
  if cmp -s "$page" "$tmp"; then
    return 0
  fi
  mv "$tmp" "$page"
  trap - EXIT   # `mv` consumed the temp file; clear the cleanup trap.
  bash "$SCRIPT_DIR/log-append.sh" recur "page=$page"
}

# Phase 18a.4 onward: real arithmetic + chain logic. Skeleton stub:
recur_compute_new_contents() {
  local page="$1"
  cat "$page"   # no-op until task 18a.4
}

main "$@"
