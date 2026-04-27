#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

# Clear stale Hugo resource cache and any prior public/ output. Stale
# fingerprinted assets (e.g. en.search.min.<hash>.js) can collide with
# fresh ones generated this session and cause the wrong asset to be
# served, breaking search.
rm -rf resources/_gen public/ .hugo_build.lock

bash "$SCRIPT_DIR/build.sh"

if command -v entr >/dev/null 2>&1; then
  echo "WATCH|tool=entr"
  ( while true; do
      find content -name '*.md' | entr -d bash "$SCRIPT_DIR/build.sh"
    done ) &
elif command -v fswatch >/dev/null 2>&1; then
  echo "WATCH|tool=fswatch"
  ( fswatch -o content | while read -r _; do bash "$SCRIPT_DIR/build.sh"; done ) &
else
  # mtime-aware polling: only rebuild when content/ has changed since
  # last build. Avoids unnecessary rebuilds (and the brief atomic-swap
  # window) every second.
  echo "WATCH|tool=poll-mtime"
  echo "  (install fswatch or entr for inotify-style change detection)"
  ( prev=""
    while true; do
      cur=$(find content -type f -name '*.md' -exec stat -f '%m %N' {} + 2>/dev/null \
            | sort | shasum | awk '{print $1}')
      if [[ "$cur" != "$prev" ]]; then
        bash "$SCRIPT_DIR/build.sh"
        prev="$cur"
      fi
      sleep 1
    done
  ) &
fi
WATCHER_PID=$!
trap 'kill $WATCHER_PID 2>/dev/null || true' EXIT

hugo server --bind 0.0.0.0 --port 1313 -D
