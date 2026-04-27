#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel)}"
cd "$REPO_ROOT"

# Load config
if [[ -f .awiki/config ]]; then
  # shellcheck disable=SC1091
  source .awiki/config
fi
THRESHOLD="${AWIKI_LINT_AFTER_N:-5}"

if [[ $# -lt 1 ]]; then
  echo "usage: ingest.sh <path-under-raw/inbox/>" >&2
  exit 1
fi

SRC="$1"
# Normalize to relative path from repo root.
if [[ "$SRC" = /* ]]; then
  SRC="${SRC#"$REPO_ROOT/"}"
fi

if [[ ! "$SRC" =~ ^raw/inbox/(interactive|batch|checkpoint)/.+ ]]; then
  echo "ERROR: path must be under raw/inbox/{interactive,batch,checkpoint}/" >&2
  exit 2
fi
MODE="${BASH_REMATCH[1]}"

if [[ ! -f "$SRC" ]]; then
  echo "ERROR: source not found: $SRC" >&2
  exit 1
fi

# Strip leading raw/inbox/<mode>/ to get relative subtree
REL="${SRC#raw/inbox/$MODE/}"
DEST="raw/processed/$MODE/$REL"
DEST_DIR="$(dirname "$DEST")"

if [[ -f "$DEST" ]]; then
  echo "ERROR: already processed: $DEST" >&2
  exit 3
fi

mkdir -p "$DEST_DIR"
mv "$SRC" "$DEST"

bash "$SCRIPT_DIR/log-append.sh" ingest "$(basename "$SRC") | mode=$MODE"

# Increment counter
COUNTER_FILE=".awiki/ingest-count"
mkdir -p .awiki
COUNT=0
[[ -f "$COUNTER_FILE" ]] && COUNT="$(cat "$COUNTER_FILE")"
COUNT=$((COUNT + 1))
echo "$COUNT" > "$COUNTER_FILE"

echo "INGEST-OK|src=$SRC|dest=$DEST|mode=$MODE|count=$COUNT"

# Auto-lint
LINT_RC=0
if [[ "$COUNT" -ge "$THRESHOLD" ]]; then
  echo "AUTO-LINT|threshold=$THRESHOLD|count=$COUNT"
  if ! bash "$SCRIPT_DIR/lint.sh"; then
    LINT_RC=4
  fi
  echo "0" > "$COUNTER_FILE"
fi

# Auto-reindex (best-effort)
QMD_RC=0
if [[ "${AWIKI_QMD_STATUS:-}" != "missing" ]] && command -v qmd >/dev/null 2>&1; then
  if ! bash "$SCRIPT_DIR/qmd-index.sh" 2>/dev/null; then
    QMD_RC=5
  fi
fi

if [[ "$LINT_RC" -ne 0 ]]; then exit 4; fi
if [[ "$QMD_RC" -ne 0 ]]; then exit 5; fi
exit 0
