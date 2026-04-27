#!/usr/bin/env bash
set -euo pipefail

# task-layer-migrate-review.sh — one-shot insertion of the Weekly Review
# subsection into an already-initialized wiki's WIKI.md.
#
# Idempotent. Safe to re-run. Looks for the existing
# <!-- BEGIN task-layer --> bracket from phase 16; if absent, exits 0
# with a notice (the wiki has not run task-init yet — running task-init
# will include this section automatically once phase 19 has shipped).
#
# Locking: takes the awiki_lock_with exclusive lock when `flock(1)` is
# present. On systems without flock (e.g. macOS without util-linux), the
# migration runs without the lock — the operation is a single-file
# atomic rewrite (mv into place) so concurrent reads are safe; the
# narrow risk is two concurrent writers, which would be a rare manual
# foot-gun for a one-shot migration anyway.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

WIKI="WIKI.md"
TEMPLATE="${SCRIPT_DIR}/templates/wiki-weekly-review.md"

_migrate_review_inner() {
  if [[ ! -f "$WIKI" ]]; then
    echo "task-layer-migrate-review: $WIKI not found; nothing to do" >&2
    return 0
  fi
  if [[ ! -f "$TEMPLATE" ]]; then
    echo "task-layer-migrate-review: template missing at $TEMPLATE" >&2
    return 1
  fi
  if ! grep -q '^<!-- BEGIN task-layer -->$' "$WIKI"; then
    echo "task-layer-migrate-review: <!-- BEGIN task-layer --> marker not present; run \`just task-init\` first" >&2
    return 0
  fi
  if grep -q '^### Weekly Review$' "$WIKI"; then
    echo "task-layer-migrate-review: Weekly Review subsection already present; no-op" >&2
    return 0
  fi

  # Insert the template just before the <!-- END task-layer --> marker.
  local TMP
  TMP="$(mktemp)"
  awk -v tmpl="$TEMPLATE" '
    /^<!-- END task-layer -->$/ {
      while ((getline line < tmpl) > 0) print line
      print ""
    }
    { print }
  ' "$WIKI" > "$TMP"
  mv "$TMP" "$WIKI"

  echo "task-layer-migrate-review: Weekly Review subsection appended to $WIKI"
}

if command -v flock >/dev/null 2>&1; then
  # shellcheck source=lib/lock.sh
  . "${SCRIPT_DIR}/lib/lock.sh"
  awiki_lock_with --timeout="${AWIKI_LOCK_TIMEOUT_USER:-30}" -- _migrate_review_inner
else
  _migrate_review_inner
fi
