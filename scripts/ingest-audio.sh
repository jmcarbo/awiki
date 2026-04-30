#!/usr/bin/env bash
set -euo pipefail

# Slice 5 of the Go ingest port: when bin/awiki is built and the user
# has not opted into the legacy bash path, exec the Go implementation.
# Mirrors scripts/ingest-pdf.sh:9-16. Set AWIKI_INGEST_AUDIO_LEGACY=1 to
# force the original bash path (used by the bats oracle and parity
# smoke tests until cleanup in slice 10).
AWIKI_INGEST_AUDIO_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AWIKI_INGEST_AUDIO_REPO_ROOT="$(cd "$AWIKI_INGEST_AUDIO_SCRIPT_DIR/.." && pwd)"
AWIKI_INGEST_AUDIO_GO_BIN="$AWIKI_INGEST_AUDIO_REPO_ROOT/bin/awiki"

if [[ "${AWIKI_INGEST_AUDIO_LEGACY:-0}" != "1" && -x "$AWIKI_INGEST_AUDIO_GO_BIN" ]]; then
  cd "$AWIKI_INGEST_AUDIO_REPO_ROOT" || exit 1
  exec "$AWIKI_INGEST_AUDIO_GO_BIN" ingest-audio "$@"
fi

[[ $# -eq 1 ]] || { echo "usage: ingest-audio.sh <audio-path-under-raw/inbox/>" >&2; exit 1; }
AUDIO="$1"

[[ -f "$AUDIO" ]] || { echo "not found: $AUDIO" >&2; exit 1; }
[[ "$AUDIO" =~ ^raw/inbox/ ]] || { echo "must be under raw/inbox/" >&2; exit 1; }
command -v whisper-cpp >/dev/null 2>&1 || { echo "Install whisper.cpp" >&2; exit 1; }
[[ -f models/ggml-base.en.bin ]] || { echo "missing models/ggml-base.en.bin (download from whisper.cpp releases)" >&2; exit 1; }

OUT="${AUDIO%.*}.md"
ORIG_DIR="raw/processed/_originals"
mkdir -p "$ORIG_DIR"
cp "$AUDIO" "$ORIG_DIR/$(basename "$AUDIO")"

whisper-cpp -m models/ggml-base.en.bin -otxt -f "$AUDIO"
mv "${AUDIO}.txt" "$OUT"

TMP="$(mktemp)"
{
  printf -- "---\ntitle: \"%s\"\ndate: %s\nlast_updated: %s\ntype: source\ntags: [audio]\naliases: []\nsources: []\noriginal: %s\ndraft: false\n---\n\n" \
    "$(basename "$AUDIO")" "$(date '+%Y-%m-%d')" "$(date '+%Y-%m-%d')" "$ORIG_DIR/$(basename "$AUDIO")"
  cat "$OUT"
} > "$TMP"
mv "$TMP" "$OUT"

rm "$AUDIO"
echo "AUDIO-TRANSCRIBED|out=$OUT|orig=$ORIG_DIR/$(basename "$AUDIO")"
echo "Now run: just ingest $OUT"
