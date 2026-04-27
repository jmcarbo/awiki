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
