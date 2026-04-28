#!/usr/bin/env bash
# Build a real local git repo from a seed directory tree.
# Usage: build-git-fixture.sh <seed-dir> <out-bare-path>
# Returns 0 on success. The output is a non-bare repo (has working tree)
# so ls-tree on it works directly, and HEAD is `main`.

set -euo pipefail

SEED="$1"
OUT="$2"

if [[ ! -d "$SEED" ]]; then
  echo "ERROR: seed not found: $SEED" >&2; exit 1
fi
if [[ -e "$OUT" ]]; then
  rm -rf "$OUT"
fi
mkdir -p "$OUT"
cp -R "$SEED"/. "$OUT"/
cd "$OUT"
git init -q -b main
git config user.email fixture@local
git config user.name fixture
git add -A
git commit -q -m "fixture: initial commit"
