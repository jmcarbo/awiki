#!/usr/bin/env bash
# agenda.sh — regenerate the five managed-region agenda pages from
# .awiki/maps/actions.tsv. Privacy filter excludes source_kind=private
# (counts only — placeholder callout, no slugs/text). Atomic temp-file
# rename per page; rewrites frontmatter `last_updated:` to today.
#
# Exit codes:
#   0  success
#   2  missing actions.tsv (run action-scan.sh first)
#   5  AWIKI_AGENDA_INCLUDE_PRIVATE=1 but content/agenda/** is not
#      under git-crypt patterns (refuse to inline private rows)
#   7  flock timeout (lock contention)
set -euo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(pwd)}"
cd "$REPO_ROOT"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/lock.sh"

MAPS="$REPO_ROOT/.awiki/maps"
ACTIONS="$MAPS/actions.tsv"
AGENDA_DIR="$REPO_ROOT/content/agenda"
TODAY="$(date +%Y-%m-%d)"
INCLUDE_PRIVATE="${AWIKI_AGENDA_INCLUDE_PRIVATE:-0}"

[[ -f "$ACTIONS" ]] || {
  echo "AGENDA|missing|$ACTIONS — run action-scan.sh first" >&2
  exit 2
}

# Encryption-coverage gate for INCLUDE_PRIVATE=1.
if [[ "$INCLUDE_PRIVATE" == "1" ]]; then
  if ! grep -Eq '^[[:space:]]*content/agenda/\*\*[[:space:]]+filter=git-crypt' \
        "$REPO_ROOT/.gitattributes" 2>/dev/null; then
    echo "AGENDA|include-private-blocked|content/agenda/** not under git-crypt; refusing to inline private rows" >&2
    exit 5
  fi
fi

# --- Per-region builders ---

# Filter actions.tsv by status set (comma-separated). Returns TSV body
# lines (no header) on stdout.
filter_status() {
  local statuses="$1"
  awk -F'\t' -v st="$statuses" '
    BEGIN { n = split(st, a, ","); for (i = 1; i <= n; i++) keep[a[i]] = 1 }
    NR > 1 && ($2 in keep) { print }
  ' "$ACTIONS"
}

# Split rows from $1 (TSV body) into PUBLIC_ROWS (newline-joined,
# possibly empty) and PRIVATE_COUNT (integer). Source_kind is column 16.
privacy_partition() {
  local raw="$1"
  PRIVATE_COUNT=0
  PUBLIC_ROWS=""
  if [[ -z "$raw" ]]; then
    return 0
  fi
  # Use awk for the split — avoids per-row shell forks and handles tabs.
  local pub_tmp priv_count
  pub_tmp="$(mktemp)"
  priv_count="$(awk -F'\t' -v out="$pub_tmp" '
    {
      if ($16 == "private") { p++ }
      else                  { print > out }
    }
    END { print p+0 }
  ' <<<"$raw")"
  PRIVATE_COUNT="$priv_count"
  if [[ -s "$pub_tmp" ]]; then
    PUBLIC_ROWS="$(cat "$pub_tmp")"
  fi
  rm -f "$pub_tmp"
}

# Print the hidden-rows callout if N > 0.
emit_hidden_placeholder() {
  local n="$1"
  (( n == 0 )) && return 0
  printf '\n> _%d action(s) hidden — origin under private/encrypted path._\n' "$n"
}

# Format TSV rows from stdin into Markdown action bullets. Uses tab
# separator; columns documented in scripts/action-scan.sh.
format_action_line() {
  awk -F'\t' '
    NF == 0 { next }
    {
      line = "- [" $2 "] " $3
      if ($6  != "") line = line " " $6
      if ($7  != "") line = line " due:" $7
      if ($8  != "") line = line " defer:" $8
      if ($9  != "") {
        # The wait column may already include "[[...]]" wrapping (action-scan
        # stores the raw token form per spec). Strip if present, then re-wrap
        # so the rendered form is always a single set of brackets.
        w = $9
        sub(/^\[\[/, "", w); sub(/\]\]$/, "", w)
        line = line " wait:[[" w "]]"
      }
      if ($10 != "") line = line " since:" $10
      if ($11 != "") line = line " every:" $11
      if ($12 != "") line = line " done:" $12
      if ($13 != "") line = line " priority:" $13
      if ($14 != "") line = line " est:" $14
      if ($1  != "") line = line " ^" $1
      print line
    }
  '
}

# next-actions: every [ ] and [/] whose defer is empty OR <= today.
# Grouped by @context, then no-context bucket last.
build_next_actions() {
  local rows filtered
  rows="$(filter_status ' ,/')"
  privacy_partition "$rows"
  printf '## By Context\n\n'
  filtered="$(printf '%s\n' "$PUBLIC_ROWS" | awk -F'\t' -v today="$TODAY" '
    NF == 0 { next }
    $8 == "" || $8 <= today { print }
  ')"
  if [[ -n "$filtered" ]]; then
    local contexts
    contexts="$(printf '%s\n' "$filtered" | awk -F'\t' '$6 != "" {print $6}' | sort -u)"
    if [[ -n "$contexts" ]]; then
      while IFS= read -r ctx; do
        [[ -z "$ctx" ]] && continue
        printf '### %s\n\n' "$ctx"
        printf '%s\n' "$filtered" | awk -F'\t' -v c="$ctx" '$6 == c' \
          | format_action_line
        printf '\n'
      done <<< "$contexts"
    fi
    local no_ctx
    no_ctx="$(printf '%s\n' "$filtered" | awk -F'\t' 'NF > 0 && $6 == ""')"
    if [[ -n "$no_ctx" ]]; then
      printf '### (no context)\n\n'
      printf '%s\n' "$no_ctx" | format_action_line
      printf '\n'
    fi
  fi
  emit_hidden_placeholder "$PRIVATE_COUNT"
}

# today: status [ ]/[/] AND (due <= today OR defer <= today).
build_today() {
  local rows filtered
  rows="$(filter_status ' ,/')"
  privacy_partition "$rows"
  printf '## Today\n\n'
  filtered="$(printf '%s\n' "$PUBLIC_ROWS" | awk -F'\t' -v today="$TODAY" '
    NF == 0 { next }
    ($7 != "" && $7 <= today) || ($8 != "" && $8 <= today) { print }
  ')"
  if [[ -n "$filtered" ]]; then
    printf '%s\n' "$filtered" | format_action_line
    printf '\n'
  fi
  emit_hidden_placeholder "$PRIVATE_COUNT"
}

# waiting: every [?] grouped by wait person.
build_waiting() {
  local rows
  rows="$(filter_status '?')"
  privacy_partition "$rows"
  printf '## Waiting\n\n'
  if [[ -n "$PUBLIC_ROWS" ]]; then
    local persons
    persons="$(printf '%s\n' "$PUBLIC_ROWS" | awk -F'\t' '$9 != "" {print $9}' | sort -u)"
    if [[ -n "$persons" ]]; then
      while IFS= read -r p; do
        [[ -z "$p" ]] && continue
        printf '### %s\n\n' "$p"
        printf '%s\n' "$PUBLIC_ROWS" | awk -F'\t' -v p="$p" '$9 == p' \
          | format_action_line
        printf '\n'
      done <<< "$persons"
    fi
    local no_person
    no_person="$(printf '%s\n' "$PUBLIC_ROWS" | awk -F'\t' 'NF > 0 && $9 == ""')"
    if [[ -n "$no_person" ]]; then
      printf '### (no wait person)\n\n'
      printf '%s\n' "$no_person" | format_action_line
      printf '\n'
    fi
  fi
  emit_hidden_placeholder "$PRIVATE_COUNT"
}

# someday: every [>] grouped by project; "unassigned" bucket last.
build_someday() {
  local rows
  rows="$(filter_status '>')"
  privacy_partition "$rows"
  printf '## Someday\n\n'
  if [[ -n "$PUBLIC_ROWS" ]]; then
    local projects
    projects="$(printf '%s\n' "$PUBLIC_ROWS" | awk -F'\t' '$15 != "" {print $15}' | sort -u)"
    if [[ -n "$projects" ]]; then
      while IFS= read -r pj; do
        [[ -z "$pj" ]] && continue
        printf '### [[%s]]\n\n' "$pj"
        printf '%s\n' "$PUBLIC_ROWS" | awk -F'\t' -v p="$pj" '$15 == p' \
          | format_action_line
        printf '\n'
      done <<< "$projects"
    fi
    local unassigned
    unassigned="$(printf '%s\n' "$PUBLIC_ROWS" | awk -F'\t' 'NF > 0 && $15 == ""')"
    if [[ -n "$unassigned" ]]; then
      printf '### (unassigned)\n\n'
      printf '%s\n' "$unassigned" | format_action_line
      printf '\n'
    fi
  fi
  emit_hidden_placeholder "$PRIVATE_COUNT"
}

# stuck-projects: status: active project pages with zero open actions,
# OR last_updated > 14d ago with no [x] action since then.
build_stuck_projects() {
  printf '## Stuck projects\n\n'
  local proj_dir="$REPO_ROOT/content/projects"
  [[ -d "$proj_dir" ]] || return 0

  # Compute "today - 14 days" via Python (portable, no GNU/BSD date split).
  local cutoff
  cutoff="$(python3 -c '
import datetime, sys
print((datetime.date.today() - datetime.timedelta(days=14)).isoformat())
' 2>/dev/null || echo "")"

  local proj_file slug
  for proj_file in "$proj_dir"/*.md; do
    [[ -f "$proj_file" ]] || continue
    grep -Eq '^status:[[:space:]]*active[[:space:]]*$' "$proj_file" || continue
    slug="$(basename "$proj_file" .md)"
    local open_count
    open_count="$(awk -F'\t' -v s="$slug" '
      NR > 1 && $15 == s && ($2 == " " || $2 == "/") { n++ }
      END { print n + 0 }
    ' "$ACTIONS")"

    if (( open_count == 0 )); then
      printf -- '- [[%s]] — no open actions\n' "$slug"
      continue
    fi

    if [[ -n "$cutoff" ]]; then
      local last_upd
      last_upd="$(awk '
        BEGIN { fm = 0 }
        /^---[[:space:]]*$/ { fm = !fm; next }
        fm && /^last_updated:/ {
          v = $0
          sub(/^last_updated:[[:space:]]*/, "", v)
          gsub(/[[:space:]]/, "", v)
          print v
          exit
        }
      ' "$proj_file")"
      if [[ -n "$last_upd" && "$last_upd" < "$cutoff" ]]; then
        local done_since
        done_since="$(awk -F'\t' -v s="$slug" -v d="$last_upd" '
          NR > 1 && $15 == s && $2 == "x" && $12 != "" && $12 > d { n++ }
          END { print n + 0 }
        ' "$ACTIONS")"
        if (( done_since == 0 )); then
          printf -- '- [[%s]] — last_updated %s, no [x] since\n' \
            "$slug" "$last_upd"
        fi
      fi
    fi
  done
}

# --- Atomic rewrite of one managed region ---
#
# Reads the current $f, replaces the body between the per-region
# BEGIN/END markers with the contents of $body_file, and rewrites
# the `last_updated:` frontmatter line to $TODAY. Writes to a
# sibling .tmp.<pid> file then mv-renames atomically.
rewrite_region() {
  local region="$1" body_file="$2"
  local f="$AGENDA_DIR/${region}.md"
  if [[ ! -f "$f" ]]; then
    echo "AGENDA|missing-page|$f" >&2
    return 0
  fi
  local tmp="$f.tmp.$$"
  awk \
    -v region="$region" \
    -v body_file="$body_file" \
    -v today="$TODAY" '
    BEGIN {
      in_fm = 0
      fm_open = 0
      fm_done = 0
      in_block = 0
      begin_marker = "<!-- BEGIN agenda:" region " -->"
      end_marker   = "<!-- END agenda:"   region " -->"
    }
    {
      # Frontmatter tracking: first --- opens, second --- closes.
      if (!fm_done && $0 ~ /^---[[:space:]]*$/) {
        if (fm_open == 0) { fm_open = 1; print; next }
        else              { fm_open = 0; fm_done = 1; print; next }
      }
      if (fm_open == 1 && $0 ~ /^last_updated:/) {
        print "last_updated: " today
        next
      }
      if ($0 == begin_marker) {
        print
        # Stream the body file verbatim, then a trailing blank line for
        # readability (regenerated content + user-content separator).
        while ((getline ln < body_file) > 0) print ln
        close(body_file)
        in_block = 1
        next
      }
      if ($0 == end_marker) {
        in_block = 0
        print
        next
      }
      if (in_block == 1) next
      print
    }
  ' "$f" > "$tmp"
  mv "$tmp" "$f"
}

# Build the five region bodies into temp files, then atomic-rewrite
# each page. We collect bodies eagerly (under the lock) so that a
# regen is a single atomic snapshot of actions.tsv state.
do_regen() {
  local body_dir
  body_dir="$(mktemp -d)"
  trap 'rm -rf "$body_dir"' RETURN

  build_next_actions   > "$body_dir/next-actions"
  build_today          > "$body_dir/today"
  build_waiting        > "$body_dir/waiting"
  build_someday        > "$body_dir/someday"
  build_stuck_projects > "$body_dir/stuck-projects"

  rewrite_region next-actions   "$body_dir/next-actions"
  rewrite_region today          "$body_dir/today"
  rewrite_region waiting        "$body_dir/waiting"
  rewrite_region someday        "$body_dir/someday"
  rewrite_region stuck-projects "$body_dir/stuck-projects"
}

# Export the bits do_regen needs so they survive the bash -c subshell
# spawned by awiki_lock_with.
export REPO_ROOT ACTIONS AGENDA_DIR TODAY PRIVATE_COUNT PUBLIC_ROWS
export -f filter_status privacy_partition emit_hidden_placeholder \
          format_action_line build_next_actions build_today build_waiting \
          build_someday build_stuck_projects rewrite_region do_regen

awiki_lock_with --timeout="$AWIKI_LOCK_TIMEOUT_USER" -- bash -c 'do_regen' \
  || {
    rc=$?
    if [[ "$rc" == "7" ]]; then
      echo "AGENDA|lock-timeout" >&2
    fi
    exit "$rc"
  }

echo "agenda regenerated: 5 regions, last_updated=$TODAY"
exit 0
