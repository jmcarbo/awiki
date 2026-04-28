#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

PROVENANCE=""
SOURCE=""
ACCEPT=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --provenance) PROVENANCE="$2"; shift 2 ;;
    --source)     SOURCE="$2"; shift 2 ;;
    --accept-source-change) ACCEPT=1; shift ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

[[ -f "$PROVENANCE" ]] || { echo "provenance not found: $PROVENANCE" >&2; exit 2; }
[[ -n "$SOURCE" ]] || { echo "--source required" >&2; exit 2; }

PINNED=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PROVENANCE" repo)

if [[ "$PINNED" == "$SOURCE" ]]; then
  exit 0
fi

if [[ $ACCEPT -eq 1 ]]; then
  echo "info: source change accepted (pinned=$PINNED, new=$SOURCE)" >&2
  exit 0
fi

cat <<EOF >&2
Source change detected:
  pinned : $PINNED
  new    : $SOURCE
This will execute migrations and overwrite tracked files from the new source.
Re-run with --accept-source-change to proceed.
EOF
exit 1
