#!/usr/bin/env bash
set -euo pipefail

# triage.sh — bash fallback for the seven-outcome triage workflow.
# Two modes: positional CLI (single item) and --interactive (walks inbox).
# The MCP triage_apply tool (phase 18b) calls into the same library entry-points
# as this script. Side effects must be identical.

# Capture caller-supplied AWIKI_LOCK_TIMEOUT_USER before sourcing lock.sh
# (lock.sh re-exports the default 30 unconditionally).
TRIAGE_USER_TIMEOUT_OVERRIDE="${AWIKI_LOCK_TIMEOUT_USER:-}"

# shellcheck source=lib/action-grammar.sh
source "$(dirname "$0")/lib/action-grammar.sh"
# shellcheck source=lib/lock.sh
source "$(dirname "$0")/lib/lock.sh"

# Re-apply the override so the rest of this script sees the caller's value.
if [[ -n "$TRIAGE_USER_TIMEOUT_OVERRIDE" ]]; then
  AWIKI_LOCK_TIMEOUT_USER="$TRIAGE_USER_TIMEOUT_OVERRIDE"
fi

readonly OUTCOMES="trash do-now act defer-scheduled waiting reference someday"

usage() {
  cat <<EOF
Usage:
  triage.sh <id> <outcome> [k=v ...]
  triage.sh --interactive
  triage.sh -h | --help

outcomes:
  trash             — strikethrough inbox line / move file to .trash
  do-now            — append [x] line under chosen project / context, log
  act               — append [ ] line, default project=_loose if absent
  defer-scheduled   — append [ ] with due: or defer:
  waiting           — append [?] with wait:[[<entity>]] since:<today>
  reference         — promote to a wiki page (page_type + ref_slug)
  someday           — append [>] under chosen project or _someday

Per-outcome k=v params:
  trash:           lineno=<int>     (required for inbox-* ids)
  do-now:          project_slug=    context_slug=    lineno=
  act:             project_slug=    context_slug=    lineno=
  defer-scheduled: project_slug=    context_slug=    due=YYYY-MM-DD | defer=YYYY-MM-DD    lineno=
  waiting:         project_slug=    wait_for=<entity-slug>    lineno=
  reference:       page_type=       ref_slug=    lineno=
  someday:         project_slug=    lineno=

Common options (env):
  AWIKI_AGENDA_AFTER_N — integer threshold for auto-rebuild (default 5)
  AWIKI_LOCK_TIMEOUT_USER — flock timeout in seconds (default 30)
EOF
}

main() {
  if [[ $# -eq 0 ]]; then
    usage; exit 2
  fi
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --interactive) shift; triage_interactive "$@"; return $? ;;
  esac
  if [[ $# -lt 2 ]]; then
    usage >&2; exit 2
  fi
  local id="$1"; shift
  local outcome="$1"; shift

  if ! [[ " $OUTCOMES " == *" $outcome "* ]]; then
    echo "triage: unknown outcome: $outcome" >&2
    usage >&2
    exit 2
  fi

  # Parse k=v positional args into associative array.
  declare -A params=()
  local kv
  for kv in "$@"; do
    if [[ "$kv" != *=* ]]; then
      echo "triage: arg must be k=v: '$kv'" >&2
      exit 2
    fi
    params["${kv%%=*}"]="${kv#*=}"
  done

  # Acquire exclusive lock and dispatch. The lock holds for the duration of the
  # outcome handler; the deferred agenda rebuild runs after release.
  local timeout="${AWIKI_LOCK_TIMEOUT_USER:-30}"
  awiki_lock_with --timeout="$timeout" -- triage_apply_locked "$id" "$outcome" "$(declare -p params)"

  # After the lock releases, increment task-count and possibly enqueue rebuild.
  triage_increment_and_maybe_rebuild
}

# triage_apply_locked <id> <outcome> <params-decl>
# Runs inside the awiki_lock_with subshell. The third arg is the output of
# `declare -p params` from the parent — we eval it to materialize the same
# associative array inside the subshell.
triage_apply_locked() {
  local id="$1"; local outcome="$2"; local params_decl="$3"

  # Re-materialize the params associative array inside this subshell.
  declare -A params=()
  eval "$params_decl"

  # Tracking arrays for the TRIAGE-RESULT trailer.
  declare -a TRIAGE_ACTIONS_TAKEN=()
  declare -a TRIAGE_CREATED_PAGES=()
  declare -a TRIAGE_UPDATED_PAGES=()

  # 1. Resolve the source: inbox-<sha>-<lineno> | file-<sha> | <block-id>.
  local src_kind src_path src_lineno src_text
  awiki_resolve_triage_source "$id" "${params[lineno]:-}" \
    src_kind src_path src_lineno src_text

  # 2. Inbox TOCTOU re-verify (Task 18a.9).
  if [[ "$src_kind" == "inbox-line" ]]; then
    awiki_verify_inbox_line_id "$id" "$src_path" "$src_lineno"
  fi

  # 3. Dispatch to outcome handler (Task 18a.8).
  case "$outcome" in
    trash)           triage_outcome_trash           "$src_kind" "$src_path" "$src_lineno" "$src_text" params ;;
    do-now)          triage_outcome_do_now          "$src_kind" "$src_path" "$src_lineno" "$src_text" params ;;
    act)             triage_outcome_act             "$src_kind" "$src_path" "$src_lineno" "$src_text" params ;;
    defer-scheduled) triage_outcome_defer_scheduled "$src_kind" "$src_path" "$src_lineno" "$src_text" params ;;
    waiting)         triage_outcome_waiting         "$src_kind" "$src_path" "$src_lineno" "$src_text" params ;;
    reference)       triage_outcome_reference       "$src_kind" "$src_path" "$src_lineno" "$src_text" params ;;
    someday)         triage_outcome_someday         "$src_kind" "$src_path" "$src_lineno" "$src_text" params ;;
  esac

  # 4. Log via the (now poisoning-protected) log-append.
  bash "$(dirname "$0")/log-append.sh" triage "$outcome | ${params[project_slug]:-} | ${params[ref_slug]:-}"

  # 5. Emit the TRIAGE-RESULT trailer (only reached on rc=0; set -e aborts earlier on failure).
  _triage_emit_trailer
}

# Stubs filled in by tasks 18a.8 / 18a.9 / 18a.11.
triage_outcome_trash() { echo "TODO trash" >&2; return 0; }
triage_outcome_do_now() { echo "TODO do-now" >&2; return 0; }
triage_outcome_act() { echo "TODO act" >&2; return 0; }
triage_outcome_defer_scheduled() { echo "TODO defer-scheduled" >&2; return 0; }
triage_outcome_waiting() { echo "TODO waiting" >&2; return 0; }
triage_outcome_reference() { echo "TODO reference" >&2; return 0; }
triage_outcome_someday() { echo "TODO someday" >&2; return 0; }
awiki_resolve_triage_source() { :; }
awiki_verify_inbox_line_id() { :; }
triage_increment_and_maybe_rebuild() { :; }
triage_interactive() { echo "TODO interactive" >&2; return 0; }
_triage_emit_trailer() { :; }

main "$@"
