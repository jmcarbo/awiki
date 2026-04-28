#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(pwd)"

usage() {
  cat <<EOF
usage: template-init.sh --repo <url> --ref <ref> --version <version> --commit <sha>

Seeds .awiki/template.json + template-cache/<commit>/ from current working tree.
Idempotent: re-run rebuilds cache from current state.
EOF
  exit 2
}

REPO=""
REF=""
VERSION=""
COMMIT=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) REPO="$2"; shift 2 ;;
    --ref) REF="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --commit) COMMIT="$2"; shift 2 ;;
    *) usage ;;
  esac
done

[[ -z "$REPO" || -z "$REF" || -z "$VERSION" || -z "$COMMIT" ]] && usage

MANIFEST="$REPO_ROOT/template.manifest.toml"
BOOTSTRAP_MD="$REPO_ROOT/BOOTSTRAP.md"
PJ="$REPO_ROOT/.awiki/template.json"

[[ -f "$MANIFEST" ]] || { echo "template.manifest.toml not found at repo root" >&2; exit 1; }
[[ -f "$BOOTSTRAP_MD" ]] || { echo "BOOTSTRAP.md not found at repo root" >&2; exit 1; }

# 1. Write template.json (init or rewrite-with-existing-progress).
if [[ -f "$PJ" ]]; then
  echo "info: $PJ already exists; preserving applied_migrations + bootstrap_steps_done" >&2
else
  bash "$SCRIPT_DIR/template-provenance.sh" init "$PJ" "$REPO" "$REF" "$VERSION" "$COMMIT"
fi

# Always update commit/version/ref to current values.
bash "$SCRIPT_DIR/template-provenance.sh" set "$PJ" commit "$COMMIT" >/dev/null
bash "$SCRIPT_DIR/template-provenance.sh" set "$PJ" ref "$REF" >/dev/null
bash "$SCRIPT_DIR/template-provenance.sh" set "$PJ" version "$VERSION" >/dev/null

# 2. Snapshot template tree to cache.
CACHE_DIR="$REPO_ROOT/.awiki/template-cache/$COMMIT"
mkdir -p "$CACHE_DIR"
# Use git archive to snapshot tracked files only (matches what template ships).
git archive --format=tar HEAD | tar -x -C "$CACHE_DIR"

# 3. Record bootstrap_steps_done — but only if NOT already present (idempotent).
EXISTING_STEPS=$(python3 -c "
import json, sys
try:
    d = json.load(open('$PJ'))
except Exception:
    sys.exit(0)
for s in d.get('bootstrap_steps_done', []):
    print(s.get('id', ''))
" 2>/dev/null)

while IFS= read -r STEP_ID; do
  [[ -z "$STEP_ID" ]] && continue
  if echo "$EXISTING_STEPS" | grep -qx "$STEP_ID"; then
    continue  # already recorded
  fi
  HASH=$(python3 "$SCRIPT_DIR/_template_helpers/bootstrap_hash.py" hash "$BOOTSTRAP_MD" "$STEP_ID")
  bash "$SCRIPT_DIR/template-provenance.sh" append-bootstrap-step "$PJ" "$STEP_ID" applied "" "$HASH" >/dev/null
done < <(python3 "$SCRIPT_DIR/_template_helpers/bootstrap_hash.py" list "$BOOTSTRAP_MD")

echo "template-init: pinned $REPO @ $COMMIT (version $VERSION)"
