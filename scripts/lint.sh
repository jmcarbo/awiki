#!/usr/bin/env bash
set -uo pipefail

FIX=0
ONLY=""
ONLY_FILE=""
HUGO_CHECK=0
CONTENT_DIR="content"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --fix) FIX=1; shift ;;
    --only=*) ONLY="${1#--only=}"; shift ;;
    --file=*) ONLY_FILE="${1#--file=}"; shift ;;
    --hugo-check) HUGO_CHECK=1; shift ;;
    --) shift; break ;;
    *) CONTENT_DIR="$1"; shift ;;
  esac
done

# Resolve repo root + scripts dir for task-rule helpers.
AWIKI_SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AWIKI_REPO_ROOT="${AWIKI_REPO_ROOT:-$(pwd)}"

# Source synth lint extension (always available; runs only if --only=synth or
# default mode includes synth pages).
if [[ -f "$(dirname "$0")/lint-synth.sh" ]]; then
  # shellcheck disable=SC1091
  source "$(dirname "$0")/lint-synth.sh"
elif [[ -f scripts/lint-synth.sh ]]; then
  # shellcheck disable=SC1091
  source scripts/lint-synth.sh
fi

apply_fixes() {
  local page="$1"
  if ! grep -q '^last_updated:' "$page"; then
    local today
    today="$(date '+%Y-%m-%d')"
    # Portable insertion: use awk (handles BSD/GNU sed differences in \n).
    awk -v today="$today" '
      /^date: / { print; print "last_updated: " today; next }
      { print }
    ' "$page" > "$page.tmp"
    if cmp -s "$page.tmp" "$page"; then
      # awk pass was a no-op (no `date:` line to anchor on); leave file untouched.
      rm -f "$page.tmp"
    else
      mv "$page.tmp" "$page"
      echo "FIX|$page|added last_updated: $today"
    fi
  fi
}

if [[ "$FIX" -eq 1 && ( -z "$ONLY" || "$ONLY" = "all" ) ]]; then
  while IFS= read -r -d '' page; do
    apply_fixes "$page"
  done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)
fi

# --- Synth --fix: S3 normalization steps 2/3/4 inside markers only ----------
if [[ "$FIX" -eq 1 && ( -z "$ONLY" || "$ONLY" = "synth" || "$ONLY" = "all" ) ]]; then
  if [[ -n "$ONLY_FILE" ]]; then
    if [[ -f "$ONLY_FILE" ]]; then
      python3 "$(dirname "$0")/lint-synth-fix-region.py" -- "$ONLY_FILE"
      echo "FIX|$ONLY_FILE|synth normalization (steps 2/3/4) applied inside markers"
    fi
  else
    while IFS= read -r -d '' page; do
      head -40 "$page" | grep -q '^type: synthesis$' || continue
      head -40 "$page" | grep -qE '^plugin: [a-z][a-z0-9-]*$' || continue
      python3 "$(dirname "$0")/lint-synth-fix-region.py" -- "$page"
      echo "FIX|$page|synth normalization (steps 2/3/4) applied inside markers"
    done < <(find "$CONTENT_DIR/synthesis" -maxdepth 2 -name '*.md' -type f -print0 2>/dev/null)
  fi
fi

# ============================================================================
# Task-layer lint rules (phase 17: T1-T7, T14, T15) + --fix mechanical fixes.
# ============================================================================
#
# Implementation notes / deviations from the plan body in
# docs/superpowers/plans/2026-04-27-phase-17-task-scanner-agenda.md (Task 17.6):
#
#  * T6 reads the alias map (`@<slug>` -> entry) from `.awiki/maps/alias-to-slug.tsv`
#    and only fires on action-grammar lines (sourced from actions.tsv $6=context).
#    Prose `@mentions` and code-block `- [ ] ...` content never reach actions.tsv
#    because action-scan.sh already excludes them.
#  * T14 has to scan source files directly because the scanner discards
#    malformed `^id` tokens (they don't end up in actions.tsv at all). The plan
#    code that read `$1` of actions.tsv missed those.
#  * T7 uses a content-shape check: any non-blank line inside the
#    `<!-- BEGIN agenda:X -->` / `<!-- END agenda:X -->` span that isn't an
#    action-line, heading, blockquote, blank, or HTML comment is a hand-edit.
#    Plan's git-diff approach can't catch a hand-edit that exists in the
#    initial commit (the test fixture is exactly that case).
#  * `awk patsplit` and 4-arg `match` are gawk-only — the plan's date-norm and
#    T7-hunk-overlap awk doesn't run on macOS BSD awk. Replaced with pure
#    POSIX-awk token scans.
#  * `--fix` skips files that have a `continuation` rejection in
#    actions-rejected.tsv (T15 risk). Modifying such a file could append
#    `since:` / `done:` / mint IDs onto wrapped intent — the plan flagged this
#    risk in prose ("silent-data-loss") but the code didn't enforce it.
#  * Mint walks the actions.tsv rows where `$1==""` (action-grammar parsed but
#    no id) AND falls back to a fence-aware scan of bare `- [ ] text` lines for
#    files not yet in actions.tsv (e.g., a freshly written page invoked via
#    `lint --fix` without running scan first).

awiki_lint_run_task_rules() {
  # shellcheck disable=SC1091
  . "$AWIKI_SCRIPTS_DIR/lib/action-grammar.sh"

  local actions_tsv="$AWIKI_REPO_ROOT/.awiki/maps/actions.tsv"
  local rejected_tsv="$AWIKI_REPO_ROOT/.awiki/maps/actions-rejected.tsv"
  local alias_map="$AWIKI_REPO_ROOT/.awiki/maps/alias-to-slug.tsv"

  awiki_lint_task_rule_T1   "$rejected_tsv"
  awiki_lint_task_rule_T2   "$actions_tsv"
  awiki_lint_task_rule_T3T4 "$actions_tsv" "$rejected_tsv"
  awiki_lint_task_rule_T5   "$actions_tsv"
  awiki_lint_task_rule_T6   "$actions_tsv" "$alias_map"
  awiki_lint_task_rule_T7
  awiki_lint_task_rule_T8   "$actions_tsv"
  awiki_lint_task_rule_T14
  awiki_lint_task_rule_T15  "$rejected_tsv"
}

awiki_lint_task_rule_T1() {
  local rej="$1"
  [[ -f "$rej" ]] || return 0
  awk -F'\t' '$3=="bad-status" {
    printf "LINT|ERROR|%s|T1: bad status marker on line %d (%s)\n", $1, $2, $5
  }' "$rej"
}

awiki_lint_task_rule_T2() {
  local act="$1"
  [[ -f "$act" ]] || return 0
  # Cross-page: same literal id (including any `~<n>`) on >=2 rows.
  # Chain head `^a05` + chain instance `^a05~2` are different literals, so
  # they coexist fine. Two `^a05~2` on different pages = error.
  awk -F'\t' '
    NR>1 && $1!="" {
      count[$1]++
      if (where[$1] != "") where[$1] = where[$1] ", "
      where[$1] = where[$1] $4 ":" $5
    }
    END {
      for (id in count) if (count[id] > 1)
        printf "LINT|ERROR|(multi)|T2: duplicate id ^%s at %s\n", id, where[id]
    }
  ' "$act"
}

awiki_lint_task_rule_T3T4() {
  local act="$1" rej="$2"
  if [[ -f "$rej" ]]; then
    awk -F'\t' '
      $3=="bad-key"  { printf "LINT|ERROR|%s|T3: unrecognized tail key on line %d (%s)\n", $1, $2, $5 }
      $3=="bad-date" { printf "LINT|ERROR|%s|T4: invalid date on line %d (%s)\n", $1, $2, $5 }
    ' "$rej"
  fi
  # T3: scan action lines in content files directly for `<word>:<value>`
  # tokens with unknown keys. The scanner+grammar drop them silently into
  # the line's text portion (no bad-key reason emitted), so actions-rejected
  # is not authoritative for T3.
  local content_dir="$AWIKI_REPO_ROOT/content"
  if [[ -d "$content_dir" ]]; then
    while IFS= read -r f; do
      [[ -f "$f" ]] || continue
      awk '
        BEGIN { in_fence = 0; in_fm = 0; fm_done = 0
                allowed["due"] = 1; allowed["defer"] = 1; allowed["wait"] = 1
                allowed["since"] = 1; allowed["every"] = 1; allowed["done"] = 1
                allowed["priority"] = 1; allowed["est"] = 1
        }
        /^---[[:space:]]*$/ {
          if (!fm_done) {
            if (!in_fm) in_fm = 1
            else        { in_fm = 0; fm_done = 1 }
          }
          next
        }
        in_fm { next }
        /^```/ { in_fence = !in_fence; next }
        in_fence { next }
        /^[[:space:]]*>[[:space:]]/ { next }
        /^[[:space:]]*-[[:space:]]+\[.\][[:space:]]+/ {
          # Strip trailing block-id, then walk tokens.
          line = $0
          sub(/[[:space:]]+\^[a-z0-9]+(~[0-9]+)?[[:space:]]*$/, "", line)
          n = split(line, parts, /[[:space:]]+/)
          for (i = 1; i <= n; i++) {
            tok = parts[i]
            if (tok == "" || tok == "-") continue
            # Skip checkbox shape `[x]`.
            if (tok ~ /^\[.\]$/) continue
            # Skip @context.
            if (tok ~ /^@/) continue
            # Skip wikilinks.
            if (tok ~ /^\[\[/) continue
            # Token shape <word>:<value> with non-empty value.
            if (tok ~ /^[a-zA-Z][a-zA-Z0-9_-]*:.+/) {
              key = tok
              sub(/:.*/, "", key)
              if (!(key in allowed)) {
                printf "LINT|ERROR|%s|T3: unrecognized tail key %s on line %d\n", FILENAME, key, NR
              }
            }
          }
        }
      ' "$f"
    done < <(find "$content_dir" -type f -name '*.md' 2>/dev/null | sort)
  fi
  # Calendar-validate date columns in actions.tsv (catches values that pass
  # the YYYY-MM-DD shape regex but are not real dates, e.g. 2026-13-40 — the
  # grammar's tail splitter caught this and rejected, but a paranoid second
  # pass over actions.tsv catches anything that slipped through, e.g. a
  # synthetic actions.tsv row that wasn't filtered).
  [[ -f "$act" ]] || return 0
  awk -F'\t' '
    function bad_date(v,    y, m, d, maxday) {
      if (v == "") return 0
      if (v !~ /^[0-9]{4}-[0-9]{2}-[0-9]{2}$/) return 1
      y = substr(v,1,4)+0; m = substr(v,6,2)+0; d = substr(v,9,2)+0
      if (m < 1 || m > 12) return 1
      maxday = 31
      if (m==4 || m==6 || m==9 || m==11) maxday = 30
      else if (m==2) {
        if ((y%4==0 && y%100!=0) || y%400==0) maxday = 29
        else                                  maxday = 28
      }
      return (d < 1 || d > maxday)
    }
    NR>1 {
      if (bad_date($7))  printf "LINT|ERROR|%s|T4: invalid due:%s on line %d\n",   $4, $7,  $5
      if (bad_date($8))  printf "LINT|ERROR|%s|T4: invalid defer:%s on line %d\n", $4, $8,  $5
      if (bad_date($10)) printf "LINT|ERROR|%s|T4: invalid since:%s on line %d\n", $4, $10, $5
      if (bad_date($12)) printf "LINT|ERROR|%s|T4: invalid done:%s on line %d\n",  $4, $12, $5
    }
  ' "$act"
}

awiki_lint_task_rule_T5() {
  local act="$1"
  [[ -f "$act" ]] || return 0
  # Column 9 in actions.tsv is `wait`. A `[?]` line with empty wait → T5.
  awk -F'\t' '
    NR>1 && $2=="?" && $9=="" {
      printf "LINT|ERROR|%s|T5: waiting line missing wait:[[person]] (line %d)\n", $4, $5
    }
  ' "$act"
}

awiki_lint_task_rule_T6() {
  local act="$1" alias_map="$2"
  [[ -f "$act" ]] || return 0
  [[ -f "$alias_map" ]] || return 0
  # For every action-grammar row with a non-empty context, check that the
  # context @-alias appears in the alias map. T6 is action-grammar-scoped:
  # actions.tsv only contains lines that the scanner identified as actions
  # (so prose @mentions and code-block content are already excluded).
  awk -F'\t' -v alias_map="$alias_map" '
    BEGIN {
      while ((getline line < alias_map) > 0) {
        n = split(line, p, "\t")
        if (n >= 1 && p[1] ~ /^@/) ctxset[p[1]] = 1
      }
      close(alias_map)
    }
    NR>1 && $6 != "" {
      if (!($6 in ctxset)) {
        printf "LINT|ERROR|%s|T6: unknown context %s (line %d)\n", $4, $6, $5
      }
    }
  ' "$act"
}

awiki_lint_task_rule_T7() {
  # Walk all content/*.md files; for each that contains a managed-region
  # marker, scan within the BEGIN..END span. Any non-blank line that does NOT
  # match the agenda-output shape (action lines, headings, blockquotes,
  # blanks, HTML comments) is a hand-edit.
  local content_dir="$AWIKI_REPO_ROOT/content"
  [[ -d "$content_dir" ]] || return 0
  while IFS= read -r f; do
    [[ -f "$f" ]] || continue
    grep -q '<!-- BEGIN agenda:' "$f" 2>/dev/null || continue
    awk '
      BEGIN { in_region = 0; region = "" }
      {
        if (match($0, /^<!-- BEGIN agenda:([a-z-]+) -->$/)) {
          in_region = 1
          # Extract region name (substring between "agenda:" and " -->").
          line = $0
          sub(/^<!-- BEGIN agenda:/, "", line)
          sub(/ -->$/, "", line)
          region = line
          next
        }
        if (in_region && $0 ~ /^<!-- END agenda:/) {
          in_region = 0
          region = ""
          next
        }
        if (in_region) {
          # Allowed shapes inside an agenda-managed region:
          #   - blank lines
          #   - action lines: ^- \[.\]
          #   - headings: ^#+ space
          #   - hidden-count blockquote: ^> _
          #   - HTML comments (defensive)
          if ($0 ~ /^[[:space:]]*$/) next
          if ($0 ~ /^[[:space:]]*-[[:space:]]+\[.\][[:space:]]/) next
          if ($0 ~ /^#+[[:space:]]/) next
          if ($0 ~ /^>[[:space:]]/) next
          if ($0 ~ /^<!--/) next
          printf "LINT|ERROR|%s|T7: hand-edit detected inside agenda:%s region (line %d)\n", FILENAME, region, NR
        }
      }
    ' "$f"
  done < <(find "$content_dir" -type f -name '*.md' 2>/dev/null | sort)
}

# T8: type:project, status:active page with zero open [ ]/[/] action lines.
# Emits LINT|WARN. Exempt: _loose.md, _someday.md (catch-all pages by spec
# convention), and any project page whose status is `someday` or `done`.
awiki_lint_task_rule_T8() {
  local act="$1"
  local content_dir="$AWIKI_REPO_ROOT/content"
  [[ -d "$content_dir" ]] || return 0

  # Build the set of project files (relative path) that have at least one
  # open action ([ ] or [/]). Read from actions.tsv if present; otherwise
  # the set is empty (every active project will be flagged).
  declare -A has_open=()
  if [[ -f "$act" ]]; then
    local _id status _text file _rest
    while IFS=$'\t' read -r _id status _text file _rest; do
      [[ "$file" == "file" ]] && continue
      case "$status" in
        ' '|'/') has_open["$file"]=1 ;;
      esac
    done < <(tail -n +2 "$act")
  fi

  while IFS= read -r f; do
    [[ -f "$f" ]] || continue
    local base; base="$(basename "$f")"
    case "$base" in
      _loose.md|_someday.md|_index.md) continue ;;
    esac
    local relpath="${f#"$AWIKI_REPO_ROOT/"}"
    local fm_type fm_status
    fm_type="$(awiki_frontmatter_value "$f" type)"
    [[ "$fm_type" == "project" ]] || continue
    fm_status="$(awiki_frontmatter_value "$f" status)"
    # Only flag explicitly-active project pages. Pages without a status:
    # frontmatter value (or status:someday|done) are exempt: they are not
    # claiming to be in the active-work pool.
    [[ "$fm_status" == "active" ]] || continue
    if [[ -z "${has_open[$relpath]:-}" ]]; then
      printf 'LINT|WARN|%s|T8: type:project, status:active page has zero open [ ]/[/] actions\n' \
        "$relpath"
    fi
  done < <(find "$content_dir" -type f -name '*.md' 2>/dev/null | sort)
}

awiki_lint_task_rule_T14() {
  # T14 must scan source files directly: malformed `^id` tokens (`^id` too
  # short, `^abc~zz` wrong chain shape) are silently dropped by the scanner
  # so they do not appear in actions.tsv with a value to lint against.
  local content_dir="$AWIKI_REPO_ROOT/content"
  [[ -d "$content_dir" ]] || return 0
  while IFS= read -r f; do
    [[ -f "$f" ]] || continue
    awk '
      BEGIN { in_fence = 0; in_fm = 0; fm_done = 0 }
      /^---[[:space:]]*$/ {
        if (!fm_done) {
          if (!in_fm) in_fm = 1
          else        { in_fm = 0; fm_done = 1 }
        }
        next
      }
      in_fm { next }
      /^```/ { in_fence = !in_fence; next }
      in_fence { next }
      /^[[:space:]]*>[[:space:]]/ { next }
      # Action-line shape only; T14 is action-grammar-scoped.
      /^[[:space:]]*-[[:space:]]+\[.\][[:space:]]/ {
        # Find every `^token` in the line (anchored by space-then-^ or start).
        line = $0
        while (match(line, /(^|[[:space:]])\^[^[:space:]]+/)) {
          tok = substr(line, RSTART, RLENGTH)
          # Strip leading whitespace and the `^`.
          sub(/^[[:space:]]*\^/, "", tok)
          # Validate: chain shape <base>~<digits> or plain <base>.
          if (tok ~ /~/) {
            if (tok !~ /^[a-z0-9]{3,16}~[0-9]+$/) {
              printf "LINT|ERROR|%s|T14: bad id shape ^%s on line %d (chain shape requires base~digits)\n", FILENAME, tok, NR
            }
          } else if (tok !~ /^[a-z0-9]{3,16}$/) {
            printf "LINT|ERROR|%s|T14: bad id shape ^%s on line %d\n", FILENAME, tok, NR
          }
          line = substr(line, RSTART + RLENGTH)
        }
      }
    ' "$f"
  done < <(find "$content_dir" -type f -name '*.md' 2>/dev/null | sort)
}

awiki_lint_task_rule_T15() {
  local rej="$1"
  [[ -f "$rej" ]] || return 0
  awk -F'\t' '$3=="continuation" {
    printf "LINT|WARN|%s|T15: action continuation on line %d not supported (multi-line action)\n", $1, $2
  }' "$rej"
}

# --- --fix mechanical fixes (task-aware) -----------------------------------

awiki_lint_fix_task_rules() {
  # shellcheck disable=SC1091
  . "$AWIKI_SCRIPTS_DIR/lib/action-grammar.sh"

  local content_dir="$AWIKI_REPO_ROOT/content"
  [[ -d "$content_dir" ]] || return 0

  # Files that contain a continuation rejection are skipped wholesale —
  # auto-rewriting wrapped intent risks silent data loss. T15 documents the
  # caveat; the user resolves manually.
  local skip_set=""
  local rejected_tsv="$AWIKI_REPO_ROOT/.awiki/maps/actions-rejected.tsv"
  if [[ -f "$rejected_tsv" ]]; then
    skip_set="$(awk -F'\t' '$3=="continuation" {print $1}' "$rejected_tsv" | sort -u)"
  fi

  awiki_lint_fix_task_walk "$skip_set"
}

# Returns 0 if relpath is in the skip-set (newline-separated), 1 otherwise.
awiki_lint_fix_task_in_skipset() {
  local relpath="$1" skip_set="$2"
  [[ -z "$skip_set" ]] && return 1
  local s
  while IFS= read -r s; do
    [[ -z "$s" ]] && continue
    [[ "$s" == "$relpath" ]] && return 0
  done <<< "$skip_set"
  return 1
}

awiki_lint_fix_task_walk() {
  local skip_set="$1"
  local today; today="$(date '+%Y-%m-%d')"

  while IFS= read -r f; do
    [[ -f "$f" ]] || continue
    local relpath="${f#"$AWIKI_REPO_ROOT/"}"
    if awiki_lint_fix_task_in_skipset "$relpath" "$skip_set"; then
      continue
    fi
    awiki_lint_fix_one_file "$f" "$today"
  done < <(find "$AWIKI_REPO_ROOT/content" -type f -name '*.md' 2>/dev/null | sort)
}

# Apply all task-aware --fix passes to a single file. Each pass is a separate
# awk invocation so the failure modes are isolated.
awiki_lint_fix_one_file() {
  local f="$1" today="$2"
  local tmp="$f.tmp.$$"

  # Pass 1: per-action-line normalization.
  #   * normalize date format (2026/4/27 -> 2026-04-27) inside due/defer/since/done.
  #   * auto-stamp done: on [x] lines lacking it.
  #   * auto-stamp since: on [?] lines lacking it.
  #   * sort tail keys to canonical order: @ctx due defer wait since every priority est done.
  #   * mint missing ^id (8-char base32, collision-checked against actions.tsv).
  awk -v today="$today" -v actions_tsv="$AWIKI_REPO_ROOT/.awiki/maps/actions.tsv" '
    function pad2(n)  { n = n + 0; return (n < 10 ? "0" n : "" n) }
    function normalize_date(v,    p, n) {
      if (v ~ /^[0-9]{4}\/[0-9]{1,2}\/[0-9]{1,2}$/) {
        n = split(v, p, "/")
        return p[1] "-" pad2(p[2]) "-" pad2(p[3])
      }
      return v
    }
    function load_used(    line, parts) {
      if (used_loaded) return
      used_loaded = 1
      while ((getline line < actions_tsv) > 0) {
        split(line, parts, "\t")
        if (parts[1] != "" && parts[1] != "id") used[parts[1]] = 1
      }
      close(actions_tsv)
      srand()
    }
    function mint_id(    n, alpha, c, cand, ch) {
      load_used()
      alpha = "abcdefghijklmnopqrstuvwxyz0123456789"
      for (n = 0; n < 5; n++) {
        cand = ""
        for (c = 0; c < 8; c++) {
          ch = int(rand() * 36) + 1
          cand = cand substr(alpha, ch, 1)
        }
        if (!(cand in used)) {
          used[cand] = 1
          return cand
        }
      }
      return ""
    }
    function fix_action_line(line,    head, body, id_tok, rest, n, parts, i,
                                       tok, ctx, due, defer, wait_, since, every,
                                       prio, est, done_, text, status, out) {
      # Match action-line shape: leading dash + checkbox.
      if (match(line, /^[[:space:]]*-[[:space:]]+\[.\][[:space:]]+/) == 0) {
        return line
      }
      head = substr(line, 1, RLENGTH)
      rest = substr(line, RLENGTH + 1)
      status = head
      sub(/^[[:space:]]*-[[:space:]]+\[/, "", status)
      sub(/\][[:space:]]+$/, "", status)

      # Strip trailing block-id (will re-emit at end).
      id_tok = ""
      if (match(rest, /(^|[[:space:]])\^[a-z0-9]{3,16}(~[0-9]+)?[[:space:]]*$/)) {
        id_tok = substr(rest, RSTART, RLENGTH)
        sub(/^[[:space:]]+/, "", id_tok)
        rest = substr(rest, 1, RSTART - 1)
        sub(/[[:space:]]+$/, "", rest)
      }

      n = split(rest, parts, /[[:space:]]+/)
      ctx = ""; due = ""; defer = ""; wait_ = ""; since = ""
      every = ""; prio = ""; est = ""; done_ = ""
      text = ""
      for (i = 1; i <= n; i++) {
        tok = parts[i]
        if (tok == "") continue
        if (tok ~ /^@[a-z0-9][a-z0-9-]*$/ && ctx == "") { ctx = tok; continue }
        if (tok ~ /^due:/)      { sub(/^due:/, "",     tok); due   = "due:"   normalize_date(tok); continue }
        if (tok ~ /^defer:/)    { sub(/^defer:/, "",   tok); defer = "defer:" normalize_date(tok); continue }
        if (tok ~ /^wait:/)     { wait_ = tok;  continue }
        if (tok ~ /^since:/)    { sub(/^since:/, "",   tok); since = "since:" normalize_date(tok); continue }
        if (tok ~ /^every:/)    { every = tok;  continue }
        if (tok ~ /^priority:/) { prio  = tok;  continue }
        if (tok ~ /^est:/)      { est   = tok;  continue }
        if (tok ~ /^done:/)     { sub(/^done:/, "",    tok); done_ = "done:"  normalize_date(tok); continue }
        text = (text == "" ? tok : text " " tok)
      }
      # Auto-stamp.
      if (status == "x" && done_ == "") done_ = "done:" today
      if (status == "?" && since == "") since = "since:" today
      # Mint missing id.
      if (id_tok == "") {
        m = mint_id()
        if (m != "") id_tok = "^" m
      }
      out = head text
      if (ctx   != "") out = out " " ctx
      if (due   != "") out = out " " due
      if (defer != "") out = out " " defer
      if (wait_ != "") out = out " " wait_
      if (since != "") out = out " " since
      if (every != "") out = out " " every
      if (prio  != "") out = out " " prio
      if (est   != "") out = out " " est
      if (done_ != "") out = out " " done_
      if (id_tok != "") out = out " " id_tok
      return out
    }
    BEGIN { in_fence = 0; in_fm = 0; fm_done = 0; used_loaded = 0 }
    /^---[[:space:]]*$/ {
      if (!fm_done) {
        if (!in_fm) in_fm = 1
        else        { in_fm = 0; fm_done = 1 }
      }
      print; next
    }
    in_fm { print; next }
    /^```/ { in_fence = !in_fence; print; next }
    in_fence { print; next }
    /^[[:space:]]*>[[:space:]]/ { print; next }
    /^[[:space:]]*-[[:space:]]+\[.\][[:space:]]+/ {
      print fix_action_line($0); next
    }
    { print }
  ' "$f" > "$tmp"

  if ! cmp -s "$f" "$tmp"; then
    mv "$tmp" "$f"
  else
    rm -f "$tmp"
  fi
}

# ============================================================================
ERRORS=0
WARNS=0
INFOS=0

# --- Task-rule --fix (runs before lint pass so task rules see fixed state) --
if [[ "$FIX" -eq 1 && ( -z "$ONLY" || "$ONLY" = "task" || "$ONLY" = "all" ) ]]; then
  awiki_lint_fix_task_rules
fi

# --- Task-rule lint dispatch -----------------------------------------------
if [[ -z "$ONLY" || "$ONLY" = "task" || "$ONLY" = "all" ]]; then
  task_out="$(awiki_lint_run_task_rules || true)"
  if [[ -n "$task_out" ]]; then
    printf '%s\n' "$task_out"
    while IFS= read -r tline; do
      [[ -z "$tline" ]] && continue
      case "$tline" in
        LINT\|ERROR\|*) ERRORS=$((ERRORS + 1)) ;;
        LINT\|WARN\|*)  WARNS=$((WARNS + 1)) ;;
        LINT\|INFO\|*)  INFOS=$((INFOS + 1)) ;;
      esac
    done <<< "$task_out"
  fi
fi

if [[ -z "$ONLY" || "$ONLY" = "all" ]]; then

declare -A SLUG_TO_PATH
declare -A ALIAS_TO_SLUG
declare -A ALIAS_COUNT

while IFS= read -r -d '' page; do
  slug="$(basename "$page" .md)"
  # Skip section indexes: every section has its own _index.md, so they all
  # share the slug "_index". Mirrors the same skip in scripts/build.sh.
  if [[ "$slug" == "_index" ]]; then
    continue
  fi
  if [[ -n "${SLUG_TO_PATH[$slug]:-}" ]]; then
    echo "LINT|ERROR|$page|duplicate slug: $slug also at ${SLUG_TO_PATH[$slug]}"
    ERRORS=$((ERRORS + 1))
  fi
  SLUG_TO_PATH[$slug]="$page"

  in_fm=0
  while IFS= read -r line; do
    [[ "$line" == "---" ]] && { in_fm=$((in_fm + 1)); continue; }
    [[ "$in_fm" -ge 2 ]] && break
    if [[ "$line" =~ ^aliases:[[:space:]]*\[(.*)\][[:space:]]*$ ]]; then
      raw="${BASH_REMATCH[1]}"
      IFS=',' read -ra parts <<< "$raw"
      for p in "${parts[@]}"; do
        a="$(echo "$p" | sed -E 's/^[ "'\'']+|[ "'\'']+$//g')"
        [[ -z "$a" ]] && continue
        ALIAS_COUNT[$a]=$((${ALIAS_COUNT[$a]:-0} + 1))
        ALIAS_TO_SLUG[$a]="$slug"
      done
    fi
  done < "$page"
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

while IFS= read -r -d '' page; do
  # Section indexes (_index.md) are landing pages with intentionally short bodies;
  # exempt them from empty-page and per-page lint checks below.
  if [[ "$(basename "$page")" == "_index.md" ]]; then
    continue
  fi
  body_len=$(awk '/^---$/{c++; next} c==2{print}' "$page" | wc -c | tr -d ' ')
  if [[ "$body_len" -lt 50 ]]; then
    echo "LINT|WARN|$page|empty page (<50 char body)"
    WARNS=$((WARNS + 1))
  fi

  # Parse the tags: line as a YAML list, check for an exact 'private' token.
  has_private_tag=0
  while IFS= read -r line; do
    if [[ "$line" =~ ^tags:[[:space:]]*\[(.*)\][[:space:]]*$ ]]; then
      raw="${BASH_REMATCH[1]}"
      IFS=',' read -ra parts <<< "$raw"
      for p in "${parts[@]}"; do
        token="$(echo "$p" | sed -E 's/^[ "'\'']+|[ "'\'']+$//g')"
        if [[ "$token" == "private" ]]; then has_private_tag=1; fi
      done
    fi
  done < "$page"

  if [[ "$has_private_tag" -eq 1 && "$page" != *"/private/"* ]]; then
    echo "LINT|WARN|$page|private tag outside private path"
    WARNS=$((WARNS + 1))
  fi

  while read -r link; do
    target="${link%%|*}"
    if [[ -z "${SLUG_TO_PATH[$target]:-}" && -z "${ALIAS_TO_SLUG[$target]:-}" ]]; then
      echo "LINT|ERROR|$page|broken wikilink: [[$target]]"
      ERRORS=$((ERRORS + 1))
    fi
  done < <(grep -oE '\[\[[^]]+\]\]' "$page" | sed -E 's/^\[\[|\]\]$//g')
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

for a in "${!ALIAS_COUNT[@]}"; do
  if [[ "${ALIAS_COUNT[$a]}" -gt 1 ]]; then
    echo "LINT|ERROR|content|alias collision: '$a' used by multiple pages"
    ERRORS=$((ERRORS + 1))
  fi
done

# Orphan check: count inbound wikilinks per slug; warn if zero.
declare -A INBOUND
while IFS= read -r -d '' page; do
  while read -r link; do
    target="${link%%|*}"
    INBOUND[$target]=$((${INBOUND[$target]:-0} + 1))
    # Also credit the alias's resolved slug, if any
    resolved="${ALIAS_TO_SLUG[$target]:-}"
    if [[ -n "$resolved" ]]; then
      INBOUND[$resolved]=$((${INBOUND[$resolved]:-0} + 1))
    fi
  done < <(grep -oE '\[\[[^]]+\]\]' "$page" | sed -E 's/^\[\[|\]\]$//g')
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

while IFS= read -r -d '' page; do
  slug="$(basename "$page" .md)"
  type_field="$(awk -F'[:[:space:]]+' '/^type:/{print $2; exit}' "$page" | tr -d '"')"
  if [[ "$type_field" =~ ^(log|catalog|section-index)$ ]]; then
    continue
  fi
  if [[ "${INBOUND[$slug]:-0}" -eq 0 ]]; then
    echo "LINT|INFO|$page|orphan: no inbound wikilinks"
    INFOS=$((INFOS + 1))
  fi
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

# Catalog cross-link check
CATALOG="$CONTENT_DIR/catalog.md"
if [[ -f "$CATALOG" ]]; then
  declare -A IN_CATALOG
  while read -r link; do
    target="${link%%|*}"
    IN_CATALOG[$target]=1
  done < <(grep -oE '\[\[[^]]+\]\]' "$CATALOG" | sed -E 's/^\[\[|\]\]$//g')

  while IFS= read -r -d '' page; do
    slug="$(basename "$page" .md)"
    [[ "$slug" =~ ^(_index|catalog|log)$ ]] && continue
    type_field="$(awk -F'[:[:space:]]+' '/^type:/{print $2; exit}' "$page" | tr -d '"')"
    if [[ "$type_field" =~ ^(log|catalog|section-index)$ ]]; then
      continue
    fi
    if [[ -z "${IN_CATALOG[$slug]:-}" ]]; then
      echo "LINT|WARN|$page|missing from catalog"
      WARNS=$((WARNS + 1))
    fi
  done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)
fi

fi  # end of: if [[ -z "$ONLY" || "$ONLY" = "all" ]]; then (mechanical lint block)

# --- Synth lint dispatch ----------------------------------------------------
if [[ -z "$ONLY" || "$ONLY" = "synth" || "$ONLY" = "all" ]]; then
  if declare -F synth_lint_file >/dev/null 2>&1; then
    if [[ -n "$ONLY_FILE" ]]; then
      synth_lint_file "$ONLY_FILE" "$CONTENT_DIR"
    else
      synth_lint_dir "$CONTENT_DIR"
    fi
  fi
fi

if [[ "${HUGO_CHECK:-0}" -eq 1 ]]; then
  if command -v hugo >/dev/null 2>&1; then
    if ! hugo --source . --renderToMemory --logLevel error >/dev/null 2>&1; then
      echo "LINT|ERROR|hugo|template render failed (run 'just build' for details)"
      ERRORS=$((ERRORS + 1))
    fi
  else
    echo "LINT|INFO|hugo|--hugo-check requested but hugo not on PATH; skipping"
    INFOS=$((INFOS + 1))
  fi
fi

echo "LINT-SUMMARY|errors=$ERRORS|warnings=$WARNS|info=$INFOS"

if [[ "$ERRORS" -gt 0 ]]; then exit 2; fi
if [[ "$WARNS" -gt 0 ]]; then exit 1; fi
exit 0
