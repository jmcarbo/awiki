#!/usr/bin/env bash
set -euo pipefail

# qmd-index.sh — re-index the awiki content/ tree with qmd.
#
# Contract:
# - Emit `QMD-INDEX|ok` on stdout on success.
# - Emit `QMD-INDEX|skip|reason=<reason>` on stderr and exit 0 if qmd is
#   not installed or the content/ directory is missing. Skipping is not
#   a failure: the agent will fall back to grep (per spec).
#
# CLI mapping (see docs/decisions/qmd-install.md):
# - `qntx-labs/qmd` v0.5.0 uses `collection add <abs-path>` + `update`,
#   not the `qmd index <path>` form referenced in the spec. We register
#   a collection named `awiki` rooted at the absolute content/ path on
#   first run, then call `qmd update -c awiki` to re-index.

if ! command -v qmd >/dev/null 2>&1; then
  echo "QMD-INDEX|skip|reason=qmd-not-installed" >&2
  exit 0
fi

if [[ ! -d content ]]; then
  echo "QMD-INDEX|skip|reason=no-content-dir" >&2
  exit 0
fi

mkdir -p .qmd
INDEX_FILE=".qmd/index.sqlite"
COLLECTION="awiki"
CONTENT_ABS="$(cd content && pwd)"

# Register collection on first run. `collection add` is not idempotent
# in v0.5.0, so we list collections (JSON) and grep for the name first.
if ! qmd --index "$INDEX_FILE" collection list --json 2>/dev/null \
    | grep -q "\"name\": \"$COLLECTION\""; then
  qmd --index "$INDEX_FILE" collection add "$CONTENT_ABS" --name "$COLLECTION"
fi

qmd --index "$INDEX_FILE" update -c "$COLLECTION"
echo "QMD-INDEX|ok"
