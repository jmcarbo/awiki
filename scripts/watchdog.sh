#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel)}"
cd "$REPO_ROOT"

if [[ -f .awiki/config ]]; then
  # shellcheck disable=SC1091
  source .awiki/config
fi

AGENT_CLI="${AWIKI_AGENT:-}"
BACKEND_OVERRIDE="${AWIKI_WATCHDOG_BACKEND:-}"
CATCHUP=0
ONCE=0
CYCLES="${AWIKI_WATCHDOG_CYCLES:-0}"
POLL_INTERVAL="${AWIKI_WATCHDOG_POLL_INTERVAL:-2}"
STABLE_CHECKS="${AWIKI_WATCHDOG_STABLE_CHECKS:-2}"
STABLE_INTERVAL="${AWIKI_WATCHDOG_STABLE_INTERVAL:-1.5}"
STABLE_MAX_ATTEMPTS="${AWIKI_WATCHDOG_STABLE_MAX:-60}"

usage() {
  cat <<'EOF'
usage: watchdog.sh [--catchup] [--once] [--agent <cli>]
                   [--backend fswatch|inotifywait|poll]
                   [--poll-interval <s>]
                   [--stable-checks <n>] [--stable-interval <s>]

Auto-ingest files arriving in raw/inbox/batch/. Foreground daemon.
Stop with Ctrl-C. On ingest failure, the source is moved to
raw/inbox/batch/_failed/ and watching continues.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --catchup) CATCHUP=1; shift ;;
    --once) ONCE=1; shift ;;
    --cycles) CYCLES="$2"; shift 2 ;;
    --cycles=*) CYCLES="${1#--cycles=}"; shift ;;
    --agent) AGENT_CLI="$2"; shift 2 ;;
    --agent=*) AGENT_CLI="${1#--agent=}"; shift ;;
    --backend) BACKEND_OVERRIDE="$2"; shift 2 ;;
    --backend=*) BACKEND_OVERRIDE="${1#--backend=}"; shift ;;
    --poll-interval) POLL_INTERVAL="$2"; shift 2 ;;
    --stable-checks) STABLE_CHECKS="$2"; shift 2 ;;
    --stable-interval) STABLE_INTERVAL="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "ERROR: unknown arg: $1" >&2; usage >&2; exit 2 ;;
  esac
done

WATCH_DIR="raw/inbox/batch"
FAILED_DIR="$WATCH_DIR/_failed"
PID_FILE=".awiki/watchdog.pid"

if [[ -n "$BACKEND_OVERRIDE" ]]; then
  BACKEND="$BACKEND_OVERRIDE"
elif command -v fswatch >/dev/null 2>&1; then
  BACKEND=fswatch
elif command -v inotifywait >/dev/null 2>&1; then
  BACKEND=inotifywait
else
  BACKEND=poll
fi

mkdir -p .awiki "$WATCH_DIR"

if [[ -f "$PID_FILE" ]]; then
  OTHER="$(cat "$PID_FILE" 2>/dev/null || true)"
  if [[ -n "$OTHER" ]] && kill -0 "$OTHER" 2>/dev/null; then
    echo "WATCHDOG|halt|reason=already-running|pid=$OTHER"
    exit 3
  fi
fi
echo "$$" > "$PID_FILE"

declare -A SEEN
declare -A SKIPPED
STOP=0

cleanup() {
  STOP=1
  pkill -P $$ 2>/dev/null || true
  rm -f "$PID_FILE"
  echo "WATCHDOG|stop"
}
trap cleanup EXIT
trap 'STOP=1' INT TERM

file_size() {
  local f="$1"
  [[ -f "$f" ]] || return 1
  local n
  n="$(wc -c < "$f" 2>/dev/null || echo)"
  printf '%s' "${n//[[:space:]]/}"
}

is_under_failed() {
  case "$1" in
    *"/_failed/"*|"$WATCH_DIR/_failed"|"$WATCH_DIR/_failed"/*) return 0 ;;
    *) return 1 ;;
  esac
}

is_hidden() {
  local b
  b="$(basename "$1")"
  [[ "$b" == .* ]]
}

quarantine() {
  local src="$1"
  local rel="${src#"$WATCH_DIR"/}"
  local dst="$FAILED_DIR/$rel"
  mkdir -p "$(dirname "$dst")"
  if [[ -f "$src" ]]; then
    mv "$src" "$dst"
    printf '%s' "$dst"
  else
    printf '%s' "noop"
  fi
}

run_ingest() {
  local path="$1"
  if [[ -n "${AWIKI_INGEST_CMD:-}" ]]; then
    # Test/override hook. Caller is responsible for any --agent handling.
    # shellcheck disable=SC2086
    eval $AWIKI_INGEST_CMD '"$path"'
    return $?
  fi
  if [[ -n "$AGENT_CLI" ]]; then
    bash "$SCRIPT_DIR/ingest.sh" --agent "$AGENT_CLI" "$path"
  else
    bash "$SCRIPT_DIR/ingest.sh" "$path"
  fi
}

process_file() {
  local path="$1"

  if is_under_failed "$path"; then
    echo "WATCHDOG|skip|$path|reason=under-failed"
    return 0
  fi
  if is_hidden "$path"; then
    echo "WATCHDOG|skip|$path|reason=hidden"
    return 0
  fi
  if [[ ! -f "$path" ]]; then
    echo "WATCHDOG|skip|$path|reason=missing"
    return 0
  fi

  echo "WATCHDOG|detect|$path"

  local prev="" cur="" ok=0 attempts=0 same=0 first=1
  while (( attempts < STABLE_MAX_ATTEMPTS )); do
    if ! cur="$(file_size "$path")"; then
      echo "WATCHDOG|skip|$path|reason=missing"
      return 0
    fi
    if (( first == 0 )) && [[ "$cur" == "$prev" ]]; then
      same=$((same + 1))
      if (( same >= STABLE_CHECKS - 1 )); then
        ok=1
        break
      fi
    else
      same=0
    fi
    first=0
    prev="$cur"
    sleep "$STABLE_INTERVAL"
    attempts=$((attempts + 1))
  done

  if (( ok != 1 )); then
    echo "WATCHDOG|skip|$path|reason=unstable"
    return 0
  fi

  echo "WATCHDOG|stable|$path|size=$cur"

  local rc=0
  run_ingest "$path" || rc=$?

  if (( rc == 0 )); then
    echo "WATCHDOG|ingest-ok|$path"
    bash "$SCRIPT_DIR/log-append.sh" watchdog "ingest-ok $(basename "$path")" >/dev/null 2>&1 || true
  else
    local moved
    moved="$(quarantine "$path")"
    echo "WATCHDOG|ingest-fail|$path|rc=$rc|moved=$moved"
    bash "$SCRIPT_DIR/log-append.sh" watchdog "ingest-fail $(basename "$path") rc=$rc" >/dev/null 2>&1 || true
  fi
}

handle_path() {
  local p="$1"
  [[ -z "$p" ]] && return 0
  p="${p#./}"
  case "$p" in
    "$WATCH_DIR"/*) ;;
    *) return 0 ;;
  esac
  if [[ -n "${SEEN[$p]:-}" ]]; then return 0; fi
  if [[ ! -f "$p" ]]; then return 0; fi

  # Persistent skip filters: hidden files (.gitkeep, .DS_Store) and paths
  # under _failed/ never become ingestible. Log the skip exactly once per
  # path per process — otherwise polling backends spam the log every cycle.
  if is_hidden "$p"; then
    if [[ -z "${SKIPPED[$p]:-}" ]]; then
      echo "WATCHDOG|skip|$p|reason=hidden"
      SKIPPED[$p]=1
    fi
    return 0
  fi
  if is_under_failed "$p"; then
    if [[ -z "${SKIPPED[$p]:-}" ]]; then
      echo "WATCHDOG|skip|$p|reason=under-failed"
      SKIPPED[$p]=1
    fi
    return 0
  fi

  SEEN[$p]=1
  process_file "$p" || true
  unset 'SEEN[$p]'
}

list_existing() {
  find "$WATCH_DIR" -type f -not -path '*/_failed/*' 2>/dev/null | sort
}

echo "WATCHDOG|start|backend=$BACKEND|agent=${AGENT_CLI:-}|catchup=$CATCHUP"

if (( CATCHUP == 1 )); then
  while IFS= read -r p; do
    [[ -z "$p" ]] && continue
    handle_path "$p"
    (( STOP == 1 )) && break
  done < <(list_existing)
fi

if (( ONCE == 1 )); then
  exit 0
fi

case "$BACKEND" in
  fswatch)
    while IFS= read -r -d '' p; do
      handle_path "$p"
      (( STOP == 1 )) && break
    done < <(fswatch -0 -r --event=Created --event=MovedTo --event=Updated "$WATCH_DIR" 2>/dev/null)
    ;;
  inotifywait)
    while IFS= read -r p; do
      [[ -z "$p" ]] && continue
      handle_path "$p"
      (( STOP == 1 )) && break
    done < <(inotifywait -m -r -q -e close_write -e moved_to --format '%w%f' "$WATCH_DIR" 2>/dev/null)
    ;;
  poll)
    iter=0
    while (( STOP == 0 )); do
      while IFS= read -r p; do
        [[ -z "$p" ]] && continue
        handle_path "$p"
        (( STOP == 1 )) && break
      done < <(list_existing)
      (( STOP == 1 )) && break
      iter=$((iter + 1))
      if (( CYCLES > 0 )) && (( iter >= CYCLES )); then break; fi
      sleep "$POLL_INTERVAL"
    done
    ;;
  *)
    echo "ERROR: unknown backend: $BACKEND" >&2
    exit 4
    ;;
esac
