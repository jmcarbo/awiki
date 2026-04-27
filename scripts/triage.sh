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

# === Source resolution ===
# Resolves <id> to (kind, path, lineno, text). Output is via name-ref.
# Kinds: "inbox-line" | "raw-file" | "action-line"
awiki_resolve_triage_source() {
  local id="$1"; local lineno_param="$2"
  local -n out_kind="$3"
  local -n out_path="$4"
  local -n out_lineno="$5"
  local -n out_text="$6"

  case "$id" in
    inbox-*)
      out_kind="inbox-line"
      out_path="${AWIKI_REPO_ROOT:-.}/content/inbox.md"
      if [[ -z "$lineno_param" ]]; then
        # Try to recover lineno from id suffix (last `-` group).
        if [[ "$id" =~ ^inbox-[a-z0-9]{10}-([0-9]+)$ ]]; then
          lineno_param="${BASH_REMATCH[1]}"
        else
          echo "triage: inbox id missing lineno: $id" >&2
          exit 9
        fi
      fi
      out_lineno="$lineno_param"
      out_text="$(awiki_extract_inbox_text "$out_path" "$out_lineno")"
      ;;
    file-*)
      out_kind="raw-file"
      # Reverse: id encodes sha1 of relpath; we need the relpath. Look it up by
      # scanning raw/inbox/interactive/ entries.
      local sha="${id#file-}"
      local found=""
      while IFS= read -r -d '' f; do
        local rel="${f#"${AWIKI_REPO_ROOT:-.}"/}"
        local fsha; fsha="$(printf '%s' "$rel" | sha1sum | awk '{print substr($1,1,10)}')"
        if [[ "$fsha" == "$sha" ]]; then found="$rel"; break; fi
      done < <(find "${AWIKI_REPO_ROOT:-.}/raw/inbox/interactive" -type f -print0 2>/dev/null)
      if [[ -z "$found" ]]; then
        echo "triage: file id does not resolve: $id" >&2
        exit 9
      fi
      out_path="${AWIKI_REPO_ROOT:-.}/$found"
      out_lineno=0
      out_text="$(basename "$found")"
      ;;
    *)
      out_kind="action-line"
      # Look up via actions.tsv.
      local row
      row="$(awk -F'\t' -v id="$id" 'NR>1 && $1==id {print; exit}' \
              "${AWIKI_REPO_ROOT:-.}/.awiki/maps/actions.tsv" 2>/dev/null || true)"
      if [[ -z "$row" ]]; then
        echo "triage: action id not found in actions.tsv: $id" >&2
        exit 9
      fi
      out_path="${AWIKI_REPO_ROOT:-.}/$(awk -F'\t' '{print $4}' <<<"$row")"
      out_lineno="$(awk -F'\t' '{print $5}' <<<"$row")"
      out_text="$(awk -F'\t' '{print $3}' <<<"$row")"
      ;;
  esac
}

# Strips the leading "- <ISO-datetime> " prefix from an inbox line.
awiki_extract_inbox_text() {
  local path="$1"; local lineno="$2"
  local raw; raw="$(sed -n "${lineno}p" "$path")"
  # Pattern: - YYYY-MM-DD HH:MM <text>
  if [[ "$raw" =~ ^-\ [0-9]{4}-[0-9]{2}-[0-9]{2}\ [0-9]{2}:[0-9]{2}\ (.*)$ ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
  else
    printf '%s' "$raw"
  fi
}

# === Validation helpers ===
awiki_check_project_slug() {
  local s="$1"
  [[ "$s" =~ ^[a-z0-9_][a-z0-9_-]{0,63}$ ]] || { echo "triage: bad project_slug: $s" >&2; exit 4; }
}
awiki_check_slug() {
  local label="$1"; local s="$2"
  [[ "$s" =~ ^[a-z0-9][a-z0-9-]{0,63}$ ]] || { echo "triage: bad $label: $s" >&2; exit 4; }
}
awiki_check_page_type() {
  case "$1" in entity|concept|topic|source) ;; *) echo "triage: bad page_type: $1" >&2; exit 4 ;; esac
}
awiki_check_iso_date() {
  local label="$1"; local s="$2"
  [[ "$s" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || { echo "triage: bad date ($label): $s" >&2; exit 4; }
  # Calendar validity. Use the action-grammar lib helper (portable across GNU/BSD date).
  if ! _awiki_is_calendar_date "$s"; then
    echo "triage: bad date ($label): $s" >&2; exit 4
  fi
}

# Mint a fresh 8-char block-id, collision-checked against actions.tsv.
awiki_mint_id() {
  local actions="${AWIKI_REPO_ROOT:-.}/.awiki/maps/actions.tsv"
  local i id existing=""
  if [[ -f "$actions" ]]; then
    existing="$(awk -F'\t' 'NR>1 {print $1}' "$actions")"
  fi
  for i in 1 2 3 4 5; do
    id="$(LC_ALL=C tr -dc 'a-z0-9' </dev/urandom | head -c 8)"
    if ! grep -qx "$id" <<<"$existing"; then
      printf '%s' "$id"; return 0
    fi
  done
  echo "triage: failed to mint unique id after 5 retries" >&2
  exit 8
}

# === Lazy page creation ===
# Ensures content/projects/<slug>.md exists; lazily creates _loose / _someday
# with the right status. Returns absolute path of the page on stdout.
# Sets the global AWIKI_LAST_PAGE_CREATED=1 if a fresh page was just minted,
# 0 if the page already existed.
awiki_ensure_project_page() {
  local slug="$1"
  awiki_check_project_slug "$slug"
  local path="${AWIKI_REPO_ROOT:-.}/content/projects/${slug}.md"
  if [[ -f "$path" ]]; then
    AWIKI_LAST_PAGE_CREATED=0
    printf '%s' "$path"
    return 0
  fi
  AWIKI_LAST_PAGE_CREATED=1
  local status_default="active"
  local title="$slug"
  case "$slug" in
    _someday) status_default="someday"; title="Someday/Maybe (catch-all)" ;;
    _loose)   status_default="active";  title="Loose actions (catch-all)" ;;
  esac
  local today; today="$(date -u +%Y-%m-%d)"
  cat > "$path" <<EOF
---
title: "${title}"
date: ${today}
last_updated: ${today}
type: project
status: ${status_default}
outcome: ""
tags: []
draft: false
---

## Open Actions

## Done
EOF
  printf '%s' "$path"
}

# Append <line> under the page's "## Open Actions" heading atomically.
awiki_append_under_open_actions() {
  local path="$1"; local line="$2"
  local tmp; tmp="$(mktemp "${path}.tmp.XXXXXX")"
  awk -v ln="$line" '
    BEGIN { inserted=0 }
    /^## Open Actions[[:space:]]*$/ {
      print
      print ln
      inserted=1
      next
    }
    { print }
    END {
      if (!inserted) {
        print ""
        print "## Open Actions"
        print ln
      }
    }' "$path" > "$tmp"
  mv "$tmp" "$path"
}

# Append <line> under the page's "## Done" heading atomically.
awiki_append_under_done() {
  local path="$1"; local line="$2"
  local tmp; tmp="$(mktemp "${path}.tmp.XXXXXX")"
  awk -v ln="$line" '
    BEGIN { inserted=0 }
    /^## Done[[:space:]]*$/ {
      print
      print ln
      inserted=1
      next
    }
    { print }
    END {
      if (!inserted) {
        print ""
        print "## Done"
        print ln
      }
    }' "$path" > "$tmp"
  mv "$tmp" "$path"
}

# Remove the inbox line at <lineno> atomically.
awiki_remove_inbox_line() {
  local path="$1"; local lineno="$2"
  local tmp; tmp="$(mktemp "${path}.tmp.XXXXXX")"
  awk -v n="$lineno" 'NR != n' "$path" > "$tmp"
  mv "$tmp" "$path"
}

# Replace the inbox line at <lineno> with strikethrough form.
awiki_strikethrough_inbox_line() {
  local path="$1"; local lineno="$2"
  local tmp; tmp="$(mktemp "${path}.tmp.XXXXXX")"
  awk -v n="$lineno" 'NR == n { print "~~" $0 "~~"; next } { print }' "$path" > "$tmp"
  mv "$tmp" "$path"
}

# === Outcome handlers ===

# Helper: every handler that touches a project page should call this AFTER
# awiki_ensure_project_page so the trailer lists creates vs updates correctly.
_triage_record_project_page() {
  local proj_path="$1"
  if [[ "${AWIKI_LAST_PAGE_CREATED:-0}" == "1" ]]; then
    TRIAGE_CREATED_PAGES+=("$proj_path")
  else
    TRIAGE_UPDATED_PAGES+=("$proj_path")
  fi
}

triage_outcome_trash() {
  local kind="$1"; local path="$2"; local lineno="$3"; local _text="$4"
  if [[ "$kind" == "inbox-line" ]]; then
    awiki_strikethrough_inbox_line "$path" "$lineno"
    TRIAGE_ACTIONS_TAKEN+=("strikethrough inbox line $lineno")
    TRIAGE_UPDATED_PAGES+=("$path")
  elif [[ "$kind" == "raw-file" ]]; then
    mkdir -p "${AWIKI_REPO_ROOT:-.}/raw/inbox/.trash"
    mv "$path" "${AWIKI_REPO_ROOT:-.}/raw/inbox/.trash/"
    TRIAGE_ACTIONS_TAKEN+=("moved raw capture to .trash: $(basename "$path")")
  fi
}

triage_outcome_do_now() {
  local kind="$1"; local path="$2"; local lineno="$3"; local text="$4"
  local -n p="$5"
  awiki_check_project_slug "${p[project_slug]}"
  awiki_check_slug context_slug "${p[context_slug]}"
  local proj; proj="$(awiki_ensure_project_page "${p[project_slug]}")"
  _triage_record_project_page "$proj"
  local id; id="$(awiki_mint_id)"
  local today; today="$(date -u +%Y-%m-%d)"
  awiki_append_under_done "$proj" \
    "- [x] ${text} @${p[context_slug]} done:${today} ^${id}"
  TRIAGE_ACTIONS_TAKEN+=("do-now @${p[context_slug]} -> ${p[project_slug]} (^${id})")
  if [[ "$kind" == "inbox-line" ]]; then
    awiki_remove_inbox_line "$path" "$lineno"
    TRIAGE_UPDATED_PAGES+=("$path")
  fi
}

triage_outcome_act() {
  local kind="$1"; local path="$2"; local lineno="$3"; local text="$4"
  local -n p="$5"
  awiki_check_project_slug "${p[project_slug]:-_loose}"
  if [[ -n "${p[context_slug]:-}" ]]; then
    awiki_check_slug context_slug "${p[context_slug]}"
  fi
  local proj; proj="$(awiki_ensure_project_page "${p[project_slug]:-_loose}")"
  _triage_record_project_page "$proj"
  local id; id="$(awiki_mint_id)"
  local ctx_token=""
  [[ -n "${p[context_slug]:-}" ]] && ctx_token=" @${p[context_slug]}"
  awiki_append_under_open_actions "$proj" \
    "- [ ] ${text}${ctx_token} ^${id}"
  TRIAGE_ACTIONS_TAKEN+=("act${ctx_token} -> ${p[project_slug]:-_loose} (^${id})")
  if [[ "$kind" == "inbox-line" ]]; then
    awiki_remove_inbox_line "$path" "$lineno"
    TRIAGE_UPDATED_PAGES+=("$path")
  fi
}

triage_outcome_defer_scheduled() {
  local kind="$1"; local path="$2"; local lineno="$3"; local text="$4"
  local -n p="$5"
  awiki_check_project_slug "${p[project_slug]:-_loose}"
  [[ -n "${p[context_slug]:-}" ]] && awiki_check_slug context_slug "${p[context_slug]}"
  local id ctx_token="" date_token=""
  [[ -n "${p[context_slug]:-}" ]] && ctx_token=" @${p[context_slug]}"
  if [[ -n "${p[due]:-}" ]]; then
    awiki_check_iso_date due "${p[due]}"; date_token=" due:${p[due]}"
  elif [[ -n "${p[defer]:-}" ]]; then
    awiki_check_iso_date defer "${p[defer]}"; date_token=" defer:${p[defer]}"
  else
    echo "triage: defer-scheduled requires due= or defer=" >&2; exit 4
  fi
  local proj; proj="$(awiki_ensure_project_page "${p[project_slug]:-_loose}")"
  _triage_record_project_page "$proj"
  id="$(awiki_mint_id)"
  awiki_append_under_open_actions "$proj" \
    "- [ ] ${text}${ctx_token}${date_token} ^${id}"
  TRIAGE_ACTIONS_TAKEN+=("defer-scheduled${date_token} -> ${p[project_slug]:-_loose} (^${id})")
  if [[ "$kind" == "inbox-line" ]]; then
    awiki_remove_inbox_line "$path" "$lineno"
    TRIAGE_UPDATED_PAGES+=("$path")
  fi
}

triage_outcome_waiting() {
  local kind="$1"; local path="$2"; local lineno="$3"; local text="$4"
  local -n p="$5"
  awiki_check_project_slug "${p[project_slug]:-_loose}"
  [[ -n "${p[wait_for]:-}" ]] || { echo "triage: waiting requires wait_for=" >&2; exit 4; }
  awiki_check_slug wait_for "${p[wait_for]}"
  local proj; proj="$(awiki_ensure_project_page "${p[project_slug]:-_loose}")"
  _triage_record_project_page "$proj"
  local id; id="$(awiki_mint_id)"
  local today; today="$(date -u +%Y-%m-%d)"
  awiki_append_under_open_actions "$proj" \
    "- [?] ${text} wait:[[${p[wait_for]}]] since:${today} ^${id}"
  TRIAGE_ACTIONS_TAKEN+=("waiting on [[${p[wait_for]}]] -> ${p[project_slug]:-_loose} (^${id})")
  if [[ "$kind" == "inbox-line" ]]; then
    awiki_remove_inbox_line "$path" "$lineno"
    TRIAGE_UPDATED_PAGES+=("$path")
  fi
}

triage_outcome_reference() {
  local kind="$1"; local path="$2"; local lineno="$3"; local text="$4"
  local -n p="$5"
  awiki_check_page_type "${p[page_type]}"
  awiki_check_slug ref_slug "${p[ref_slug]}"
  local target_dir="${AWIKI_REPO_ROOT:-.}/content/${p[page_type]}s"
  mkdir -p "$target_dir"
  local target="${target_dir}/${p[ref_slug]}.md"
  # Path-resolution guard: confirm the resolved path is under the canonical parent.
  local canonical; canonical="$(cd "$target_dir" && pwd)"
  local repo_canonical; repo_canonical="$(cd "${AWIKI_REPO_ROOT:-.}" && pwd)"
  case "$canonical" in
    "${repo_canonical}"/content/*s) ;;
    *) echo "triage: refusing to write outside content/<page_type>s/" >&2; exit 4 ;;
  esac
  if [[ -e "$target" ]]; then
    echo "triage: reference target already exists: $target" >&2; exit 5
  fi
  local today; today="$(date -u +%Y-%m-%d)"
  cat > "$target" <<EOF
---
title: "${text}"
date: ${today}
last_updated: ${today}
type: ${p[page_type]}
tags: []
aliases: []
draft: false
---

(captured ${today})
EOF
  TRIAGE_ACTIONS_TAKEN+=("reference -> ${p[page_type]}/${p[ref_slug]}")
  TRIAGE_CREATED_PAGES+=("$target")
  if [[ "$kind" == "inbox-line" ]]; then
    awiki_remove_inbox_line "$path" "$lineno"
    TRIAGE_UPDATED_PAGES+=("$path")
  fi
}

triage_outcome_someday() {
  local kind="$1"; local path="$2"; local lineno="$3"; local text="$4"
  local -n p="$5"
  awiki_check_project_slug "${p[project_slug]:-_someday}"
  local proj; proj="$(awiki_ensure_project_page "${p[project_slug]:-_someday}")"
  _triage_record_project_page "$proj"
  local id; id="$(awiki_mint_id)"
  awiki_append_under_open_actions "$proj" \
    "- [>] ${text} ^${id}"
  TRIAGE_ACTIONS_TAKEN+=("someday -> ${p[project_slug]:-_someday} (^${id})")
  if [[ "$kind" == "inbox-line" ]]; then
    awiki_remove_inbox_line "$path" "$lineno"
    TRIAGE_UPDATED_PAGES+=("$path")
  fi
}

# === TRIAGE-RESULT trailer ===
# Emit JSON array from a bash array (escapes \\ and ").
_triage_json_array() {
  local arr_name="$1"
  local -n arr="$arr_name"
  if [[ ${#arr[@]} -eq 0 ]]; then
    printf '[]'
    return
  fi
  printf '['
  local first=1 item
  for item in "${arr[@]}"; do
    [[ $first -eq 0 ]] && printf ','
    item="${item//\\/\\\\}"
    item="${item//\"/\\\"}"
    printf '"%s"' "$item"
    first=0
  done
  printf ']'
}

_triage_emit_trailer() {
  local at cp up
  at="$(_triage_json_array TRIAGE_ACTIONS_TAKEN)"
  cp="$(_triage_json_array TRIAGE_CREATED_PAGES)"
  up="$(_triage_json_array TRIAGE_UPDATED_PAGES)"
  printf 'TRIAGE-RESULT|{"actions_taken":%s,"created_pages":%s,"updated_pages":%s}\n' \
    "$at" "$cp" "$up"
}

# Stubs filled in by tasks 18a.9 / 18a.11.
awiki_verify_inbox_line_id() { :; }
triage_increment_and_maybe_rebuild() { :; }
triage_interactive() { echo "TODO interactive" >&2; return 0; }

main "$@"
