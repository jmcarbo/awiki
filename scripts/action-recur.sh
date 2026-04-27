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

# Compute the would-be page contents on stdout. Single-pass over the file.
# Preserves all non-action content verbatim. For each [x] every:... line,
# prepends an open instance line if not already present in the chain.
recur_compute_new_contents() {
  local page="$1"
  awiki_recur_emit_pass "$page"
}

awiki_recur_emit_pass() {
  local page="$1"

  # Collect existing chain state on this page: lines of "<base>\t<n>".
  local chains
  chains="$(awiki_collect_chain_state "$page")"

  # Stream the page; emit open-copy line above each [x] every:... line.
  local line
  while IFS= read -r line || [[ -n "$line" ]]; do
    if awiki_is_completed_recurring_action "$line"; then
      local base every done_date next_due next_n new_id new_line
      base="$(awiki_extract_chain_base "$line")"
      every="$(awiki_extract_tail_key "$line" every)"
      done_date="$(awiki_extract_tail_key "$line" done)"
      [[ -z "$done_date" ]] && done_date="$(date -u +%Y-%m-%d)"
      next_due="$(awiki_recur_compute_due "$done_date" "$every")"
      next_n="$(awiki_recur_next_instance_n "$chains" "$base")"
      # Idempotence: if next_n already exists, emit nothing extra.
      if awiki_recur_chain_has "$chains" "$base" "$next_n"; then
        printf '%s\n' "$line"
        continue
      fi
      # Cap check: refuse-to-emit at chain length >= 200.
      if [[ "$next_n" -ge 200 ]]; then
        echo "action-recur: chain ^${base} reached 200 instances; refusing to emit (exit 6)" >&2
        exit 6
      fi
      new_id="${base}${AWIKI_RECUR_SEP}${next_n}"
      new_line="$(awiki_build_open_copy "$line" "$next_due" "$new_id")"
      printf '%s\n' "$new_line"
      printf '%s\n' "$line"
      # Update local chain state for any subsequent [x] in same chain on this page.
      chains="${chains}"$'\n'"${base}"$'\t'"${next_n}"
    else
      printf '%s\n' "$line"
    fi
  done < "$page"
}

main "$@"
