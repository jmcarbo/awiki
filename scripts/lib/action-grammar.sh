#!/usr/bin/env bash
# Source-only helper. Canonical action-line grammar for the awiki task layer.
# Sourced by: action-scan.sh (phase 17), lint task-rules T1-T15 (phase 17/18),
# action-recur.sh (phase 18), triage.sh (phase 18). Phase 16 ships ONLY this
# library and its unit tests — do not import it from any consumer in phase 16.

# Allowed status markers (raw chars; pad-with-space rendering is the same).
AWIKI_STATUS_MARKERS=(' ' '/' '?' '>' 'x' '-')

# Match a canonical single-line action:
#   leading "- " (or "* ") + "[<status>] " + text + optional "  ^<id>" tail.
# Captures (positional via BASH_REMATCH):
#   1: status char
#   2: full body after the checkbox (text + optional context + tail + id)
AWIKI_ACTION_LINE_RE='^[[:space:]]*[-*][[:space:]]+\[([ /?>x-])\][[:space:]]+(.+)$'

# A single tail key:value token. Allowed keys per spec:
#   due, defer, wait, since, every, done, priority, est.
AWIKI_TAIL_KEY_RE='^(due|defer|wait|since|every|done|priority|est):[^[:space:]]+$'

# Recurrence-chain separator. Default `~`; phase-17 spike may flip to `__`
# if `~` breaks Obsidian/Hugo round-trip.
AWIKI_RECUR_SEP="${AWIKI_RECUR_SEP:-~}"

# Block-ID:
#   ^<base>                              where <base> = [a-z0-9]{3,16}
#   OR ^<base><AWIKI_RECUR_SEP><n>       recurrence chain instance, n = 1+ digits
_awiki_quote_re() { printf '%s' "$1" | sed 's/[][\\.^$*+?()|{}-]/\\&/g'; }
AWIKI_BLOCK_ID_RE='^\^[a-z0-9]{3,16}('"$(_awiki_quote_re "$AWIKI_RECUR_SEP")"'[0-9]+)?$'

# Internal: per-key value shape regexes.
_AWIKI_RE_DATE='^[0-9]{4}-[0-9]{2}-[0-9]{2}$'
_AWIKI_RE_EVERY='^([0-9]+[dwm]|daily|weekly|monthly)$'
_AWIKI_RE_PRIORITY='^[1-3]$'
_AWIKI_RE_EST='^[0-9]+[mh]$'
_AWIKI_RE_WAIT='^\[\[[a-z0-9][a-z0-9-]*\]\]$'

# Validate that a YYYY-MM-DD string is also a real calendar date.
_awiki_is_calendar_date() {
  local s="$1"
  [[ "$s" =~ $_AWIKI_RE_DATE ]] || return 1
  local y="${s:0:4}" m="${s:5:2}" d="${s:8:2}"
  y=$((10#$y)); m=$((10#$m)); d=$((10#$d))
  (( m >= 1 && m <= 12 )) || return 1
  local maxday=31
  case "$m" in
    4|6|9|11) maxday=30 ;;
    2)
      if (( y % 4 == 0 && (y % 100 != 0 || y % 400 == 0) )); then
        maxday=29
      else
        maxday=28
      fi
      ;;
  esac
  (( d >= 1 && d <= maxday ))
}

# Parse an action line. On match (return 0):
#   AWIKI_AG_STATUS  — single status char
#   AWIKI_AG_TEXT    — text portion (everything between status and the first
#                      @context / tail-key / block-id, trimmed)
#   AWIKI_AG_CONTEXT — "@<slug>" or empty
#   AWIKI_AG_TAIL    — space-joined tail key:value tokens, in source order
#   AWIKI_AG_ID      — "^<id>" or empty
# On non-match: returns 1.
awiki_grammar_parse_action() {
  local line="$1"
  AWIKI_AG_STATUS=""; AWIKI_AG_TEXT=""; AWIKI_AG_CONTEXT=""
  AWIKI_AG_TAIL=""; AWIKI_AG_ID=""

  if ! [[ "$line" =~ $AWIKI_ACTION_LINE_RE ]]; then
    return 1
  fi
  local status="${BASH_REMATCH[1]}"
  local body="${BASH_REMATCH[2]}"

  # Reject double-checkbox lines (e.g., "- [ ] [ ] doubled").
  if [[ "$body" =~ ^\[.\][[:space:]] ]]; then
    return 1
  fi

  AWIKI_AG_STATUS="$status"

  local -a toks=()
  local IFS=' '
  read -r -a toks <<< "$body"

  local text_toks=()
  local ctx="" tail_toks=() id=""
  local tok
  for tok in "${toks[@]}"; do
    if [[ -z "$tok" ]]; then continue; fi
    if [[ -z "$ctx" && "$tok" =~ ^@[a-z0-9][a-z0-9-]*$ ]]; then
      ctx="$tok"; continue
    fi
    if [[ "$tok" =~ $AWIKI_TAIL_KEY_RE ]]; then
      tail_toks+=("$tok"); continue
    fi
    if [[ "$tok" =~ $AWIKI_BLOCK_ID_RE ]]; then
      id="$tok"; continue
    fi
    if (( ${#tail_toks[@]} > 0 )) || [[ -n "$id" || -n "$ctx" ]]; then
      continue
    fi
    text_toks+=("$tok")
  done

  AWIKI_AG_TEXT="${text_toks[*]}"
  AWIKI_AG_CONTEXT="$ctx"
  AWIKI_AG_TAIL="${tail_toks[*]}"
  AWIKI_AG_ID="$id"
  return 0
}

# Split a tail string and emit "k=v" per line on stdout. Sets
# AWIKI_AG_REJECT_REASON to the first failure reason encountered, if any.
awiki_grammar_split_tail() {
  local tail="$1"
  AWIKI_AG_REJECT_REASON=""
  local tok key val
  for tok in $tail; do
    if [[ "$tok" != *:* ]]; then
      [[ -z "$AWIKI_AG_REJECT_REASON" ]] && AWIKI_AG_REJECT_REASON="bad-format"
      continue
    fi
    key="${tok%%:*}"
    val="${tok#*:}"
    case "$key" in
      due|defer|since|done)
        if ! _awiki_is_calendar_date "$val"; then
          [[ -z "$AWIKI_AG_REJECT_REASON" ]] && AWIKI_AG_REJECT_REASON="bad-date"
        fi
        ;;
      every)
        if ! [[ "$val" =~ $_AWIKI_RE_EVERY ]]; then
          [[ -z "$AWIKI_AG_REJECT_REASON" ]] && AWIKI_AG_REJECT_REASON="bad-format"
        fi
        ;;
      priority)
        if ! [[ "$val" =~ $_AWIKI_RE_PRIORITY ]]; then
          [[ -z "$AWIKI_AG_REJECT_REASON" ]] && AWIKI_AG_REJECT_REASON="bad-format"
        fi
        ;;
      est)
        if ! [[ "$val" =~ $_AWIKI_RE_EST ]]; then
          [[ -z "$AWIKI_AG_REJECT_REASON" ]] && AWIKI_AG_REJECT_REASON="bad-format"
        fi
        ;;
      wait)
        if ! [[ "$val" =~ $_AWIKI_RE_WAIT ]]; then
          [[ -z "$AWIKI_AG_REJECT_REASON" ]] && AWIKI_AG_REJECT_REASON="bad-format"
        fi
        ;;
      *)
        [[ -z "$AWIKI_AG_REJECT_REASON" ]] && AWIKI_AG_REJECT_REASON="bad-key"
        ;;
    esac
    printf -- '%s=%s\n' "$key" "$val"
  done
}

# awiki_date_add_days <YYYY-MM-DD> <N>  — emit YYYY-MM-DD plus N days.
# Negative N is allowed (subtracts). Portable: prefers GNU `date -d`,
# falls back to BSD `date -j -v`.
awiki_date_add_days() {
  local in="$1" n="$2"
  _awiki_is_calendar_date "$in" || { echo "awiki_date_add_days: bad date $in" >&2; return 1; }
  [[ "$n" =~ ^-?[0-9]+$ ]] || { echo "awiki_date_add_days: bad delta $n" >&2; return 1; }
  if date -d "$in +$n days" "+%Y-%m-%d" >/dev/null 2>&1; then
    date -d "$in +$n days" "+%Y-%m-%d"
  else
    # BSD/macOS form. -v takes a signed magnitude: "+3d" or "-3d".
    local sign="+"
    if [[ "$n" =~ ^- ]]; then sign="-"; fi
    date -j -v"${sign}${n#-}d" -f "%Y-%m-%d" "$in" "+%Y-%m-%d"
  fi
}

# awiki_recur_compute_due <done-date> <every-token>
# Dispatches Nd / Nw / Nm / daily / weekly / monthly. Echoes YYYY-MM-DD.
# Exits 4 on bad token.
awiki_recur_compute_due() {
  local done_date="$1" every="$2"
  local n unit
  case "$every" in
    daily)   awiki_date_add_days   "$done_date" 1; return 0 ;;
    weekly)  awiki_date_add_days   "$done_date" 7; return 0 ;;
    monthly) awiki_date_add_months "$done_date" 1; return 0 ;;
  esac
  if [[ "$every" =~ ^([0-9]+)([dwm])$ ]]; then
    n="${BASH_REMATCH[1]}"; unit="${BASH_REMATCH[2]}"
    case "$unit" in
      d) awiki_date_add_days   "$done_date" "$n" ;;
      w) awiki_date_add_days   "$done_date" "$((n * 7))" ;;
      m) awiki_date_add_months "$done_date" "$n" ;;
    esac
    return 0
  fi
  echo "awiki_recur_compute_due: bad every: '$every'" >&2
  return 4
}

# Detect a [x] action line that carries an every: tail key.
awiki_is_completed_recurring_action() {
  local l="$1"
  [[ "$l" =~ ^-\ \[x\]\  ]] || return 1
  [[ "$l" =~ \ every:[A-Za-z0-9]+ ]]
}

# Extract the chain base of an action line — the portion of ^id before the
# AWIKI_RECUR_SEP, or the full id if no separator is present.
awiki_extract_chain_base() {
  local l="$1"
  local id
  # Match the trailing ^id (anchored to end-of-line, allowing trailing spaces).
  if [[ "$l" =~ \^([A-Za-z0-9_~]+)[[:space:]]*$ ]]; then
    id="${BASH_REMATCH[1]}"
    if [[ "$id" == *"${AWIKI_RECUR_SEP}"* ]]; then
      printf '%s' "${id%%"${AWIKI_RECUR_SEP}"*}"
    else
      printf '%s' "$id"
    fi
  fi
}

# Extract a tail key value ("every", "done", "due") from an action line.
# Returns empty if absent.
awiki_extract_tail_key() {
  local l="$1" k="$2"
  if [[ "$l" =~ [[:space:]]${k}:([^[:space:]]+) ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
  fi
}

# Build an open-copy line from a completed-recurring line by:
#  - flipping [x] -> [ ]
#  - replacing due:<old> with due:<new>
#  - removing done:<date>
#  - replacing the trailing ^id with the new instance id
awiki_build_open_copy() {
  local in="$1" new_due="$2" new_id="$3"
  local out="$in"
  # Flip the checkbox. Note: bash parameter expansion treats `[x]` as a glob
  # character class, so we use sed for a literal replacement.
  out="$(printf '%s' "$out" | sed -E 's/^- \[x\] /- [ ] /')"
  # Replace due:<date> if present; otherwise splice one in just before the ^id.
  if [[ "$out" =~ ^(.*[[:space:]])due:[0-9]{4}-[0-9]{2}-[0-9]{2}([[:space:]].*)$ ]]; then
    out="${BASH_REMATCH[1]}due:${new_due}${BASH_REMATCH[2]}"
  elif [[ "$out" =~ ^(.*[[:space:]])due:[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
    out="${BASH_REMATCH[1]}due:${new_due}"
  else
    # No prior due — splice one in just before the ^id (if any).
    if [[ "$out" =~ ^(.*)[[:space:]]\^[A-Za-z0-9_~]+[[:space:]]*$ ]]; then
      out="${BASH_REMATCH[1]} due:${new_due} ^${new_id}"
      printf '%s' "$out"; return 0
    else
      out="${out} due:${new_due}"
    fi
  fi
  # Strip done:<date>.
  out="$(printf '%s' "$out" | sed -E 's/[[:space:]]done:[0-9]{4}-[0-9]{2}-[0-9]{2}//')"
  # Replace the trailing ^id with the new instance id.
  out="$(printf '%s' "$out" | sed -E "s/\\^[A-Za-z0-9_~]+\$/^${new_id}/")"
  printf '%s' "$out"
}

# Collect chain state from a page: emits "<base>\t<n>" lines for each instance.
# The chain head (no separator) is "<base>\t1".
awiki_collect_chain_state() {
  local page="$1"
  case "$AWIKI_RECUR_SEP" in
    '~'|'__') ;;
    *) echo "awiki_collect_chain_state: unknown AWIKI_RECUR_SEP='$AWIKI_RECUR_SEP'" >&2; return 1 ;;
  esac
  awk -v sep="$AWIKI_RECUR_SEP" '
    /^- \[[ \/?>x-]\]/ {
      # Find the trailing ^id (allow trailing whitespace).
      if (match($0, /\^[A-Za-z0-9_~]+[[:space:]]*$/)) {
        id = substr($0, RSTART+1, RLENGTH-1)
        sub(/[[:space:]]+$/, "", id)
        sep_pos = index(id, sep)
        if (sep_pos == 0) {
          print id "\t1"
        } else {
          base = substr(id, 1, sep_pos - 1)
          n = substr(id, sep_pos + length(sep))
          if (n ~ /^[0-9]+$/) print base "\t" n
        }
      }
    }' "$page"
}

# Given collected chain state, return the next free <n> for <base>. Always >= 2.
awiki_recur_next_instance_n() {
  local chains="$1" base="$2"
  local max=1 b n
  if [[ -z "$chains" ]]; then
    printf '%s' 2
    return 0
  fi
  while IFS=$'\t' read -r b n; do
    [[ -z "$b" ]] && continue
    if [[ "$b" == "$base" && "$n" =~ ^[0-9]+$ && "$n" -gt "$max" ]]; then
      max="$n"
    fi
  done <<<"$chains"
  printf '%s' "$((max + 1))"
}

# Membership check: does <base> already have <n> in chain state?
awiki_recur_chain_has() {
  local chains="$1" base="$2" n="$3"
  local b nn
  [[ -z "$chains" ]] && return 1
  while IFS=$'\t' read -r b nn; do
    [[ -z "$b" ]] && continue
    if [[ "$b" == "$base" && "$nn" == "$n" ]]; then return 0; fi
  done <<<"$chains"
  return 1
}

# awiki_date_add_months <YYYY-MM-DD> <N>  — month-add with last-day clamp.
# 2026-01-31 + 1m -> 2026-02-28 (clamp). 2024-01-31 + 1m -> 2024-02-29 (leap).
awiki_date_add_months() {
  local in="$1" n="$2"
  _awiki_is_calendar_date "$in" || { echo "awiki_date_add_months: bad date $in" >&2; return 1; }
  [[ "$n" =~ ^-?[0-9]+$ ]] || { echo "awiki_date_add_months: bad delta $n" >&2; return 1; }
  local y="${in:0:4}" m="${in:5:2}" d="${in:8:2}"
  y=$((10#$y)); m=$((10#$m)); d=$((10#$d))
  local total=$(( (y * 12 + (m - 1)) + n ))
  local ny=$(( total / 12 ))
  local nm=$(( (total % 12) + 1 ))
  local maxday=31
  case "$nm" in
    4|6|9|11) maxday=30 ;;
    2)
      if (( ny % 4 == 0 && (ny % 100 != 0 || ny % 400 == 0) )); then
        maxday=29
      else
        maxday=28
      fi
      ;;
  esac
  local nd="$d"
  (( nd > maxday )) && nd=$maxday
  printf '%04d-%02d-%02d\n' "$ny" "$nm" "$nd"
}
