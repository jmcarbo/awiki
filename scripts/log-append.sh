#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: log-append.sh <action> [message...]" >&2
  exit 1
fi

# Neutralize log-poisoning characters in caller-supplied substrings.
# - '|'  -> '_'  (would break the field separator)
# - '\n' -> ' '  (would break the line terminator)
# - '\r' -> ' '  (CR alone or before LF is also a line terminator under some readers)
# Tabs and other characters are left as-is; the log format is pipe-delimited
# so a tab inside a field is harmless. Idempotent on already-clean input,
# so phase-16 callers passing clean strings see no behavioral change.
awiki_log_sanitize() {
  local s="$1"
  s="${s//|/_}"
  # tr replaces single bytes; portable and avoids regex escaping issues.
  s="$(printf '%s' "$s" | tr '\n\r' '  ')"
  printf '%s' "$s"
}

ACTION="$(awiki_log_sanitize "$1")"
shift
MESSAGE="$(awiki_log_sanitize "$*")"
LOG_FILE="${AWIKI_LOG_FILE:-content/log.md}"
NOW="$(date '+%Y-%m-%d %H:%M')"

if [[ ! -f "$LOG_FILE" ]]; then
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n\n" > "$LOG_FILE"
fi

printf -- "## [%s] %s | %s\n" "$NOW" "$ACTION" "$MESSAGE" >> "$LOG_FILE"
