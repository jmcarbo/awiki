#!/usr/bin/env bash
# Source-only helper. Wraps flock around .awiki/lock with two timeout profiles.
# Defines:
#   awiki_lock_with    --timeout=<seconds> -- <command...>   (exclusive write lock)
#   awiki_lock_shared  --timeout=<seconds> -- <command...>   (shared read lock)
# On contention (timeout reached), exits 7. Rejects missing `--` separator
# and non-numeric timeouts (exit 1). Caller is responsible for set -e discipline.
#
# Exports:
#   AWIKI_LOCK_TIMEOUT_USER=30        (user-facing mutations)
#   AWIKI_LOCK_TIMEOUT_DEFERRED=180   (deferred agenda rebuild path)

export AWIKI_LOCK_TIMEOUT_USER=30
export AWIKI_LOCK_TIMEOUT_DEFERRED=180

_awiki_lock_run() {
  local mode="$1"; shift
  local timeout=""
  local saw_dashdash=0

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --timeout=*) timeout="${1#--timeout=}"; shift ;;
      --) saw_dashdash=1; shift; break ;;
      *) echo "ERROR: unexpected arg before '--': $1" >&2; return 1 ;;
    esac
  done

  if [[ "$saw_dashdash" -ne 1 ]]; then
    echo "ERROR: missing '--' separator before command" >&2
    return 1
  fi
  if [[ -z "$timeout" || ! "$timeout" =~ ^[0-9]+$ ]]; then
    echo "ERROR: invalid or missing --timeout=<seconds>" >&2
    return 1
  fi
  if [[ $# -eq 0 ]]; then
    echo "ERROR: no command provided after '--'" >&2
    return 1
  fi

  mkdir -p .awiki
  if [[ ! -e .awiki/lock ]]; then
    : > .awiki/lock
  fi

  (
    flock -w "$timeout" "$mode" 9 || exit 7
    "$@"
  ) 9>>.awiki/lock
}

awiki_lock_with() {
  _awiki_lock_run -x "$@"
}

awiki_lock_shared() {
  _awiki_lock_run -s "$@"
}
