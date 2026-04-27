#!/usr/bin/env bash
# review-status.sh — structured weekly-review report.
#
# Reads:
#   .awiki/maps/actions.tsv        (built by action-scan.sh)
#   content/inbox.md               (inbox lines)
#   raw/inbox/interactive/         (file-shaped captures)
#   .awiki/last-review             (ISO timestamp of last review)
#   content/projects/*.md          (frontmatter for status / last_updated)
#
# Writes: nothing.
# Stdout: REVIEW|<key>|... lines + a closing REVIEW-SUMMARY|... line.
# Exit 0 in the normal case; non-zero is reserved for genuine failures
# (lock contention via lock.sh's exit 7, etc.).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/lock.sh"

REPO_ROOT="${AWIKI_REPO_ROOT:-$(pwd)}"
cd "$REPO_ROOT"

TODAY="${AWIKI_TODAY:-$(date -u +%Y-%m-%d)}"
ACTIONS_TSV=".awiki/maps/actions.tsv"
LAST_REVIEW_FILE=".awiki/last-review"

# Resolve `today` to seconds since epoch using GNU date (or gdate on macOS).
# The whole report assumes ISO-8601 dates, so a single resolution at the
# top is sufficient — failure here means we cannot compute deltas at all.
_to_epoch() {
  local d="$1"
  if date -u -d "$d" +%s 2>/dev/null; then return 0; fi
  if gdate -u -d "$d" +%s 2>/dev/null; then return 0; fi
  return 1
}
TODAY_S="$(_to_epoch "$TODAY" 2>/dev/null || true)"

# Take a brief shared lock to confirm no exclusive writer is mid-rebuild.
# We do not hold the lock across the report — agenda.sh / triage-apply
# release before returning, and the report is read-only.
mkdir -p .awiki
[[ -e .awiki/lock ]] || : > .awiki/lock
awiki_lock_shared --timeout=30 -- true

# ---- 1. inbox-unprocessed -------------------------------------------------
inbox_count=0
if [[ -f content/inbox.md ]]; then
  inbox_count="$(awk '
    BEGIN { fm = 0; in_body = 0 }
    /^---$/ { fm++; if (fm == 2) { in_body = 1 }; next }
    in_body && /^- [0-9]{4}-[0-9]{2}-[0-9]{2}/ { c++ }
    END { print c + 0 }
  ' content/inbox.md)"
fi
printf 'REVIEW|inbox-unprocessed|count=%s\n' "$inbox_count"

# ---- 2. raw-inbox-files ---------------------------------------------------
raw_count=0
if [[ -d raw/inbox/interactive ]]; then
  raw_count="$(find raw/inbox/interactive -mindepth 1 -maxdepth 1 -type f \
                ! -name '.*' 2>/dev/null | wc -l | tr -d ' ')"
fi
printf 'REVIEW|raw-inbox-files|count=%s\n' "$raw_count"

# ---- 3. projects-no-next-action -------------------------------------------
# Active projects with zero open status=' ' or status='/' rows in actions.tsv.
# Exempt: _loose, _someday, _index (system pages).
no_next_slugs=()
shopt -s nullglob
for f in content/projects/*.md; do
  base="$(basename "$f" .md)"
  case "$base" in _loose|_someday|_index) continue ;; esac
  status="$(awk '/^status:/ { print $2; exit }' "$f" | tr -d '"' | tr -d "'")"
  [[ "$status" == "active" ]] || continue

  open=0
  if [[ -f "$ACTIONS_TSV" ]]; then
    open="$(awk -F'\t' -v slug="$base" '
      NR == 1 { next }
      $15 == slug && ($2 == " " || $2 == "/") { c++ }
      END { print c + 0 }
    ' "$ACTIONS_TSV")"
  fi
  if [[ "$open" -eq 0 ]]; then
    no_next_slugs+=("$base")
  fi
done
shopt -u nullglob

if [[ ${#no_next_slugs[@]} -gt 0 ]]; then
  IFS=, ; slugs="${no_next_slugs[*]}" ; IFS=$' \t\n'
  printf 'REVIEW|projects-no-next-action|count=%s|slugs=%s\n' \
    "${#no_next_slugs[@]}" "$slugs"
else
  printf 'REVIEW|projects-no-next-action|count=0\n'
fi

# ---- 4. waiting-stale-14d -------------------------------------------------
# status='?' rows whose since: is more than 14 days before TODAY.
# We use awk to extract (id, since) pairs because bash `read` with
# IFS=$'\t' collapses consecutive empty fields (POSIX whitespace-IFS rule).
stale_ids=()
if [[ -f "$ACTIONS_TSV" && -n "$TODAY_S" ]]; then
  while IFS=$'\t' read -r id since; do
    [[ -n "$since" ]] || continue
    if since_s="$(_to_epoch "$since")"; then
      days=$(( (TODAY_S - since_s) / 86400 ))
      if [[ "$days" -gt 14 ]]; then
        stale_ids+=("$id")
      fi
    fi
  done < <(awk -F'\t' 'NR > 1 && $2 == "?" && $10 != "" { printf "%s\t%s\n", $1, $10 }' "$ACTIONS_TSV")
fi

if [[ ${#stale_ids[@]} -gt 0 ]]; then
  IFS=, ; ids="${stale_ids[*]}" ; IFS=$' \t\n'
  printf 'REVIEW|waiting-stale-14d|count=%s|ids=%s\n' "${#stale_ids[@]}" "$ids"
else
  printf 'REVIEW|waiting-stale-14d|count=0\n'
fi

# ---- 5. overdue -----------------------------------------------------------
# status=' ' or '/' with due: < TODAY.
overdue_ids=()
if [[ -f "$ACTIONS_TSV" ]]; then
  while IFS=$'\t' read -r id due; do
    [[ -n "$due" ]] || continue
    # Lexical comparison is correct for ISO-8601 calendar dates.
    if [[ "$due" < "$TODAY" ]]; then
      overdue_ids+=("$id")
    fi
  done < <(awk -F'\t' 'NR > 1 && ($2 == " " || $2 == "/") && $7 != "" { printf "%s\t%s\n", $1, $7 }' "$ACTIONS_TSV")
fi

if [[ ${#overdue_ids[@]} -gt 0 ]]; then
  # Sort for stable output.
  IFS=$'\n' read -r -d '' -a sorted_overdue < <(printf '%s\n' "${overdue_ids[@]}" | sort && printf '\0') || true
  IFS=, ; ids="${sorted_overdue[*]}" ; IFS=$' \t\n'
  printf 'REVIEW|overdue|count=%s|ids=%s\n' "${#sorted_overdue[@]}" "$ids"
else
  printf 'REVIEW|overdue|count=0\n'
fi

# ---- 6. completed-since-last-review --------------------------------------
last_review_iso=""
if [[ -f "$LAST_REVIEW_FILE" ]]; then
  last_review_iso="$(tr -d '[:space:]' < "$LAST_REVIEW_FILE")"
fi
last_review_date="${last_review_iso%%T*}"
[[ -z "$last_review_date" ]] && last_review_date="never"

completed=0
if [[ -f "$ACTIONS_TSV" && "$last_review_date" != "never" ]]; then
  completed="$(awk -F'\t' -v cutoff="$last_review_date" '
    NR == 1 { next }
    $2 == "x" && $12 != "" && $12 >= cutoff { c++ }
    END { print c + 0 }
  ' "$ACTIONS_TSV")"
elif [[ -f "$ACTIONS_TSV" ]]; then
  # No last-review baseline; surface all completed.
  completed="$(awk -F'\t' 'NR > 1 && $2 == "x" { c++ } END { print c + 0 }' "$ACTIONS_TSV")"
fi
printf 'REVIEW|completed-since-last-review|count=%s\n' "$completed"

# ---- 7. stuck-projects ----------------------------------------------------
# Active projects with last_updated more than 14d before TODAY AND no
# completed action since last_updated.
stuck_slugs=()
shopt -s nullglob
for f in content/projects/*.md; do
  base="$(basename "$f" .md)"
  case "$base" in _loose|_someday|_index) continue ;; esac
  status="$(awk '/^status:/ { print $2; exit }' "$f" | tr -d '"' | tr -d "'")"
  [[ "$status" == "active" ]] || continue

  last_updated="$(awk '/^last_updated:/ { print $2; exit }' "$f" | tr -d '"' | tr -d "'")"
  [[ -n "$last_updated" ]] || continue
  [[ -n "$TODAY_S" ]] || continue

  if ! lu_s="$(_to_epoch "$last_updated")"; then continue; fi
  days_idle=$(( (TODAY_S - lu_s) / 86400 ))
  [[ "$days_idle" -gt 14 ]] || continue

  recent_done=0
  if [[ -f "$ACTIONS_TSV" ]]; then
    recent_done="$(awk -F'\t' -v slug="$base" -v cutoff="$last_updated" '
      NR > 1 && $15 == slug && $2 == "x" && $12 != "" && $12 >= cutoff { c++ }
      END { print c + 0 }
    ' "$ACTIONS_TSV")"
  fi

  if [[ "$recent_done" -eq 0 ]]; then
    stuck_slugs+=("$base")
  fi
done
shopt -u nullglob

if [[ ${#stuck_slugs[@]} -gt 0 ]]; then
  IFS=, ; slugs="${stuck_slugs[*]}" ; IFS=$' \t\n'
  printf 'REVIEW|stuck-projects|count=%s|slugs=%s\n' "${#stuck_slugs[@]}" "$slugs"
else
  printf 'REVIEW|stuck-projects|count=0\n'
fi

# ---- 8. someday-count -----------------------------------------------------
someday=0
if [[ -f "$ACTIONS_TSV" ]]; then
  someday="$(awk -F'\t' 'NR > 1 && $2 == ">" { c++ } END { print c + 0 }' "$ACTIONS_TSV")"
fi
printf 'REVIEW|someday-count|count=%s\n' "$someday"

# ---- REVIEW-SUMMARY -------------------------------------------------------
attention=$(( ${#overdue_ids[@]} + ${#stale_ids[@]} + ${#no_next_slugs[@]} + ${#stuck_slugs[@]} ))
printf 'REVIEW-SUMMARY|last-review=%s|attention=%s\n' "$last_review_date" "$attention"

exit 0
