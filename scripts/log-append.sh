#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: log-append.sh <action> [message...]" >&2
  exit 1
fi

ACTION="$1"
shift
MESSAGE="$*"
LOG_FILE="${AWIKI_LOG_FILE:-content/log.md}"
NOW="$(date '+%Y-%m-%d %H:%M')"

if [[ ! -f "$LOG_FILE" ]]; then
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n\n" > "$LOG_FILE"
fi

printf -- "## [%s] %s | %s\n" "$NOW" "$ACTION" "$MESSAGE" >> "$LOG_FILE"
