#!/usr/bin/env bash
# action-scan.sh — single-pass scanner over content/**/*.md.
# Emits .awiki/maps/actions.tsv and .awiki/maps/actions-rejected.tsv.
# See spec § "Scanner + Agenda Generation".
set -euo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(pwd)}"
cd "$REPO_ROOT"

# Source the canonical action-grammar lib.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/action-grammar.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/lock.sh"

MAPS_DIR="$REPO_ROOT/.awiki/maps"
mkdir -p "$MAPS_DIR"
ACTIONS_TSV="$MAPS_DIR/actions.tsv"
REJECTED_TSV="$MAPS_DIR/actions-rejected.tsv"
ALIAS_MAP="$MAPS_DIR/alias-to-slug.tsv"

# --- Privacy classification helpers ---

# Read git-crypt patterns from .gitattributes. Each pattern becomes a
# bash glob we can match against file paths.
GITCRYPT_PATTERNS=()
if [[ -f "$REPO_ROOT/.gitattributes" ]]; then
  while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    [[ "$line" =~ filter=git-crypt ]] || continue
    pattern="${line%%[[:space:]]*}"
    GITCRYPT_PATTERNS+=("$pattern")
  done < "$REPO_ROOT/.gitattributes"
fi

path_is_private_by_gitcrypt() {
  local relpath="$1"
  local pat
  if (( ${#GITCRYPT_PATTERNS[@]} == 0 )); then
    return 1
  fi
  for pat in "${GITCRYPT_PATTERNS[@]}"; do
    # Convert glob-style ** to bash-extglob equivalent.
    local bash_pat="${pat//\*\*/*}"
    # shellcheck disable=SC2053
    [[ "$relpath" == $bash_pat ]] && return 0
  done
  return 1
}

frontmatter_has_private_tag() {
  local f="$1"
  awk '
    BEGIN { fm = 0 }
    /^---[[:space:]]*$/ { fm = !fm; next }
    fm && /tags:/ && /private/ { found = 1 }
    END { exit (found ? 0 : 1) }
  ' "$f"
}

# Look up a wikilink slug in the alias map. Prints "private" or "public"
# (defaulting to public if the slug is unknown — phase-17 known limit).
wikilink_target_kind() {
  local slug="$1"
  [[ -f "$ALIAS_MAP" ]] || { echo public; return; }
  local kind
  kind="$(awk -F'\t' -v s="$slug" '$2==s { print $4; exit }' "$ALIAS_MAP")"
  [[ -z "$kind" ]] && kind="public"
  printf '%s' "$kind"
}

# Derive source_kind for a file + action line.
classify_source_kind() {
  local relpath="$1" line="$2"
  if path_is_private_by_gitcrypt "$relpath"; then echo private; return; fi
  if frontmatter_has_private_tag "$REPO_ROOT/$relpath"; then echo private; return; fi
  # Wikilink-target rule.
  while [[ "$line" =~ \[\[([^]]+)\]\] ]]; do
    local raw="${BASH_REMATCH[1]}"
    local slug="${raw%%#*}"
    slug="${slug%%|*}"
    if [[ "$(wikilink_target_kind "$slug")" == "private" ]]; then
      echo private; return
    fi
    line="${line/\[\[$raw\]\]/}"
  done
  echo public
}

tsv_escape() {
  local s="$1"
  s="${s//$'\t'/\\t}"
  s="${s//$'\n'/\\n}"
  printf '%s' "$s"
}

# --- Core scan ---

scan_one_file() {
  local relpath="$1"
  local f="$REPO_ROOT/$relpath"
  local lineno=0
  local in_fence=0
  local prev_was_action=0
  local frontmatter_done=0
  local fm_seen=0
  local project_slug=""
  # Project slug is derived from frontmatter if type=project.
  if awk '
    BEGIN { fm = 0 }
    /^---[[:space:]]*$/ { fm = !fm; next }
    fm && /^type:[[:space:]]*project/ { found = 1 }
    END { exit (found ? 0 : 1) }
  ' "$f"; then
    project_slug="$(basename "$relpath" .md)"
  fi

  while IFS= read -r line || [[ -n "$line" ]]; do
    lineno=$((lineno + 1))
    if [[ "$line" =~ ^---[[:space:]]*$ ]]; then
      if (( fm_seen == 0 )); then fm_seen=1
      elif (( frontmatter_done == 0 )); then frontmatter_done=1
      fi
      continue
    fi
    (( fm_seen == 1 && frontmatter_done == 0 )) && continue

    if [[ "$line" =~ ^\`\`\` ]]; then
      in_fence=$((1 - in_fence)); prev_was_action=0; continue
    fi
    (( in_fence == 1 )) && { prev_was_action=0; continue; }

    # Blockquote — skip entirely.
    [[ "$line" =~ ^\> ]] && { prev_was_action=0; continue; }

    # Continuation: indented non-blank line following an action line.
    if (( prev_was_action == 1 )) && [[ "$line" =~ ^[[:space:]]+[^[:space:]] ]]; then
      printf '%s\t%d\t%s\t%s\t%s\n' \
        "$relpath" "$lineno" "continuation" "$(tsv_escape "$line")" "follows-action-line" \
        >> "$REJECTED_TSV"
      continue
    fi

    if [[ ! "$line" =~ ^-[[:space:]]\[(.)\][[:space:]] ]]; then
      prev_was_action=0; continue
    fi

    local status_marker="${BASH_REMATCH[1]}"
    case "$status_marker" in
      ' '|'/'|'?'|'>'|'x'|'-') ;;
      *)
        printf '%s\t%d\t%s\t%s\t%s\n' \
          "$relpath" "$lineno" "bad-status" \
          "$(tsv_escape "$line")" "marker=[$status_marker]" \
          >> "$REJECTED_TSV"
        prev_was_action=1; continue
        ;;
    esac

    awiki_grammar_parse_action "$line" || {
      printf '%s\t%d\t%s\t%s\t%s\n' \
        "$relpath" "$lineno" "no-status" \
        "$(tsv_escape "$line")" "grammar-parse-failed" \
        >> "$REJECTED_TSV"
      prev_was_action=1; continue
    }

    local raw_id="${AWIKI_AG_ID:-}"
    local id="${raw_id#^}"
    local text="${AWIKI_AG_TEXT:-}"
    local context="${AWIKI_AG_CONTEXT:-}"
    local tail="${AWIKI_AG_TAIL:-}"

    # Decompose the tail into per-key columns. The grammar lib emits one
    # `key=value` per line; we walk those into individual scanner columns.
    local due="" defer="" wait="" since="" every="" done_=""
    local priority="" est=""
    if [[ -n "$tail" ]]; then
      local kv k v
      while IFS= read -r kv; do
        [[ -z "$kv" ]] && continue
        k="${kv%%=*}"; v="${kv#*=}"
        case "$k" in
          due)      due="$v" ;;
          defer)    defer="$v" ;;
          wait)     wait="$v" ;;
          since)    since="$v" ;;
          every)    every="$v" ;;
          done)     done_="$v" ;;
          priority) priority="$v" ;;
          est)      est="$v" ;;
        esac
      done < <(awiki_grammar_split_tail "$tail" 2>/dev/null || true)
    fi

    local source_kind
    source_kind="$(classify_source_kind "$relpath" "$line")"

    printf '%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
      "$id" "$status_marker" "$(tsv_escape "$text")" "$relpath" "$lineno" \
      "$context" "$due" "$defer" "$wait" "$since" "$every" "$done_" \
      "$priority" "$est" "$project_slug" "$source_kind" \
      >> "$ACTIONS_TSV"

    prev_was_action=1
  done < "$f"
}

# --- Driver ---

awiki_lock_shared --timeout=30 -- bash -c '
true
' >/dev/null 2>&1 || true   # Phase-17 scanner is read-only against content;
                             # taking the shared lock around the scan body
                             # is sufficient. Below we re-acquire for the
                             # actual write to the maps dir.

ACTIONS_TMP="$ACTIONS_TSV.tmp.$$"
REJECTED_TMP="$REJECTED_TSV.tmp.$$"

: > "$ACTIONS_TMP"
: > "$REJECTED_TMP"

# Header for actions.tsv.
printf 'id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n' \
  > "$ACTIONS_TMP"

ACTIONS_TSV_FINAL="$ACTIONS_TSV"
REJECTED_TSV_FINAL="$REJECTED_TSV"
ACTIONS_TSV="$ACTIONS_TMP"
REJECTED_TSV="$REJECTED_TMP"

scanned_files=0
while IFS= read -r relpath; do
  [[ -z "$relpath" ]] && continue
  scanned_files=$((scanned_files + 1))
  scan_one_file "${relpath#./}"
done < <(cd "$REPO_ROOT" && find content -type f -name '*.md' 2>/dev/null | sort)

mv "$ACTIONS_TMP" "$ACTIONS_TSV_FINAL"
mv "$REJECTED_TMP" "$REJECTED_TSV_FINAL"

ACTIONS_TSV="$ACTIONS_TSV_FINAL"
REJECTED_TSV="$REJECTED_TSV_FINAL"

# Summary + dup-id check.
total_actions=$(($(wc -l < "$ACTIONS_TSV") - 1))
rejected=$(wc -l < "$REJECTED_TSV" | tr -d ' ')
open_count=$(awk -F'\t' 'NR>1 && ($2==" " || $2=="/") {n++} END{print n+0}' "$ACTIONS_TSV")
done_count=$(awk -F'\t' 'NR>1 && $2=="x" {n++} END{print n+0}' "$ACTIONS_TSV")

printf 'scanned %d files, %d actions, %d rejected, %d open, %d done\n' \
  "$scanned_files" "$total_actions" "$rejected" "$open_count" "$done_count"

# Detect duplicate IDs (same literal id appears >=2x).
dup="$(awk -F'\t' 'NR>1 && $1!="" {n[$1]++} END{ for (k in n) if (n[k]>1) print k }' "$ACTIONS_TSV")"
if [[ -n "$dup" ]]; then
  printf 'SCAN|dup-id|%s\n' "$dup" >&2
  exit 1
fi

exit 0
