#!/usr/bin/env bash
# scripts/lib/vendor-vega.sh — fetch vendored vega + vega-lite + vega-embed.
# Pinned versions; bump together. Re-run is idempotent.

set -euo pipefail

VEGA_VERSION="5.30.0"
VEGA_LITE_VERSION="5.21.0"
VEGA_EMBED_VERSION="6.27.0"
OUT_DIR="${1:-static/vendor/vega}"
mkdir -p "$OUT_DIR"

CDN_BASE="https://cdn.jsdelivr.net/npm"

_fetch() {
  local url="$1" dest="$2"
  if [[ -f "$dest" ]]; then
    echo "VENDOR-VEGA|skip $dest (exists)"
    return 0
  fi
  if [[ -n "${AWIKI_VEGA_VENDOR_LOCAL_DIR:-}" ]]; then
    local name
    name="$(basename "$dest")"
    local local_src="$AWIKI_VEGA_VENDOR_LOCAL_DIR/$name"
    if [[ -f "$local_src" ]]; then
      cp "$local_src" "$dest"
      echo "VENDOR-VEGA|copied $name from local cache"
      return 0
    fi
  fi
  if ! command -v curl >/dev/null 2>&1; then
    echo "VENDOR-VEGA|ERROR curl missing; cannot fetch $url" >&2
    return 1
  fi
  curl --fail --silent --show-error -L "$url" -o "$dest"
  echo "VENDOR-VEGA|fetched $dest"
}

_fetch "$CDN_BASE/vega@$VEGA_VERSION/build/vega.min.js"             "$OUT_DIR/vega.min.js"
_fetch "$CDN_BASE/vega-lite@$VEGA_LITE_VERSION/build/vega-lite.min.js" "$OUT_DIR/vega-lite.min.js"
_fetch "$CDN_BASE/vega-embed@$VEGA_EMBED_VERSION/build/vega-embed.min.js" "$OUT_DIR/vega-embed.min.js"

cat > "$OUT_DIR/VERSIONS.txt" <<EOF
vega=$VEGA_VERSION
vega-lite=$VEGA_LITE_VERSION
vega-embed=$VEGA_EMBED_VERSION
fetched=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
EOF
echo "VENDOR-VEGA|done"
